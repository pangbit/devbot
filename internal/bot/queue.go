package bot

import (
	"sync"
	"sync/atomic"
)

type MessageQueue struct {
	mu     sync.Mutex
	queues map[string]chan func()
	counts map[string]*int32
}

func NewMessageQueue() *MessageQueue {
	return &MessageQueue{
		queues: make(map[string]chan func()),
		counts: make(map[string]*int32),
	}
}

func (q *MessageQueue) Enqueue(chatID string, task func()) {
	q.mu.Lock()
	ch, ok := q.queues[chatID]
	cnt := q.counts[chatID]
	if !ok {
		ch = make(chan func(), 100)
		cnt = new(int32)
		q.counts[chatID] = cnt
		q.queues[chatID] = ch
		go q.worker(cnt, ch)
	}
	q.mu.Unlock()

	atomic.AddInt32(cnt, 1)
	ch <- task
}

func (q *MessageQueue) worker(cnt *int32, ch chan func()) {
	for task := range ch {
		task()
		atomic.AddInt32(cnt, -1)
	}
}

func (q *MessageQueue) PendingCount(chatID string) int {
	q.mu.Lock()
	cnt := q.counts[chatID]
	q.mu.Unlock()
	if cnt == nil {
		return 0
	}
	return int(atomic.LoadInt32(cnt))
}
