package bot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- ParseDocID unit tests ---

func TestParseDocID_RawID(t *testing.T) {
	got := ParseDocID("doxcnSS4ouQkQEouGSUkTg9NJPe")
	if got != "doxcnSS4ouQkQEouGSUkTg9NJPe" {
		t.Fatalf("expected raw doc ID, got %q", got)
	}
}

func TestParseDocID_URL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.feishu.cn/docx/ABC123", "ABC123"},
		{"https://foo.feishu.cn/docx/ABC123/", "ABC123"},
		{"https://bar.feishu.cn/docx/XYZ?query=1", "XYZ"},
		{"https://baz.larksuite.com/docx/DOC456", "DOC456"},
	}
	for _, tt := range tests {
		got := ParseDocID(tt.input)
		if got != tt.want {
			t.Errorf("ParseDocID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseDocID_Empty(t *testing.T) {
	got := ParseDocID("")
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestParseDocID_Whitespace(t *testing.T) {
	got := ParseDocID("  ABC123  ")
	if got != "ABC123" {
		t.Fatalf("expected trimmed ID, got %q", got)
	}
}

func TestParseDocID_FallbackLastSegment(t *testing.T) {
	// Non-docx URL: should fall back to returning the last non-empty path segment.
	tests := []struct {
		input string
		want  string
	}{
		{"https://feishu.cn/wiki/DOC123", "DOC123"},
		{"https://feishu.cn/drive/home/FILEID", "FILEID"},
		{"https://feishu.cn/wiki/DOC123/", "DOC123"},
		// Root URL with no path segments — falls back to returning raw URL
		{"https://feishu.cn/", "https://feishu.cn/"},
	}
	for _, tt := range tests {
		got := ParseDocID(tt.input)
		if got != tt.want {
			t.Errorf("ParseDocID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseDocID_URLParseError(t *testing.T) {
	// url.Parse returns error for malformed URLs — should return raw string
	malformed := "://invalid"
	got := ParseDocID(malformed)
	if got != malformed {
		t.Fatalf("expected raw string on url.Parse error, got %q", got)
	}
}

// --- Fake DocPusher for router tests ---

type fakeDocPusher struct {
	createdTitle   string
	createdContent string
	returnDocID    string
	returnDocURL   string
	returnErr      error

	pullDocID      string
	pullContent    string
	pullErr        error
}

func (f *fakeDocPusher) CreateAndPushDoc(_ context.Context, title, content string) (string, string, error) {
	f.createdTitle = title
	f.createdContent = content
	if f.returnErr != nil {
		return "", "", f.returnErr
	}
	return f.returnDocID, f.returnDocURL, nil
}

func (f *fakeDocPusher) PullDocContent(_ context.Context, docID string) (string, error) {
	f.pullDocID = docID
	if f.pullErr != nil {
		return "", f.pullErr
	}
	return f.pullContent, nil
}

func newTestRouterWithDoc(t *testing.T, dp DocPusher) (*Router, *spySender, string) {
	t.Helper()
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "project1"), 0755)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\nHello World"), 0644)

	store, err := NewStore(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	sender := &spySender{}
	exec := NewClaudeExecutor("claude", "sonnet", 10*time.Second)
	r := NewRouter(context.Background(), exec, store, sender, map[string]bool{"user1": true}, dir, dp)
	return r, sender, dir
}

// --- /doc push tests ---

func TestDocPush_Success(t *testing.T) {
	fake := &fakeDocPusher{
		returnDocID:  "doc123",
		returnDocURL: "https://feishu.cn/docx/doc123",
	}
	r, sender, dir := newTestRouterWithDoc(t, fake)

	// Create a test file
	os.WriteFile(filepath.Join(dir, "test.md"), []byte("line1\nline2"), 0644)

	r.Route(context.Background(), "chat1", "user1", "/doc push test.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "doc123") {
		t.Fatalf("expected doc ID in response, got: %q", msg)
	}
	if !strings.Contains(msg, "https://feishu.cn/docx/doc123") {
		t.Fatalf("expected doc URL in response, got: %q", msg)
	}
	if fake.createdTitle != "test.md" {
		t.Fatalf("expected title 'test.md', got %q", fake.createdTitle)
	}
	if fake.createdContent != "line1\nline2" {
		t.Fatalf("expected content 'line1\\nline2', got %q", fake.createdContent)
	}

	// Verify binding was stored
	bindings := r.store.DocBindings()
	boundPath := filepath.Join(dir, "test.md")
	if bindings[boundPath] != "doc123" {
		t.Fatalf("expected binding for %s -> doc123, got %v", boundPath, bindings)
	}
}

func TestDocPush_FileNotFound(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, _ := newTestRouterWithDoc(t, fake)

	r.Route(context.Background(), "chat1", "user1", "/doc push nonexistent.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "不存在") && !strings.Contains(msg, "出错") {
		t.Fatalf("expected error for missing file, got: %q", msg)
	}
}

func TestDocPush_NoArgs(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, _ := newTestRouterWithDoc(t, fake)

	r.Route(context.Background(), "chat1", "user1", "/doc push")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "用法") {
		t.Fatalf("expected usage message, got: %q", msg)
	}
}

func TestDocPush_APIError(t *testing.T) {
	fake := &fakeDocPusher{
		returnErr: fmt.Errorf("API rate limit"),
	}
	r, sender, dir := newTestRouterWithDoc(t, fake)
	os.WriteFile(filepath.Join(dir, "test.md"), []byte("content"), 0644)

	r.Route(context.Background(), "chat1", "user1", "/doc push test.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "出错") || !strings.Contains(msg, "API rate limit") {
		t.Fatalf("expected API error message, got: %q", msg)
	}
}

// --- /doc pull tests ---

func TestDocPull_Success(t *testing.T) {
	fake := &fakeDocPusher{
		pullContent: "pulled content here",
	}
	r, sender, dir := newTestRouterWithDoc(t, fake)

	// Set up a binding
	filePath := filepath.Join(dir, "test.md")
	os.WriteFile(filePath, []byte("old content"), 0644)
	r.store.SetDocBinding(filePath, "doc456")

	r.Route(context.Background(), "chat1", "user1", "/doc pull test.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "文档已拉取") {
		t.Fatalf("expected pull confirmation, got: %q", msg)
	}

	// Verify the file was overwritten
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "pulled content here" {
		t.Fatalf("expected 'pulled content here', got %q", string(data))
	}
	if fake.pullDocID != "doc456" {
		t.Fatalf("expected pull from doc456, got %q", fake.pullDocID)
	}
}

func TestDocPull_NoBinding(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, _ := newTestRouterWithDoc(t, fake)

	r.Route(context.Background(), "chat1", "user1", "/doc pull test.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "未找到") {
		t.Fatalf("expected no binding message, got: %q", msg)
	}
}

func TestDocPull_NoArgs(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, _ := newTestRouterWithDoc(t, fake)

	r.Route(context.Background(), "chat1", "user1", "/doc pull")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "用法") {
		t.Fatalf("expected usage message, got: %q", msg)
	}
}

func TestDocPull_APIError(t *testing.T) {
	fake := &fakeDocPusher{
		pullErr: fmt.Errorf("network timeout"),
	}
	r, sender, dir := newTestRouterWithDoc(t, fake)
	filePath := filepath.Join(dir, "test.md")
	os.WriteFile(filePath, []byte("old"), 0644)
	r.store.SetDocBinding(filePath, "doc789")

	r.Route(context.Background(), "chat1", "user1", "/doc pull test.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "出错") || !strings.Contains(msg, "network timeout") {
		t.Fatalf("expected API error, got: %q", msg)
	}
}

// --- /doc bind tests ---

func TestDocBind_WithURL(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, dir := newTestRouterWithDoc(t, fake)

	r.Route(context.Background(), "chat1", "user1", "/doc bind test.md https://example.feishu.cn/docx/ABC123")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "已绑定") {
		t.Fatalf("expected bound confirmation, got: %q", msg)
	}

	bindings := r.store.DocBindings()
	boundPath := filepath.Join(dir, "test.md")
	if bindings[boundPath] != "ABC123" {
		t.Fatalf("expected binding ABC123, got %v", bindings)
	}
}

func TestDocBind_WithRawID(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, dir := newTestRouterWithDoc(t, fake)

	r.Route(context.Background(), "chat1", "user1", "/doc bind test.md DOC999")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "已绑定") {
		t.Fatalf("expected bound confirmation, got: %q", msg)
	}

	bindings := r.store.DocBindings()
	boundPath := filepath.Join(dir, "test.md")
	if bindings[boundPath] != "DOC999" {
		t.Fatalf("expected binding DOC999, got %v", bindings)
	}
}

func TestDocBind_NoArgs(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, _ := newTestRouterWithDoc(t, fake)

	r.Route(context.Background(), "chat1", "user1", "/doc bind")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "用法") {
		t.Fatalf("expected usage message, got: %q", msg)
	}
}

func TestDocBind_MissingDocID(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, _ := newTestRouterWithDoc(t, fake)

	r.Route(context.Background(), "chat1", "user1", "/doc bind test.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "用法") {
		t.Fatalf("expected usage message, got: %q", msg)
	}
}

// --- /doc unbind tests ---

func TestDocUnbind(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, dir := newTestRouterWithDoc(t, fake)
	filePath := filepath.Join(dir, "test.md")
	r.store.SetDocBinding(filePath, "doc123")

	r.Route(context.Background(), "chat1", "user1", "/doc unbind test.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "已解除绑定") {
		t.Fatalf("expected unbound confirmation, got: %q", msg)
	}

	bindings := r.store.DocBindings()
	if _, ok := bindings[filePath]; ok {
		t.Fatalf("expected binding to be removed, got %v", bindings)
	}
}

func TestDocUnbind_NotFound(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, _ := newTestRouterWithDoc(t, fake)

	r.Route(context.Background(), "chat1", "user1", "/doc unbind nonexistent_binding.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "未找到") {
		t.Fatalf("expected 'No binding found' message, got: %q", msg)
	}
}

func TestDocUnbind_NoArgs(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, _ := newTestRouterWithDoc(t, fake)

	r.Route(context.Background(), "chat1", "user1", "/doc unbind")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "用法") {
		t.Fatalf("expected usage message, got: %q", msg)
	}
}

// --- /doc list tests ---

func TestDocList_Empty(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, _ := newTestRouterWithDoc(t, fake)

	r.Route(context.Background(), "chat1", "user1", "/doc list")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "暂无绑定") {
		t.Fatalf("expected no bindings message, got: %q", msg)
	}
}

func TestDocList_WithBindings(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, _ := newTestRouterWithDoc(t, fake)
	r.store.SetDocBinding("/path/to/file.md", "doc111")
	r.store.SetDocBinding("/path/to/other.md", "doc222")

	r.Route(context.Background(), "chat1", "user1", "/doc list")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "file.md") || !strings.Contains(msg, "doc111") {
		t.Fatalf("expected file.md -> doc111 in list, got: %q", msg)
	}
	if !strings.Contains(msg, "other.md") || !strings.Contains(msg, "doc222") {
		t.Fatalf("expected other.md -> doc222 in list, got: %q", msg)
	}
}

// --- /doc with nil DocPusher ---

func TestDoc_NotConfigured(t *testing.T) {
	r, sender, _ := newTestRouterWithDoc(t, nil)

	r.Route(context.Background(), "chat1", "user1", "/doc push test.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "未配置") {
		t.Fatalf("expected not configured message, got: %q", msg)
	}
}

func TestDoc_NotConfigured_ListStillWorks(t *testing.T) {
	r, sender, _ := newTestRouterWithDoc(t, nil)

	// list, bind, unbind should still work without DocPusher
	r.Route(context.Background(), "chat1", "user1", "/doc list")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "暂无绑定") {
		t.Fatalf("expected no bindings, got: %q", msg)
	}
}

// --- /doc unknown subcommand ---

func TestDoc_UnknownSubcommand(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, _ := newTestRouterWithDoc(t, fake)

	r.Route(context.Background(), "chat1", "user1", "/doc foo")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "未知的 doc 子命令") {
		t.Fatalf("expected unknown subcommand message, got: %q", msg)
	}
}

func TestDoc_NoSubcommand(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, _ := newTestRouterWithDoc(t, fake)

	r.Route(context.Background(), "chat1", "user1", "/doc")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "用法") || !strings.Contains(msg, "doc") {
		t.Fatalf("expected usage message, got: %q", msg)
	}
}

func TestDocPush_FuzzyPath(t *testing.T) {
	fake := &fakeDocPusher{
		returnDocID:  "doc_fuzzy",
		returnDocURL: "https://feishu.cn/docx/doc_fuzzy",
	}
	r, sender, _ := newTestRouterWithDoc(t, fake)

	// "readme" should fuzzy-match "README.md"
	r.Route(context.Background(), "chat1", "user1", "/doc push readme")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "doc_fuzzy") {
		t.Fatalf("expected fuzzy match to work for /doc push, got: %q", msg)
	}
	if fake.createdTitle != "README.md" {
		t.Fatalf("expected title 'README.md', got %q", fake.createdTitle)
	}
}

func TestDocBind_FuzzyPath(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, dir := newTestRouterWithDoc(t, fake)

	// Bind using fuzzy path
	r.Route(context.Background(), "chat1", "user1", "/doc bind readme DOC999")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "DOC999") {
		t.Fatalf("expected bind confirmation, got: %q", msg)
	}

	// Verify binding was created with resolved path
	bindings := r.store.DocBindings()
	expectedPath := filepath.Join(dir, "README.md")
	if bindings[expectedPath] != "DOC999" {
		t.Fatalf("expected binding for %s, got bindings: %v", expectedPath, bindings)
	}
}

func TestDocPull_WriteFileError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; permission-based tests are unreliable")
	}
	fake := &fakeDocPusher{pullContent: "new content"}
	r, sender, dir := newTestRouterWithDoc(t, fake)

	// Create a binding for a target file, then make it non-writable.
	filePath := filepath.Join(dir, "README.md")
	r.store.SetDocBinding(filePath, "doc_write_err")

	// Remove write permission from the file itself so WriteFile fails.
	os.Chmod(filePath, 0000)
	defer os.Chmod(filePath, 0644)

	r.Route(context.Background(), "chat1", "user1", "/doc pull README.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "写入文件出错") {
		t.Fatalf("expected write error message, got: %q", msg)
	}
}

func TestDocPull_OutsideRoot(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, _ := newTestRouterWithDoc(t, fake)

	// Create a binding that points outside the work root using a unique name so
	// the fuzzy match in findDocBinding will find it.
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "outsidefile_zzz.md")
	os.WriteFile(outsidePath, []byte("outside content"), 0644)
	r.store.SetDocBinding(outsidePath, "doc_outside")

	// Query by filename — fuzzy match finds it, but underRoot check rejects it.
	r.Route(context.Background(), "chat1", "user1", "/doc pull outsidefile_zzz.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "不允许访问工作根目录以外") {
		t.Fatalf("expected outside work root rejection, got: %q", msg)
	}
}

func TestDocPush_AbsolutePath(t *testing.T) {
	// Absolute paths should be rejected because resolveFilePath returns "" for them.
	fake := &fakeDocPusher{}
	r, sender, _ := newTestRouterWithDoc(t, fake)

	r.Route(context.Background(), "chat1", "user1", "/doc push /absolute/path/file.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "不存在") && !strings.Contains(msg, "出错") {
		t.Fatalf("expected error for absolute path, got: %q", msg)
	}
}

func TestDocPull_FuzzyBinding(t *testing.T) {
	fake := &fakeDocPusher{
		pullContent: "pulled content",
	}
	r, sender, dir := newTestRouterWithDoc(t, fake)

	// Create a binding with full path
	fullPath := filepath.Join(dir, "README.md")
	r.store.SetDocBinding(fullPath, "DOC_PULL")
	r.store.Save()

	// Pull using fuzzy path "readme"
	r.Route(context.Background(), "chat1", "user1", "/doc pull readme")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "文档已拉取") {
		t.Fatalf("expected pull confirmation, got: %q", msg)
	}

	// Verify file was written
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
	if string(data) != "pulled content" {
		t.Fatalf("expected 'pulled content', got %q", string(data))
	}
}

func TestDocPull_NotConfigured(t *testing.T) {
	// /doc pull with nil DocPusher should report "not configured".
	r, sender, _ := newTestRouterWithDoc(t, nil)
	r.Route(context.Background(), "chat1", "user1", "/doc pull test.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "未配置") {
		t.Fatalf("expected 'not configured' message, got: %q", msg)
	}
}

func TestDocPush_OutsideRoot(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, _ := newTestRouterWithDoc(t, fake)

	// Create the session first, then point WorkDir outside the work root.
	r.getSession("chat1")
	outsideDir := t.TempDir()
	os.WriteFile(filepath.Join(outsideDir, "outside.md"), []byte("content"), 0644)
	r.store.UpdateSession("chat1", func(s *Session) {
		s.WorkDir = outsideDir
	})

	r.Route(context.Background(), "chat1", "user1", "/doc push outside.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "不允许访问工作根目录以外") {
		t.Fatalf("expected outside root rejection, got: %q", msg)
	}
}

func TestDocPush_ReadError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root; permission-based tests are unreliable")
	}
	fake := &fakeDocPusher{}
	r, sender, dir := newTestRouterWithDoc(t, fake)

	filePath := filepath.Join(dir, "secret.md")
	os.WriteFile(filePath, []byte("secret content"), 0644)
	os.Chmod(filePath, 0000)
	defer os.Chmod(filePath, 0644)

	r.Route(context.Background(), "chat1", "user1", "/doc push secret.md")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "读取文件出错") {
		t.Fatalf("expected read error message, got: %q", msg)
	}
}

func TestDocBind_OutsideRoot(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, _ := newTestRouterWithDoc(t, fake)

	// Create the session first, then point WorkDir outside the work root.
	r.getSession("chat1")
	outsideDir := t.TempDir()
	r.store.UpdateSession("chat1", func(s *Session) {
		s.WorkDir = outsideDir
	})

	r.Route(context.Background(), "chat1", "user1", "/doc bind file.md DOCID123")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "不允许访问工作根目录以外") {
		t.Fatalf("expected outside root rejection, got: %q", msg)
	}
}

func TestDocUnbind_FuzzyBinding(t *testing.T) {
	fake := &fakeDocPusher{}
	r, sender, dir := newTestRouterWithDoc(t, fake)

	fullPath := filepath.Join(dir, "README.md")
	r.store.SetDocBinding(fullPath, "DOC_UNBIND")
	r.store.Save()

	// Unbind using fuzzy path
	r.Route(context.Background(), "chat1", "user1", "/doc unbind readme")
	msg := sender.LastMessage()
	if !strings.Contains(msg, "已解除绑定") {
		t.Fatalf("expected unbind confirmation, got: %q", msg)
	}

	bindings := r.store.DocBindings()
	if _, ok := bindings[fullPath]; ok {
		t.Fatalf("expected binding to be removed")
	}
}
