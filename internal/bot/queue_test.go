package bot

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestQueueSequentialExecution(t *testing.T) {
	q := NewMessageQueue()

	var order []int
	var mu sync.Mutex

	done := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		for i := 0; i < 3; i++ {
			i := i
			wg.Add(1)
			q.Enqueue("chat1", func() {
				defer wg.Done()
				time.Sleep(10 * time.Millisecond)
				mu.Lock()
				order = append(order, i)
				mu.Unlock()
			})
		}
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(order))
	}
	for i, v := range order {
		if v != i {
			t.Fatalf("tasks not sequential: %v", order)
		}
	}
}

func TestQueueDifferentChatsParallel(t *testing.T) {
	q := NewMessageQueue()

	var running int32
	var maxRunning int32

	var wg sync.WaitGroup
	for _, chatID := range []string{"chat1", "chat2"} {
		chatID := chatID
		wg.Add(1)
		q.Enqueue(chatID, func() {
			defer wg.Done()
			cur := atomic.AddInt32(&running, 1)
			for {
				old := atomic.LoadInt32(&maxRunning)
				if cur > old {
					if atomic.CompareAndSwapInt32(&maxRunning, old, cur) {
						break
					}
				} else {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&running, -1)
		})
	}
	wg.Wait()

	if atomic.LoadInt32(&maxRunning) < 2 {
		t.Fatalf("expected parallel execution for different chats")
	}
}

func TestQueueShutdown(t *testing.T) {
	q := NewMessageQueue()

	done := make(chan struct{})
	q.Enqueue("chat1", func() {
		// This task runs before shutdown
		close(done)
	})
	<-done

	q.Shutdown()

	// After shutdown, new enqueue should work on a fresh worker
	done2 := make(chan struct{})
	q.Enqueue("chat1", func() {
		close(done2)
	})

	select {
	case <-done2:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout: task should run after shutdown and re-enqueue")
	}
}

func TestQueuePendingCount(t *testing.T) {
	q := NewMessageQueue()

	started := make(chan struct{})
	release := make(chan struct{})

	q.Enqueue("chat1", func() {
		close(started)
		<-release
	})

	<-started

	q.Enqueue("chat1", func() {})

	count := q.PendingCount("chat1")
	if count < 1 {
		t.Fatalf("expected at least 1 pending, got %d", count)
	}

	close(release)
	time.Sleep(100 * time.Millisecond)
}
