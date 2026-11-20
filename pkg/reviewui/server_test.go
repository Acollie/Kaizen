package reviewui

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alexcollie/kaizen/pkg/review"
)

type testServer struct {
	server  *Server
	baseURL string // http://127.0.0.1:port
	events  *EventLog
	poster  *recordingPoster
}

type recordingPoster struct {
	requests []review.PostRequest
	fail     bool
}

func (poster *recordingPoster) PostReview(ctx context.Context, pr review.PullRequest, files []review.FileDiff, request review.PostRequest) (*review.PostResult, error) {
	if poster.fail {
		return nil, errors.New("github said no")
	}
	poster.requests = append(poster.requests, request)
	return &review.PostResult{URL: "https://example.test/review/1", InlineComments: len(request.Findings)}, nil
}

func loadFixture(t *testing.T) *review.ReviewContext {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("testdata", "fixture_context.json"))
	if err != nil {
		t.Fatal(err)
	}
	reviewContext := &review.ReviewContext{}
	if err := json.Unmarshal(content, reviewContext); err != nil {
		t.Fatal(err)
	}
	worktree, err := filepath.Abs(filepath.Join("testdata", "worktree"))
	if err != nil {
		t.Fatal(err)
	}
	reviewContext.WorktreePath = worktree
	return reviewContext
}

func startServer(t *testing.T, reviewContext *review.ReviewContext) *testServer {
	t.Helper()
	events := NewEventLog()
	poster := &recordingPoster{}
	server, err := New(Options{Context: reviewContext, Events: events, Poster: poster})
	if err != nil {
		t.Fatal(err)
	}
	server.heartbeat = 50 * time.Millisecond
	pageURL, err := server.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	})
	parsed, _ := url.Parse(pageURL)
	if parsed.Query().Get("t") != server.Token() || len(server.Token()) != 64 {
		t.Fatalf("URL %s does not carry a 32-byte hex token", pageURL)
	}
	return &testServer{server: server, baseURL: "http://" + parsed.Host, events: events, poster: poster}
}

func (ts *testServer) get(t *testing.T, path string, headers map[string]string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, ts.baseURL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func (ts *testServer) withToken(path string) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + "t=" + ts.server.Token()
}

func TestNewRequiresContext(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatal("expected error without context")
	}
}

func TestIndexAndStaticServedWithCSP(t *testing.T) {
	ts := startServer(t, loadFixture(t))
	response := ts.get(t, "/", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET / = %d", response.StatusCode)
	}
	csp := response.Header.Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'self'", "script-src 'self' https://cdn.jsdelivr.net"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP %q missing %q", csp, want)
		}
	}
	if strings.Contains(csp, "unsafe-inline") {
		t.Errorf("CSP must not allow inline scripts: %q", csp)
	}
	for _, asset := range []string{"/static/app.js", "/static/app.css", "/static/keys.js"} {
		if response := ts.get(t, asset, nil); response.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d", asset, response.StatusCode)
		}
	}
	if response := ts.get(t, "/static/", nil); response.StatusCode != http.StatusNotFound {
		t.Errorf("directory listing should 404, got %d", response.StatusCode)
	}
	if response := ts.get(t, "/nope", nil); response.StatusCode != http.StatusNotFound {
		t.Errorf("unknown path should 404, got %d", response.StatusCode)
	}
}

func TestKeymapModuleIsEmbeddedAndImported(t *testing.T) {
	ts := startServer(t, loadFixture(t))
	response := ts.get(t, "/static/keys.js", nil)
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "export") {
		t.Fatalf("keys.js not served as a module: %d", response.StatusCode)
	}
	if contentType := response.Header.Get("Content-Type"); !strings.Contains(contentType, "javascript") {
		t.Errorf("keys.js content type %q", contentType)
	}
	app, err := staticFiles.ReadFile("static/app.js")
	if err != nil || !strings.Contains(string(app), "./keys.js") {
		t.Error("app.js does not import ./keys.js")
	}
}

func TestAPIRequiresToken(t *testing.T) {
	ts := startServer(t, loadFixture(t))
	for _, path := range []string{"/api/context", "/api/events", "/api/file?path=pkg/churn/cache.go"} {
		if response := ts.get(t, path, nil); response.StatusCode != http.StatusForbidden {
			t.Errorf("%s without token = %d, want 403", path, response.StatusCode)
		}
		wrong := path + map[bool]string{true: "&", false: "?"}[strings.Contains(path, "?")] + "t=deadbeef"
		if response := ts.get(t, wrong, nil); response.StatusCode != http.StatusForbidden {
			t.Errorf("%s with wrong token = %d, want 403", path, response.StatusCode)
		}
	}
	response := ts.get(t, "/api/context", map[string]string{tokenHeader: ts.server.Token()})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("context with header token = %d", response.StatusCode)
	}
	var decoded review.ReviewContext
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil || decoded.PR.Number != 142 {
		t.Fatalf("decode context: %v (pr %d)", err, decoded.PR.Number)
	}
}

func TestHostHeaderChecked(t *testing.T) {
	ts := startServer(t, loadFixture(t))
	port := strings.TrimPrefix(ts.baseURL, "http://127.0.0.1:")
	cases := map[string]int{
		"127.0.0.1:" + port:     http.StatusOK,
		"localhost:" + port:     http.StatusOK,
		"evil.example:" + port:  http.StatusForbidden,
		"127.0.0.1:1":           http.StatusForbidden,
		"attacker.test":         http.StatusForbidden,
		"localhost.evil.test:1": http.StatusForbidden,
	}
	for host, want := range cases {
		request, _ := http.NewRequest(http.MethodGet, ts.baseURL+ts.withToken("/api/context"), nil)
		request.Host = host
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != want {
			t.Errorf("Host %q = %d, want %d", host, response.StatusCode, want)
		}
	}
}

// sseReader parses `id:`/`data:` frames from an event stream
type sseReader struct {
	scanner *bufio.Scanner
}

func (reader *sseReader) next(t *testing.T) (string, review.Event) {
	t.Helper()
	var id string
	var data string
	for reader.scanner.Scan() {
		line := reader.scanner.Text()
		switch {
		case strings.HasPrefix(line, "id: "):
			id = strings.TrimPrefix(line, "id: ")
		case strings.HasPrefix(line, "data: "):
			data = strings.TrimPrefix(line, "data: ")
		case line == "" && data != "":
			var event review.Event
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				t.Fatalf("bad event json %q: %v", data, err)
			}
			return id, event
		}
	}
	t.Fatalf("stream ended: %v", reader.scanner.Err())
	return "", review.Event{}
}

func openStream(t *testing.T, ts *testServer, path string, headers map[string]string) *sseReader {
	t.Helper()
	response := ts.get(t, ts.withToken(path), headers)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d", path, response.StatusCode)
	}
	if contentType := response.Header.Get("Content-Type"); contentType != "text/event-stream" {
		t.Fatalf("content type %q", contentType)
	}
	return &sseReader{scanner: bufio.NewScanner(response.Body)}
}

func TestEventsReplayAndLiveStream(t *testing.T) {
	ts := startServer(t, loadFixture(t))
	ts.events.Emit(statusEvent("first"))
	ts.events.Emit(statusEvent("second"))

	stream := openStream(t, ts, "/api/events", nil)
	if id, event := stream.next(t); id != "1" || event.Seq != 1 {
		t.Fatalf("first replayed event id=%s seq=%d", id, event.Seq)
	}
	if id, _ := stream.next(t); id != "2" {
		t.Fatalf("second replayed id=%s", id)
	}

	ts.events.Emit(review.NewEvent(review.EventFinding, review.Finding{ID: "f1", Title: "live"}))
	id, event := stream.next(t)
	if id != "3" || event.Type != review.EventFinding {
		t.Fatalf("live event id=%s type=%s", id, event.Type)
	}
	data, _ := json.Marshal(event.Data)
	if !strings.Contains(string(data), `"live"`) {
		t.Errorf("finding payload lost: %s", data)
	}
}

func TestEventsResumeFromLastEventIDAndSince(t *testing.T) {
	ts := startServer(t, loadFixture(t))
	for index := 0; index < 5; index++ {
		ts.events.Emit(statusEvent(fmt.Sprint(index)))
	}
	stream := openStream(t, ts, "/api/events?since=1", map[string]string{"Last-Event-ID": "3"})
	if id, _ := stream.next(t); id != "4" {
		t.Errorf("Last-Event-ID should win: got id %s", id)
	}
	stream = openStream(t, ts, "/api/events?since=4", nil)
	if id, _ := stream.next(t); id != "5" {
		t.Errorf("since=4 should resume at 5, got %s", id)
	}
}

func TestEventsHeartbeatAndFinishedLog(t *testing.T) {
	ts := startServer(t, loadFixture(t))
	response := ts.get(t, ts.withToken("/api/events"), nil)
	scanner := bufio.NewScanner(response.Body)
	sawHeartbeat := false
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), ": heartbeat") {
			sawHeartbeat = true
			break
		}
	}
	if !sawHeartbeat {
		t.Error("no heartbeat comment")
	}

	ts.events.Emit(statusEvent("only"))
	ts.events.Close()
	finished := ts.get(t, ts.withToken("/api/events"), map[string]string{"Last-Event-ID": "1"})
	if finished.StatusCode != http.StatusNoContent {
		t.Errorf("drained closed log should return 204, got %d", finished.StatusCode)
	}
}

type fileResponse struct {
	Lines []struct {
		N    int    `json:"n"`
		Text string `json:"text"`
	} `json:"lines"`
	Total int `json:"total"`
}

func getFile(t *testing.T, ts *testServer, query string) (int, fileResponse) {
	t.Helper()
	response := ts.get(t, ts.withToken("/api/file?"+query), nil)
	var decoded fileResponse
	if response.StatusCode == http.StatusOK {
		if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
			t.Fatal(err)
		}
	}
	return response.StatusCode, decoded
}

func TestFileRangesAndBounds(t *testing.T) {
	ts := startServer(t, loadFixture(t))

	status, body := getFile(t, ts, "path=pkg/churn/analyzer.go&from=40&to=42")
	if status != http.StatusOK || len(body.Lines) != 3 || body.Lines[0].N != 40 || body.Total != 188 {
		t.Fatalf("range 40-42: status %d body %+v", status, body)
	}
	if !strings.Contains(body.Lines[0].Text, "func (analyzer *Analyzer) GetFileChurn") {
		t.Errorf("line 40 = %q", body.Lines[0].Text)
	}

	status, body = getFile(t, ts, "path=pkg/churn/analyzer.go&from=180&to=999")
	if status != http.StatusOK || len(body.Lines) != 9 || body.Lines[8].N != 188 {
		t.Errorf("range clamped to end: %d lines", len(body.Lines))
	}
	status, body = getFile(t, ts, "path=pkg/churn/analyzer.go&from=500&to=510")
	if status != http.StatusOK || len(body.Lines) != 0 {
		t.Errorf("past end should be empty, got %d", len(body.Lines))
	}
	_, body = getFile(t, ts, "path=pkg/churn/analyzer.go")
	if len(body.Lines) != 188 || body.Lines[0].N != 1 {
		t.Errorf("default range should be whole (small) file, got %d", len(body.Lines))
	}
	// A caller file outside the diff is allowed
	if status, _ := getFile(t, ts, "path=cmd/kaizen/main.go&from=1&to=5"); status != http.StatusOK {
		t.Errorf("caller file status %d", status)
	}
}

func TestFileRangeCappedAt500Lines(t *testing.T) {
	lines := make([]string, 2000)
	slice := sliceLines(lines, 10, 1900)
	if len(slice.Lines) != maxLinesPerRequest || slice.Lines[0].Number != 10 || slice.Total != 2000 {
		t.Errorf("cap: %d lines from %d", len(slice.Lines), slice.Lines[0].Number)
	}
}

func TestFileRejectsEscapesAndUnlistedFiles(t *testing.T) {
	ts := startServer(t, loadFixture(t))
	rejected := []string{
		"../server.go",
		"pkg/../../server.go",
		"/etc/passwd",
		url.QueryEscape("/etc/passwd"),
		"pkg/churn/../../../../server.go",
		`pkg\churn\cache.go`,
		"",
		"pkg/churn/missing.go",
		"pkg/churn", // a directory, and not listed
	}
	for _, path := range rejected {
		if status, _ := getFile(t, ts, "path="+path); status == http.StatusOK {
			t.Errorf("path %q should be rejected", path)
		}
	}
}

func TestFileRejectsSymlinkEscape(t *testing.T) {
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("top secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	worktree := t.TempDir()
	if err := os.WriteFile(filepath.Join(worktree, "real.go"), []byte("package real\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(worktree, "leak.go")); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}
	if err := os.Symlink(filepath.Join(worktree, "real.go"), filepath.Join(worktree, "inside.go")); err != nil {
		t.Fatal(err)
	}

	reviewContext := &review.ReviewContext{
		WorktreePath: worktree,
		Files: []review.FileDiff{
			{Path: "leak.go", Status: review.FileModified},
			{Path: "inside.go", Status: review.FileModified},
		},
	}
	ts := startServer(t, reviewContext)
	if status, _ := getFile(t, ts, "path=leak.go"); status == http.StatusOK {
		t.Error("symlink escaping the worktree must be rejected")
	}
	if status, body := getFile(t, ts, "path=inside.go"); status != http.StatusOK || body.Lines[0].Text != "package real" {
		t.Errorf("symlink inside the worktree should resolve, got %d", status)
	}
}

func post(t *testing.T, ts *testServer, body string, headers map[string]string) *http.Response {
	t.Helper()
	request, _ := http.NewRequest(http.MethodPost, ts.baseURL+"/api/post", strings.NewReader(body))
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func TestPostReview(t *testing.T) {
	ts := startServer(t, loadFixture(t))
	body := `{"summary":"LGTM","findings":[{"id":"f1","severity":"warning","file":"pkg/churn/cache.go","line":41,"title":"t","body":"b"}]}`
	good := map[string]string{tokenHeader: ts.server.Token(), "Content-Type": "application/json; charset=utf-8"}

	response := post(t, ts, body, good)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("post = %d", response.StatusCode)
	}
	var result review.PostResult
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil || result.URL == "" || result.InlineComments != 1 {
		t.Fatalf("result %+v err %v", result, err)
	}
	if len(ts.poster.requests) != 1 || ts.poster.requests[0].Summary != "LGTM" {
		t.Fatalf("poster got %+v", ts.poster.requests)
	}

	ts.poster.fail = true
	if response := post(t, ts, body, good); response.StatusCode != http.StatusBadGateway {
		t.Errorf("poster failure = %d, want 502", response.StatusCode)
	}
}

func TestPostReviewRejectsBadRequests(t *testing.T) {
	ts := startServer(t, loadFixture(t))
	token := ts.server.Token()
	cases := []struct {
		name    string
		path    string
		body    string
		headers map[string]string
		want    int
	}{
		{"no token", "", "{}", map[string]string{"Content-Type": "application/json"}, http.StatusForbidden},
		{"wrong token", "", "{}", map[string]string{tokenHeader: "nope", "Content-Type": "application/json"}, http.StatusForbidden},
		{"query token only", "?t=" + token, "{}", map[string]string{"Content-Type": "application/json"}, http.StatusForbidden},
		{"form content type", "", "{}", map[string]string{tokenHeader: token, "Content-Type": "application/x-www-form-urlencoded"}, http.StatusUnsupportedMediaType},
		{"text content type", "", "{}", map[string]string{tokenHeader: token, "Content-Type": "text/plain"}, http.StatusUnsupportedMediaType},
		{"foreign origin", "", "{}", map[string]string{tokenHeader: token, "Content-Type": "application/json", "Origin": "http://evil.test"}, http.StatusForbidden},
		{"bad json", "", "{", map[string]string{tokenHeader: token, "Content-Type": "application/json"}, http.StatusBadRequest},
	}
	for _, testCase := range cases {
		request, _ := http.NewRequest(http.MethodPost, ts.baseURL+"/api/post"+testCase.path, strings.NewReader(testCase.body))
		for key, value := range testCase.headers {
			request.Header.Set(key, value)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
		if response.StatusCode != testCase.want {
			t.Errorf("%s: status %d, want %d", testCase.name, response.StatusCode, testCase.want)
		}
	}
	if len(ts.poster.requests) != 0 {
		t.Error("rejected requests must not reach the poster")
	}
	if response := ts.get(t, ts.withToken("/api/post"), nil); response.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/post = %d, want 405", response.StatusCode)
	}
}

func TestPostWithoutPosterIsNotImplemented(t *testing.T) {
	server, err := New(Options{Context: loadFixture(t)})
	if err != nil {
		t.Fatal(err)
	}
	pageURL, err := server.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Shutdown(context.Background()) }()
	parsed, _ := url.Parse(pageURL)
	request, _ := http.NewRequest(http.MethodPost, "http://"+parsed.Host+"/api/post", strings.NewReader("{}"))
	request.Header.Set(tokenHeader, server.Token())
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNotImplemented {
		t.Errorf("status %d, want 501", response.StatusCode)
	}
}

func TestShutdownEndsOpenStreams(t *testing.T) {
	ts := startServer(t, loadFixture(t))
	response := ts.get(t, ts.withToken("/api/events"), nil)
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, response.Body)
		close(done)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := ts.server.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stream still open after shutdown")
	}
}
