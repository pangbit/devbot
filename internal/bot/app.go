package bot

import (
    "context"

    larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
    "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
    larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
    larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

func defaultWSFactory(appID, appSecret string, handler *dispatcher.EventDispatcher) WSClient {
    client := larkws.NewClient(
        appID,
        appSecret,
        larkws.WithEventHandler(handler),
        larkws.WithLogLevel(larkcore.LogLevelDebug),
    )
    return client
}

func buildEventHandler(h *Handler) *dispatcher.EventDispatcher {
    return dispatcher.NewEventDispatcher("", "").
        OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
            return h.HandleMessage(ctx, event)
        })
}

func Run(ctx context.Context, cfg Config, h *Handler, factory WSFactory) error {
    if factory == nil {
        factory = defaultWSFactory
    }
    handler := buildEventHandler(h)
    client := factory(cfg.AppID, cfg.AppSecret, handler)

    // The Lark SDK's Start method blocks with select{} and ignores context
    // cancellation. Run it in a goroutine so we can return when ctx is done.
    errCh := make(chan error, 1)
    go func() {
        errCh <- client.Start(ctx)
    }()

    select {
    case err := <-errCh:
        return err
    case <-ctx.Done():
        return ctx.Err()
    }
}
