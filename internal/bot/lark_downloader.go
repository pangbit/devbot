package bot

import (
	"context"
	"fmt"
	"io"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// LarkDownloader implements Downloader using the Lark SDK.
// It uses the MessageResource API for both images and files,
// since Image.Get only works for bot-uploaded images.
type LarkDownloader struct {
	client *lark.Client
}

func NewLarkDownloader(client *lark.Client) *LarkDownloader {
	return &LarkDownloader{client: client}
}

// DownloadImage downloads an image from a message using the MessageResource API.
// The Image.Get API only works for bot-uploaded images, so we use MessageResource.Get
// with type=image to download user-sent images.
func (d *LarkDownloader) DownloadImage(ctx context.Context, messageID, imageKey string) (io.ReadCloser, error) {
	req := larkim.NewGetMessageResourceReqBuilder().
		MessageId(messageID).
		FileKey(imageKey).
		Type("image").
		Build()

	resp, err := d.client.Im.MessageResource.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("lark API error: %w", err)
	}
	if !resp.Success() {
		return nil, fmt.Errorf("lark API failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	return io.NopCloser(resp.File), nil
}

// DownloadFile downloads a file from a message using the MessageResource API.
// Returns a reader and the server-provided filename.
func (d *LarkDownloader) DownloadFile(ctx context.Context, messageID, fileKey string) (io.ReadCloser, string, error) {
	req := larkim.NewGetMessageResourceReqBuilder().
		MessageId(messageID).
		FileKey(fileKey).
		Type("file").
		Build()

	resp, err := d.client.Im.MessageResource.Get(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("lark API error: %w", err)
	}
	if !resp.Success() {
		return nil, "", fmt.Errorf("lark API failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	return io.NopCloser(resp.File), resp.FileName, nil
}
