// Package coopsched is a benchmark and playground for https://github.com/golang/go/issues/51071.
package coopsched

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// A Scheduler can manage a set of goroutines started with Go. If
// Yield is called, the goroutine may beblocked until the scheduler
// unblocks it. Yield blocks if the time slot is up, but is otherwise
// a no-op.
type Scheduler struct {
	yieldCh chan *task
	doneCh  chan struct{}
	wg      sync.WaitGroup

	numP       uintptr // Configured number of running goroutines.
	numRunning uintptr // Actual number of running goroutines.
	timeSlot   uintptr // The currently executing time slot.

	blockingTimeNS int64
	runningTimeNS  int64
	sumQueued      int // Sum of the number of queued tasks for each Get.
	numGetCalls    int // The number of successful Get calls.
}

// NewScheduler creates a new scheduler with the given algorithm.
func NewScheduler(numP int, algo SchedulingAlgo) *Scheduler {
	if numP <= 0 {
		numP = runtime.GOMAXPROCS(0) - 1
		if numP <= 0 {
			numP = 1
		}
	}

	s := &Scheduler{
		yieldCh: make(chan *task, runtime.GOMAXPROCS(0)),
		doneCh:  make(chan struct{}),
		numP:    uintptr(numP),
	}

	s.wg.Add(2)
	go s.runQueue(newTaskPriorityQueue(algo))
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
func RunningTimeFair(t *task) int { return int(t.runningTimeNS) }

// Close stops the scheduler's internal goroutines, but does not stop
// goroutines started by Go. Yield panics if called after this
// function has been called.
func (s *Scheduler) Close() error {
	close(s.yieldCh)
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
	atomic.AddUintptr(&s.numRunning, 1)

	go func() {
		defer func() {
			t.runningTimeNS += nowNano() - t.start

			close(t.wakeCh)

			atomic.AddUintptr(&s.numRunning, ^uintptr(0))
			s.yieldCh <- nil

			atomic.AddInt64(&s.blockingTimeNS, t.blockingTimeNS)
			atomic.AddInt64(&s.runningTimeNS, t.runningTimeNS)
		}()

		t.yield()
		f(t.newContext(ctx))
	}()
}

// RunningTime returns the total running time (not waiting in Yield)
// for all goroutines.
func (s *Scheduler) RunningTime() time.Duration {
	return time.Duration(atomic.LoadInt64(&s.runningTimeNS)) * time.Nanosecond
}

// BlockingTime returns the total blocking time (waiting in Yield) for
// all goroutines.
func (s *Scheduler) BlockingTime() time.Duration {
	return time.Duration(atomic.LoadInt64(&s.blockingTimeNS)) * time.Nanosecond
}

// AvgLoad returns the average task queue size.
func (s *Scheduler) AvgLoad() float32 {
	return float32(s.sumQueued) / float32(s.numGetCalls)
}

// runQueue reads from the task queue and unblocks goroutines in Yield.
func (s *Scheduler) runQueue(q taskQueue) {
	defer s.wg.Done()

	for {
		if !s.recvYielded(q) {
			break
		}

		s.resumeEnough(q)
	}
}

// recvYielded blocks until a task has yielded or terminated. It
// receives as many tasks as are available, to maximize queue load.
func (s *Scheduler) recvYielded(q taskQueue) bool {
	t, ok := <-s.yieldCh
	if !ok {
		return false
	}

	for {
		if t != nil {
			atomic.AddUintptr(&s.numRunning, ^uintptr(0))
			q.Put(t)
		}
		select {
		case t, ok = <-s.yieldCh:
			if !ok {
				return false
			}
		default:
			return true
		}
	}
}

// resumeEnough resumes tasks from the task queue until numP tasks are
// running, or the queue is empty.
func (s *Scheduler) resumeEnough(q taskQueue) {
	for {
		n := atomic.LoadUintptr(&s.numRunning)
		if n >= s.numP {
			return
		} else if !atomic.CompareAndSwapUintptr(&s.numRunning, n, n+1) {
			continue
		}

		t := q.Get()
		if t == nil {
			atomic.AddUintptr(&s.numRunning, ^uintptr(0))
			return
		}
		s.sumQueued += q.Len() + 1
		s.numGetCalls++

		select {
		case t.wakeCh <- struct{}{}:
			// Continue.
		default:
			// Continue.
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
			// Continue.
		case <-s.doneCh:
			return
		}

		atomic.AddUintptr(&s.timeSlot, 1)
	}
}

// Yield blocks the goroutine if it has been preempted and waits for
// the scheduler to resume it.
func Yield(ctx context.Context) {
	t := taskFromContext(ctx)
	if t == nil {
		panic(errors.New("the context doesn't reference a Scheduler"))
	}

	if t.timeSlot >= atomic.LoadUintptr(&t.s.timeSlot) {
		return
	}

	select {
	case <-t.s.doneCh:
		panic(errors.New("Yield was called after the scheduler was closed."))
	default:
	}

	t.yield()
}

type task struct {
	s *Scheduler

	wakeCh   chan struct{}
	timeSlot uintptr

	start          int64
	runningTimeNS  int64
	blockingTimeNS int64
}

func taskFromContext(ctx context.Context) *task {
	return ctx.Value(taskKey).(*task)
}

var taskKey = new(int)

// newContext creates a context with the task embedded.
func (t *task) newContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, taskKey, t)
}

// yield unconditionally marks the task as blocked and sends it to the
// scheduler.
func (t *task) yield() {
	now := nowNano()
	t.runningTimeNS += now - t.start
	t.start = now

	t.s.yieldCh <- t
	<-t.wakeCh

	t.timeSlot = atomic.LoadUintptr(&t.s.timeSlot)

	now = nowNano()
	t.blockingTimeNS += now - t.start
	t.start = now
}

func nowNano() int64 {
	return time.Now().UnixNano()
}
