package bot

import (
    "context"

    "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
)

type WSClient interface {
    Start(ctx context.Context) error
}

type WSFactory func(appID, appSecret string, handler *dispatcher.EventDispatcher) WSClient
