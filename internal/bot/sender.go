package bot

import (
    "context"
    "encoding/json"
    "log"
    "unicode/utf8"

    lark "github.com/larksuite/oapi-sdk-go/v3"
    larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

const MaxMessageLen = 4000

type LarkSender struct {
	client *lark.Client
}

func NewLarkSender(client *lark.Client) *LarkSender {
    return &LarkSender{client: client}
}

func buildSendMessageBody(chatID, text string) map[string]interface{} {
    // Use json.Marshal for proper escaping of newlines, quotes, etc.
    // The SDK's TextMsgBuilder.Text() does NOT escape special characters.
    content, _ := json.Marshal(map[string]string{"text": text})
    return map[string]interface{}{
        "receive_id": chatID,
        "msg_type":   "text",
        "content":    string(content),
    }
}

func (s *LarkSender) SendText(ctx context.Context, chatID, text string) error {
	body := buildSendMessageBody(chatID, text)
	resp, err := s.client.Post(
		ctx,
		"https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id",
		body,
		larkcore.AccessTokenTypeTenant,
	)
	if err != nil {
		log.Printf("sender: SendText failed chat=%s: %v", chatID, err)
		return err
	}
	if resp != nil && resp.StatusCode != 200 {
		log.Printf("sender: SendText non-200 chat=%s status=%d body=%s", chatID, resp.StatusCode, string(resp.RawBody))
	} else if resp != nil {
		// Check for API-level errors in response body
		var codeErr struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
		}
		if json.Unmarshal(resp.RawBody, &codeErr) == nil && codeErr.Code != 0 {
			log.Printf("sender: SendText API error chat=%s code=%d msg=%s", chatID, codeErr.Code, codeErr.Msg)
		}
	}
	return nil
}

func SplitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLen {
			chunks = append(chunks, text)
			break
		}

		// Find a good cut point within maxLen bytes
		cutAt := maxLen
		// Don't cut in the middle of a UTF-8 character
		for cutAt > 0 && cutAt < len(text) && !utf8.RuneStart(text[cutAt]) {
			cutAt--
		}

		// Try to find a newline to split at
		lastNewline := -1
		for i := 0; i < cutAt; i++ {
			if text[i] == '\n' {
				lastNewline = i + 1
			}
		}
		if lastNewline > 0 {
			cutAt = lastNewline
		}

		chunks = append(chunks, text[:cutAt])
		text = text[cutAt:]
	}
	return chunks
}

func (s *LarkSender) SendTextChunked(ctx context.Context, chatID, text string) error {
	chunks := SplitMessage(text, MaxMessageLen)
	for _, chunk := range chunks {
		if err := s.SendText(ctx, chatID, chunk); err != nil {
			return err
		}
	}
	return nil
}
