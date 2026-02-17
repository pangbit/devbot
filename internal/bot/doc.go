package bot

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
)

const maxBlocksPerRequest = 50 // Feishu API limit for DocumentBlockChildren.Create

// DocPusher is the interface used by the router for document operations.
// It abstracts away the Lark DocX API so that tests can use a fake implementation.
type DocPusher interface {
	CreateAndPushDoc(ctx context.Context, title, content string) (docID, docURL string, err error)
	PullDocContent(ctx context.Context, docID string) (string, error)
}

// DocSyncer implements DocPusher using the Lark DocX API.
type DocSyncer struct {
	client *lark.Client
}

// NewDocSyncer creates a new DocSyncer backed by the given Lark client.
func NewDocSyncer(client *lark.Client) *DocSyncer {
	return &DocSyncer{client: client}
}

// CreateAndPushDoc creates a new Feishu document with the given title, then
// inserts the content as text paragraph blocks. Returns the document ID and URL.
func (d *DocSyncer) CreateAndPushDoc(ctx context.Context, title, content string) (string, string, error) {
	// 1. Create the document
	createReq := larkdocx.NewCreateDocumentReqBuilder().
		Body(larkdocx.NewCreateDocumentReqBodyBuilder().
			Title(title).
			Build()).
		Build()

	createResp, err := d.client.Docx.Document.Create(ctx, createReq)
	if err != nil {
		return "", "", fmt.Errorf("create document: %w", err)
	}
	if !createResp.Success() {
		return "", "", fmt.Errorf("create document failed: code=%d msg=%s", createResp.Code, createResp.Msg)
	}

	if createResp.Data == nil || createResp.Data.Document == nil || createResp.Data.Document.DocumentId == nil {
		return "", "", fmt.Errorf("create document: response missing document ID")
	}
	docID := *createResp.Data.Document.DocumentId
	docURL := fmt.Sprintf("https://feishu.cn/docx/%s", docID)

	// 2. Insert content as paragraph blocks (max 50 per API call)
	if content != "" {
		blocks := buildParagraphBlocks(content)
		for i := 0; i < len(blocks); i += maxBlocksPerRequest {
			end := i + maxBlocksPerRequest
			if end > len(blocks) {
				end = len(blocks)
			}
			batch := blocks[i:end]

			childrenReq := larkdocx.NewCreateDocumentBlockChildrenReqBuilder().
				DocumentId(docID).
				BlockId(docID).
				DocumentRevisionId(-1).
				Body(larkdocx.NewCreateDocumentBlockChildrenReqBodyBuilder().
					Children(batch).
					Index(-1).
					Build()).
				Build()

			childResp, err := d.client.Docx.DocumentBlockChildren.Create(ctx, childrenReq)
			if err != nil {
				return docID, docURL, fmt.Errorf("insert blocks (batch %d): %w", i/maxBlocksPerRequest, err)
			}
			if !childResp.Success() {
				return docID, docURL, fmt.Errorf("insert blocks failed (batch %d): code=%d msg=%s", i/maxBlocksPerRequest, childResp.Code, childResp.Msg)
			}
		}
	}

	return docID, docURL, nil
}

// PullDocContent retrieves the raw text content of a Feishu document.
func (d *DocSyncer) PullDocContent(ctx context.Context, docID string) (string, error) {
	req := larkdocx.NewRawContentDocumentReqBuilder().
		DocumentId(docID).
		Build()

	resp, err := d.client.Docx.Document.RawContent(ctx, req)
	if err != nil {
		return "", fmt.Errorf("get raw content: %w", err)
	}
	if !resp.Success() {
		return "", fmt.Errorf("get raw content failed: code=%d msg=%s", resp.Code, resp.Msg)
	}

	if resp.Data == nil || resp.Data.Content == nil {
		return "", nil
	}
	return *resp.Data.Content, nil
}

// buildParagraphBlocks splits content by newlines and creates a text block for each line.
// Empty lines are converted to blocks with a single space, since the Feishu API
// rejects empty TextRun content (error 99992402 "field validation failed").
func buildParagraphBlocks(content string) []*larkdocx.Block {
	lines := strings.Split(content, "\n")
	blocks := make([]*larkdocx.Block, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			line = " "
		}
		textElement := larkdocx.NewTextElementBuilder().
			TextRun(larkdocx.NewTextRunBuilder().Content(line).Build()).
			Build()

		block := larkdocx.NewBlockBuilder().
			BlockType(2). // 2 = Text block
			Text(larkdocx.NewTextBuilder().
				Elements([]*larkdocx.TextElement{textElement}).
				Build()).
			Build()
		blocks = append(blocks, block)
	}
	return blocks
}

// ParseDocID extracts a document ID from a Feishu URL or returns the raw ID if
// it does not look like a URL. Supported URL formats:
//   - https://xxx.feishu.cn/docx/DOCID
//   - https://xxx.feishu.cn/docx/DOCID?query...
//   - Raw doc ID string (returned as-is)
func ParseDocID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// If it does not look like a URL, treat it as a raw doc ID.
	if !strings.Contains(raw, "://") {
		return raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	// Path is expected to be /docx/DOCID or /docx/DOCID/
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "docx" {
		return parts[1]
	}

	// Fallback: return the last non-empty path segment
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return raw
}
