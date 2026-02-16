package bot

import (
	"fmt"
	"sync"
	"sync/atomic"
)

type MessageQueue struct {
	mu     sync.Mutex
	queues map[string]chan func()
	counts map[string]*int32
	wg     sync.WaitGroup
}

func NewMessageQueue() *MessageQueue {
	return &MessageQueue{
		queues: make(map[string]chan func()),
		counts: make(map[string]*int32),
	}
}

func (q *MessageQueue) Enqueue(chatID string, task func()) error {
	q.mu.Lock()
	ch, ok := q.queues[chatID]
	cnt := q.counts[chatID]
	if !ok {
		ch = make(chan func(), 100)
		cnt = new(int32)
		q.counts[chatID] = cnt
		q.queues[chatID] = ch
		q.wg.Add(1)
		go q.worker(cnt, ch)
	}
	q.mu.Unlock()

	atomic.AddInt32(cnt, 1)
	select {
	case ch <- task:
		return nil
	default:
		atomic.AddInt32(cnt, -1)
		return fmt.Errorf("queue full for chat %s", chatID)
	}
}

func (q *MessageQueue) worker(cnt *int32, ch chan func()) {
	defer q.wg.Done()
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

// Shutdown closes all channels and waits for workers to finish in-flight tasks.
func (q *MessageQueue) Shutdown() {
	q.mu.Lock()
	for _, ch := range q.queues {
		close(ch)
	}
	q.mu.Unlock()

	q.wg.Wait()

	q.mu.Lock()
	q.queues = make(map[string]chan func())
	q.counts = make(map[string]*int32)
	q.mu.Unlock()
}
