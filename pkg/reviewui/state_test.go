package reviewui

import (
	"context"
	"encoding/json"
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

// startServerWithOptions starts a server for the fixture with extra options applied
func startServerWithOptions(t *testing.T, configure func(*Options)) *testServer {
	t.Helper()
	events := NewEventLog()
	poster := &recordingPoster{}
	options := Options{Context: loadFixture(t), Events: events, Poster: poster}
	configure(&options)
	server, err := New(options)
	if err != nil {
		t.Fatal(err)
	}
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
	return &testServer{server: server, baseURL: "http://" + parsed.Host, events: events, poster: poster}
}

func stateServer(t *testing.T) (*testServer, string) {
	t.Helper()
	statePath := filepath.Join(t.TempDir(), "reviews", "pr-7-abc.state.json")
	return startServerWithOptions(t, func(options *Options) { options.StatePath = statePath }), statePath
}

func stateRequest(t *testing.T, ts *testServer, method, path, body string, headers map[string]string) (int, string) {
	t.Helper()
	request, _ := http.NewRequest(method, ts.baseURL+path, strings.NewReader(body))
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	content, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(content)
}

func goodStateHeaders(ts *testServer) map[string]string {
	return map[string]string{tokenHeader: ts.server.Token(), "Content-Type": "application/json"}
}

func TestStateRoundTrip(t *testing.T) {
	ts, statePath := stateServer(t)
	headers := goodStateHeaders(ts)

	if status, _ := stateRequest(t, ts, http.MethodGet, "/api/state", "", headers); status != http.StatusNoContent {
		t.Fatalf("GET before any save = %d, want 204", status)
	}
	document := `{"version":1,"triage":{"f1":{"state":"accepted","note":"checked"}}}`
	if status, body := stateRequest(t, ts, http.MethodPut, "/api/state", document, headers); status != http.StatusNoContent {
		t.Fatalf("PUT = %d %s", status, body)
	}
	status, body := stateRequest(t, ts, http.MethodGet, "/api/state", "", headers)
	if status != http.StatusOK || body != document {
		t.Fatalf("GET = %d %q, want the saved document", status, body)
	}
	onDisk, err := os.ReadFile(statePath)
	if err != nil || string(onDisk) != document {
		t.Fatalf("state file %q err %v", onDisk, err)
	}
}

func TestStateSurvivesANewServer(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	first := startServerWithOptions(t, func(options *Options) { options.StatePath = statePath })
	if status, _ := stateRequest(t, first, http.MethodPut, "/api/state", `{"a":1}`, goodStateHeaders(first)); status != http.StatusNoContent {
		t.Fatalf("PUT = %d", status)
	}
	second := startServerWithOptions(t, func(options *Options) { options.StatePath = statePath })
	if first.baseURL == second.baseURL {
		t.Fatal("expected a different port (origin)")
	}
	if status, body := stateRequest(t, second, http.MethodGet, "/api/state", "", goodStateHeaders(second)); status != http.StatusOK || body != `{"a":1}` {
		t.Fatalf("restarted GET = %d %q", status, body)
	}
}

func TestStateRequiresTokenOriginAndJSON(t *testing.T) {
	ts, statePath := stateServer(t)
	token := ts.server.Token()
	cases := []struct {
		name    string
		method  string
		path    string
		headers map[string]string
		want    int
	}{
		{"GET no token", http.MethodGet, "/api/state", nil, http.StatusForbidden},
		{"GET wrong token", http.MethodGet, "/api/state", map[string]string{tokenHeader: "nope"}, http.StatusForbidden},
		{"GET foreign origin", http.MethodGet, "/api/state", map[string]string{tokenHeader: token, "Origin": "http://evil.test"}, http.StatusForbidden},
		{"PUT no token", http.MethodPut, "/api/state", map[string]string{"Content-Type": "application/json"}, http.StatusForbidden},
		{"PUT query token only", http.MethodPut, "/api/state?t=" + token, map[string]string{"Content-Type": "application/json"}, http.StatusForbidden},
		{"PUT foreign origin", http.MethodPut, "/api/state", map[string]string{tokenHeader: token, "Content-Type": "application/json", "Origin": "http://evil.test"}, http.StatusForbidden},
		{"PUT form content type", http.MethodPut, "/api/state", map[string]string{tokenHeader: token, "Content-Type": "text/plain"}, http.StatusUnsupportedMediaType},
		{"PUT no content type", http.MethodPut, "/api/state", map[string]string{tokenHeader: token}, http.StatusUnsupportedMediaType},
		{"POST not allowed", http.MethodPost, "/api/state", map[string]string{tokenHeader: token, "Content-Type": "application/json"}, http.StatusMethodNotAllowed},
	}
	for _, testCase := range cases {
		if status, _ := stateRequest(t, ts, testCase.method, testCase.path, `{"x":1}`, testCase.headers); status != testCase.want {
			t.Errorf("%s: status %d, want %d", testCase.name, status, testCase.want)
		}
	}
	allowedOrigin := map[string]string{tokenHeader: token, "Content-Type": "application/json", "Origin": ts.baseURL}
	if status, _ := stateRequest(t, ts, http.MethodPut, "/api/state", `{"ok":true}`, allowedOrigin); status != http.StatusNoContent {
		t.Errorf("PUT from our own origin = %d, want 204", status)
	}
	if content, _ := os.ReadFile(statePath); string(content) != `{"ok":true}` {
		t.Errorf("only the allowed request may write; file = %q", content)
	}
}

func TestStateRejectsBadBodies(t *testing.T) {
	ts, statePath := stateServer(t)
	headers := goodStateHeaders(ts)
	for _, body := range []string{"", "[1,2]", `"text"`, "null", "{not json", `{"a":1} trailing`} {
		if status, _ := stateRequest(t, ts, http.MethodPut, "/api/state", body, headers); status != http.StatusBadRequest {
			t.Errorf("body %q: status %d, want 400", body, status)
		}
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Error("rejected bodies must not create the state file")
	}
}

func TestStateSizeCap(t *testing.T) {
	ts, statePath := stateServer(t)
	headers := goodStateHeaders(ts)
	padding := strings.Repeat("x", maxStateBodyBytes)
	if status, _ := stateRequest(t, ts, http.MethodPut, "/api/state", `{"pad":"`+padding+`"}`, headers); status != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized PUT = %d, want 413", status)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Error("an oversized body must not be written")
	}
	justUnder := `{"pad":"` + strings.Repeat("x", maxStateBodyBytes-20) + `"}`
	if status, _ := stateRequest(t, ts, http.MethodPut, "/api/state", justUnder, headers); status != http.StatusNoContent {
		t.Errorf("PUT under the cap = %d, want 204", status)
	}
}

func TestStateWithoutPathIsDisabled(t *testing.T) {
	ts := startServer(t, loadFixture(t))
	headers := goodStateHeaders(ts)
	if status, _ := stateRequest(t, ts, http.MethodGet, "/api/state", "", headers); status != http.StatusNotFound {
		t.Errorf("GET = %d, want 404", status)
	}
	if status, _ := stateRequest(t, ts, http.MethodPut, "/api/state", "{}", headers); status != http.StatusNotImplemented {
		t.Errorf("PUT = %d, want 501", status)
	}
}

func TestWriteFileAtomicReplacesWithoutLeavingTempFiles(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "nested", "state.json")
	if err := writeFileAtomic(path, []byte(`{"v":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(path, []byte(`{"v":2}`)); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(path)
	if string(content) != `{"v":2}` {
		t.Errorf("content %q", content)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Errorf("temp files left behind: %v", entries)
	}
}

func TestWriteFileAtomicKeepsOldFileOnFailure(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "state.json")
	if err := writeFileAtomic(path, []byte(`{"v":1}`)); err != nil {
		t.Fatal(err)
	}
	// The target is a non-empty directory, so the rename fails after the temp file was
	// fully written: the failure must not touch other state or leave the temp behind
	blocked := filepath.Join(directory, "blocked")
	if err := os.MkdirAll(filepath.Join(blocked, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(blocked, []byte(`{"v":2}`)); err == nil {
		t.Fatal("expected the rename over a non-empty directory to fail")
	}
	if content, _ := os.ReadFile(path); string(content) != `{"v":1}` {
		t.Errorf("existing state changed: %q", content)
	}
	entries, _ := os.ReadDir(directory)
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Errorf("temp file left behind after a failed write: %s", entry.Name())
		}
	}
}

func TestStatePathFor(t *testing.T) {
	cache := filepath.Join("repo", ".kaizen", "reviews", "pr-5-0123456789ab.json")
	want := filepath.Join("repo", ".kaizen", "reviews", "pr-5-0123456789ab.state.json")
	if got := StatePathFor(cache); got != want {
		t.Errorf("StatePathFor = %s, want %s", got, want)
	}
}

// slowPoster blocks until its context ends, like a hung `gh api`
type slowPoster struct{ sawDeadline bool }

func (poster *slowPoster) PostReview(ctx context.Context, pr review.PullRequest, files []review.FileDiff, request review.PostRequest) (*review.PostResult, error) {
	_, poster.sawDeadline = ctx.Deadline()
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestPostTimesOutAndReleasesTheLock(t *testing.T) {
	poster := &slowPoster{}
	ts := startServerWithOptions(t, func(options *Options) {
		options.Poster = poster
		options.PostTimeout = 50 * time.Millisecond
	})
	headers := goodStateHeaders(ts)
	for attempt := 0; attempt < 2; attempt++ {
		started := time.Now()
		// The first timeout arms the repost guard, so the retry must confirm
		status, body := stateRequest(t, ts, http.MethodPost, "/api/post", `{"summary":"s","confirm_repost":true}`, headers)
		if status != http.StatusGatewayTimeout {
			t.Fatalf("attempt %d: status %d, want 504", attempt, status)
		}
		var payload map[string]string
		_ = json.Unmarshal([]byte(body), &payload)
		if !strings.Contains(payload["error"], "timed out") {
			t.Errorf("error %q should say it timed out", payload["error"])
		}
		if elapsed := time.Since(started); elapsed > 2*time.Second {
			t.Errorf("attempt %d took %s", attempt, elapsed)
		}
	}
	if !poster.sawDeadline {
		t.Error("the poster's context must carry a deadline")
	}
}

func TestPostTimeoutDefaultsToTwoMinutes(t *testing.T) {
	server, err := New(Options{Context: loadFixture(t)})
	if err != nil {
		t.Fatal(err)
	}
	if server.options.PostTimeout != 2*time.Minute {
		t.Errorf("PostTimeout = %s", server.options.PostTimeout)
	}
}

// flakyPoster times out (blocks past the deadline) until hang is cleared, then succeeds
type flakyPoster struct {
	hang  bool
	calls int
}

func (poster *flakyPoster) PostReview(ctx context.Context, pr review.PullRequest, files []review.FileDiff, request review.PostRequest) (*review.PostResult, error) {
	poster.calls++
	if poster.hang {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &review.PostResult{URL: "https://example.test/review/2"}, nil
}

func TestTimedOutPostArmsTheRepostGuard(t *testing.T) {
	poster := &flakyPoster{hang: true}
	ts := startServerWithOptions(t, func(options *Options) {
		options.Poster = poster
		options.PostTimeout = 50 * time.Millisecond
	})
	headers := goodStateHeaders(ts)
	if status, _ := stateRequest(t, ts, http.MethodPost, "/api/post", `{"summary":"s"}`, headers); status != http.StatusGatewayTimeout {
		t.Fatalf("first post: status %d, want 504", status)
	}

	poster.hang = false
	status, body := stateRequest(t, ts, http.MethodPost, "/api/post", `{"summary":"s"}`, headers)
	if status != http.StatusConflict {
		t.Fatalf("unconfirmed retry after a timeout: status %d, want 409", status)
	}
	var conflict alreadyPosted
	if err := json.Unmarshal([]byte(body), &conflict); err != nil {
		t.Fatal(err)
	}
	if !conflict.OutcomeUnknown || conflict.PreviousURL != "" || !strings.Contains(conflict.Error, "may have reached GitHub") {
		t.Errorf("conflict body %+v", conflict)
	}
	if poster.calls != 1 {
		t.Fatalf("the refused retry must not reach the poster, got %d calls", poster.calls)
	}

	if status, _ := stateRequest(t, ts, http.MethodPost, "/api/post", `{"summary":"s","confirm_repost":true}`, headers); status != http.StatusOK {
		t.Fatalf("confirmed retry: status %d, want 200", status)
	}
	status, body = stateRequest(t, ts, http.MethodPost, "/api/post", `{"summary":"s"}`, headers)
	if status != http.StatusConflict {
		t.Fatalf("post after a success: status %d, want 409", status)
	}
	var afterSuccess alreadyPosted
	_ = json.Unmarshal([]byte(body), &afterSuccess)
	if afterSuccess.OutcomeUnknown || afterSuccess.PreviousURL != "https://example.test/review/2" {
		t.Errorf("after a success the guard names the posted review, got %+v", afterSuccess)
	}
}
