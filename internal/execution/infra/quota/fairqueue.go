package quota

import (
	"errors"
	"sort"
	"sync"
)

var ErrInvalidWeight = errors.New("queue weight must be positive")

// QueueItem is a runnable scheduling request. Weight is a team priority: a
// larger value receives a proportionally larger share, while FIFO is retained
// within the same team.
type QueueItem struct {
	ID     string
	TeamID int64
	Weight int
	Value  any
}

type queuedItem struct {
	QueueItem
	finish   float64
	sequence uint64
}

// FairQueue is a lock-safe weighted fair queue (WFQ). It has no process-local
// correctness role in production; Redis Streams will persist the same ordering
// key. This implementation pins the scheduling policy with deterministic tests.
type FairQueue struct {
	mu       sync.Mutex
	virtual  float64
	last     map[int64]float64
	items    []queuedItem
	sequence uint64
}

func NewFairQueue() *FairQueue { return &FairQueue{last: make(map[int64]float64)} }

func (q *FairQueue) Enqueue(item QueueItem) error {
	if item.TeamID <= 0 || item.Weight <= 0 {
		return ErrInvalidWeight
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	start := q.virtual
	if q.last[item.TeamID] > start {
		start = q.last[item.TeamID]
	}
	finish := start + 1/float64(item.Weight)
	q.last[item.TeamID] = finish
	q.sequence++
	q.items = append(q.items, queuedItem{QueueItem: item, finish: finish, sequence: q.sequence})
	return nil
}

func (q *FairQueue) Dequeue() (QueueItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return QueueItem{}, false
	}
	sort.SliceStable(q.items, func(i, j int) bool {
		if q.items[i].finish == q.items[j].finish {
			return q.items[i].sequence < q.items[j].sequence
		}
		return q.items[i].finish < q.items[j].finish
	})
	next := q.items[0]
	q.items = q.items[1:]
	q.virtual = next.finish
	return next.QueueItem, true
}

func (q *FairQueue) Len() int { q.mu.Lock(); defer q.mu.Unlock(); return len(q.items) }
