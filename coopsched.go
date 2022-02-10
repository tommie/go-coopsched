// Package coopsched is a benchmark and playground for https://github.com/golang/go/issues/51071.
package coopsched

import (
	"container/heap"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// A Scheduler can manage a set of goroutines started with Go. If
// Yield is called, the goroutine may beblocked until the scheduler
// unblocks it. Yield blocks if the time slot is up, but is otherwise
// a no-op.
type Scheduler struct {
	taskq        taskQueue
	doneCh       chan struct{}
	wakeCh       chan struct{}
	wg           sync.WaitGroup
	timeSlot     uintptr
	blockingTime int64
	runningTime  int64
}

// NewScheduler creates a new scheduler with the given algorithm.
func NewScheduler(algo SchedulingAlgo) *Scheduler {
	s := &Scheduler{
		taskq:  newTaskPriorityQueue(algo),
		wakeCh: make(chan struct{}, 1),
		doneCh: make(chan struct{}),
	}

	s.wg.Add(2)
	go s.runQueue()
	go s.runTimeSlot()

	return s
}

// SchedulingAlgo is an algorithm for ordering tasks when scheduling
// them.
type SchedulingAlgo func(t *task) int

// FIFO selects the task that has waited the longest in the
// queue. This is what the Go scheduler (runq) does now.
func FIFO(t *task) int { return int(t.start) }

// RunningTimeFair selects the goroutine that has been running the
// least amount of time. This implements the proposed CFS without
// priorities.
func RunningTimeFair(t *task) int { return int(t.runningTime) }

// Close stops the scheduler's internal goroutines, but does not stop
// goroutines started by Go. Yield panics if called after this
// function has been called.
func (s *Scheduler) Close() error {
	close(s.wakeCh)
	close(s.doneCh)
	s.wg.Wait()

	return nil
}

// Go creates a new goroutine, managed by the scheduler. There's
// nothing special about the goroutine unless Yield is called.
func (s *Scheduler) Go(ctx context.Context, f func(context.Context)) {
	t := &task{
		s:        s,
		wakeCh:   make(chan struct{}, 1),
		timeSlot: atomic.LoadUintptr(&s.timeSlot),
		start:    nowNano(),
	}

	go func() {
		defer close(t.wakeCh)
		defer func() {
			t.runningTime += nowNano() - t.start

			atomic.AddInt64(&s.blockingTime, t.blockingTime)
			atomic.AddInt64(&s.runningTime, t.runningTime)
		}()

		f(context.WithValue(ctx, taskKey, t))
	}()
}

// RunningTime returns the total running time (not waiting in Yield)
// for all goroutines.
func (s *Scheduler) RunningTime() time.Duration {
	return time.Duration(atomic.LoadInt64(&s.runningTime)) * time.Nanosecond
}

// BlockingTime returns the total blocking time (waiting in Yield) for
// all goroutines.
func (s *Scheduler) BlockingTime() time.Duration {
	return time.Duration(atomic.LoadInt64(&s.blockingTime)) * time.Nanosecond
}

// runQueue reads from the task queue and unblocks goroutines in Yield.
func (s *Scheduler) runQueue() {
	defer s.wg.Done()

	for range s.wakeCh {
		for {
			t := s.taskq.Get()
			if t == nil {
				break
			}

			select {
			case t.wakeCh <- struct{}{}:
			default:
			}
		}
	}
}

// runTimeSlot updates the `timeSlot` so Yield preempts a goroutine
// only slated for an earlier time slot.
func (s *Scheduler) runTimeSlot() {
	defer s.wg.Done()

	t := time.NewTicker(10 * time.Millisecond)
	defer t.Stop()

	for {
		select {
		case <-t.C:
		case <-s.doneCh:
			return
		}

		atomic.AddUintptr(&s.timeSlot, 1)
	}
}

// Yield blocks the goroutine if it has been preempted and waits for
// the scheduler to resume it.
func Yield(ctx context.Context) {
	t := fromContext(ctx)

	if t.timeSlot >= atomic.LoadUintptr(&t.s.timeSlot) {
		return
	}

	select {
	case <-t.s.doneCh:
		panic(errors.New("Yield was called after the scheduler was closed."))
	default:
	}

	now := nowNano()
	t.runningTime += now - t.start
	t.start = now

	t.s.taskq.Put(t)
	select {
	case t.s.wakeCh <- struct{}{}:
	default:
	}
	<-t.wakeCh

	t.timeSlot = atomic.LoadUintptr(&t.s.timeSlot)

	now = nowNano()
	t.blockingTime += now - t.start
	t.start = now
}

func nowNano() int64 {
	return time.Now().UnixNano()
}

var taskKey = new(int)

func fromContext(ctx context.Context) *task {
	return ctx.Value(taskKey).(*task)
}

type task struct {
	s *Scheduler

	wakeCh       chan struct{}
	timeSlot     uintptr
	start        int64
	runningTime  int64
	blockingTime int64
}

type taskQueue interface {
	Len() int
	Put(t *task)
	Get() *task
}

type taskPriorityQueue struct {
	prio func(*task) int

	mu sync.Mutex
	ts []*task
}

func newTaskPriorityQueue(prio func(*task) int) *taskPriorityQueue {
	return &taskPriorityQueue{
		prio: prio,
	}
}

func (q *taskPriorityQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	return len(q.ts)
}

func (q *taskPriorityQueue) Put(t *task) {
	q.mu.Lock()
	defer q.mu.Unlock()

	heap.Push((*taskHeap)(q), t)
}

func (q *taskPriorityQueue) Get() *task {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.ts) == 0 {
		return nil
	}

	return heap.Pop((*taskHeap)(q)).(*task)
}

// A taskHeap implements heap.Interface and orders the queue based on
// the taskPriorityQueue.prio function.
type taskHeap taskPriorityQueue

func (h *taskHeap) Len() int           { return len(h.ts) }
func (h *taskHeap) Less(i, j int) bool { return h.prio(h.ts[i]) < h.prio(h.ts[j]) }
func (h *taskHeap) Swap(i, j int)      { h.ts[i], h.ts[j] = h.ts[j], h.ts[i] }
func (h *taskHeap) Push(t interface{}) { h.ts = append(h.ts, t.(*task)) }

func (h *taskHeap) Pop() interface{} {
	t := h.ts[len(h.ts)-1]
	h.ts = h.ts[:len(h.ts)-1]
	return t
}
