package bot

import (
	"context"
	"testing"
	"time"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
)

type fakeWS struct{ started bool }

func (f *fakeWS) Start(_ context.Context) error {
	f.started = true
	return nil
}

// blockingWS simulates the real Lark SDK behavior: Start blocks forever with select{}.
// startedCh is closed when Start is called, allowing the test to synchronize safely.
type blockingWS struct {
	startedCh chan struct{}
}

func (b *blockingWS) Start(_ context.Context) error {
	close(b.startedCh) // signal that Start was called
	select {}          // mimics the real SDK
}

func TestRunStartsWS(t *testing.T) {
	cfg := Config{AppID: "app", AppSecret: "secret", AllowedUserIDs: map[string]bool{}, SkipBotSelf: true}
	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", nil)

	var gotAppID, gotSecret string
	var gotHandler *dispatcher.EventDispatcher

	ws := &fakeWS{}
	factory := func(appID, appSecret string, handler *dispatcher.EventDispatcher) WSClient {
		gotAppID = appID
		gotSecret = appSecret
		gotHandler = handler
		return ws
	}

	if err := Run(context.Background(), cfg, h, factory); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !ws.started {
		t.Fatalf("expected ws to start")
	}
	if gotAppID != "app" || gotSecret != "secret" || gotHandler == nil {
		t.Fatalf("factory args not set correctly")
	}
}

func TestRunReturnsOnContextCancel(t *testing.T) {
	cfg := Config{AppID: "app", AppSecret: "secret", AllowedUserIDs: map[string]bool{}, SkipBotSelf: true}
	router := &fakeRouter{}
	h := NewHandler(router, nil, nil, true, "bot_id", nil)

	ws := &blockingWS{startedCh: make(chan struct{})}
	factory := func(appID, appSecret string, handler *dispatcher.EventDispatcher) WSClient {
		return ws
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, cfg, h, factory)
	}()

	// Wait for ws.Start to be called (race-free via channel)
	select {
	case <-ws.startedCh:
	case <-time.After(2 * time.Second):
		t.Fatal("expected ws to start within 2s")
	}

	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("expected context.Canceled, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancellation (would hang with old code)")
	}
}
