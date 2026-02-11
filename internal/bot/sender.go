package bot

import (
    "context"

    lark "github.com/larksuite/oapi-sdk-go/v3"
    larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
    larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type LarkSender struct {
    client *lark.Client
}

func NewLarkSender(client *lark.Client) *LarkSender {
    return &LarkSender{client: client}
}

func buildSendMessageBody(chatID, text string) map[string]interface{} {
    content := larkim.NewTextMsgBuilder().Text(text).Build()
    return map[string]interface{}{
        "receive_id": chatID,
        "msg_type":   "text",
        "content":    content,
    }
}

func (s *LarkSender) SendText(ctx context.Context, chatID, text string) error {
    body := buildSendMessageBody(chatID, text)
    _, err := s.client.Post(
        ctx,
        "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id",
        body,
        larkcore.AccessTokenTypeTenant,
    )
    return err
}
