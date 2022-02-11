package coopsched

import "container/heap"

type taskQueue interface {
	Len() int
	Put(t *task)
	Get() *task
}

type taskPriorityQueue struct {
	ts []*task
}

func newTaskPriorityQueue() *taskPriorityQueue {
	return &taskPriorityQueue{}
}

func (q *taskPriorityQueue) Len() int {
	return len(q.ts)
}

func (q *taskPriorityQueue) Put(t *task) {
	heap.Push((*taskHeap)(q), t)
}

func (q *taskPriorityQueue) Get() *task {
	if len(q.ts) == 0 {
		return nil
	}

	return heap.Pop((*taskHeap)(q)).(*task)
}

// A taskHeap implements heap.Interface and orders the queue based on
// the taskPriorityQueue.prio function.
type taskHeap taskPriorityQueue

func (h *taskHeap) Len() int           { return len(h.ts) }
func (h *taskHeap) Less(i, j int) bool { return h.ts[i].priority < h.ts[j].priority }
func (h *taskHeap) Swap(i, j int)      { h.ts[i], h.ts[j] = h.ts[j], h.ts[i] }
func (h *taskHeap) Push(t interface{}) { h.ts = append(h.ts, t.(*task)) }

func (h *taskHeap) Pop() interface{} {
	t := h.ts[len(h.ts)-1]
	h.ts = h.ts[:len(h.ts)-1]
	return t
}
