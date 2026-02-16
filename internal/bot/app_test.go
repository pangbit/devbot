package bot

import (
	"context"
	"testing"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
)

type fakeWS struct{ started bool }

func (f *fakeWS) Start(_ context.Context) error {
	f.started = true
	return nil
}

func TestRunStartsWS(t *testing.T) {
	cfg := Config{AppID: "app", AppSecret: "secret", AllowedUserIDs: map[string]bool{}, SkipBotSelf: true}
	router := &fakeRouter{}
	h := NewHandler(router, true, "bot_id")

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
