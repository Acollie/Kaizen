package reviewui

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/alexcollie/kaizen/pkg/review"
)

const maxPostBodyBytes = 1 << 20

// handleEvents streams the event log as Server-Sent Events. It resumes after
// the Last-Event-ID header when present, otherwise after the since parameter.
func (server *Server) handleEvents(writer http.ResponseWriter, request *http.Request) {
	since := resumePoint(request)
	eventLog := server.options.Events

	// A finished log with nothing new: 204 tells EventSource to stop reconnecting
	if eventLog.Closed() && eventLog.LastSeq() <= since {
		writer.WriteHeader(http.StatusNoContent)
		return
	}

	controller := http.NewResponseController(writer)
	headers := writer.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")
	writer.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(writer, "retry: 3000\n\n")
	_ = controller.Flush()

	server.streamEvents(writer, request, controller, since)
}

// streamEvents writes events and heartbeats until the client leaves or the log is drained
func (server *Server) streamEvents(writer http.ResponseWriter, request *http.Request, controller *http.ResponseController, since int) {
	ctx := request.Context()
	events := server.options.Events.SubscribeSince(ctx, since)
	heartbeat := time.NewTicker(server.heartbeat)
	defer heartbeat.Stop()

	for {
		select {
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := writeSSEEvent(writer, event); err != nil {
				return
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(writer, ": heartbeat\n\n"); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
		if err := controller.Flush(); err != nil {
			return
		}
	}
}

// writeSSEEvent writes one event as `id:` and `data:` lines
func writeSSEEvent(writer http.ResponseWriter, event review.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "id: %d\ndata: %s\n\n", event.Seq, payload)
	return err
}

// resumePoint reads Last-Event-ID (preferred) or ?since=N
func resumePoint(request *http.Request) int {
	if lastID, ok := parseNonNegative(request.Header.Get("Last-Event-ID")); ok {
		return lastID
	}
	since, _ := parseNonNegative(request.URL.Query().Get("since"))
	return since
}

// parseNonNegative parses a non-negative integer
func parseNonNegative(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}

// handleFile returns a range of head-side lines for context expansion
func (server *Server) handleFile(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	relativePath := query.Get("path")
	if !server.allowedFiles[cleanRelativePath(relativePath)] {
		writeError(writer, http.StatusNotFound, "file is not part of this review")
		return
	}
	fullPath, err := resolveInsideWorktree(server.options.Context.WorktreePath, relativePath)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	lines, err := readFileLines(fullPath)
	if err != nil {
		writeError(writer, http.StatusNotFound, "file unavailable")
		return
	}
	from, _ := parseNonNegative(query.Get("from"))
	to, hasTo := parseNonNegative(query.Get("to"))
	if !hasTo {
		to = 0
	}
	writeJSON(writer, http.StatusOK, sliceLines(lines, from, to))
}

// handlePost publishes the selected findings through the configured Poster.
// It requires the token in the header (not the query) and a JSON body, so a
// cross-site form or simple request cannot trigger it.
func (server *Server) handlePost(writer http.ResponseWriter, request *http.Request) {
	if !server.tokenMatches(request.Header.Get(tokenHeader)) {
		writeError(writer, http.StatusForbidden, "invalid or missing session token")
		return
	}
	if origin := request.Header.Get("Origin"); origin != "" && !server.allowedHosts[originHost(origin)] {
		writeError(writer, http.StatusForbidden, "origin not allowed")
		return
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(writer, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
		return
	}
	if server.options.Poster == nil {
		writeError(writer, http.StatusNotImplemented, "posting is not configured for this session")
		return
	}

	var postRequest review.PostRequest
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, maxPostBodyBytes))
	if err := decoder.Decode(&postRequest); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON body")
		return
	}

	server.postMutex.Lock()
	defer server.postMutex.Unlock()
	if server.lastPost != nil && !postRequest.ConfirmRepost {
		writeJSON(writer, http.StatusConflict, alreadyPosted{
			Error:       "a review was already posted in this session; send confirm_repost to post another",
			PreviousURL: server.lastPost.URL,
		})
		return
	}

	reviewContext := server.options.Context
	result, err := server.options.Poster.PostReview(request.Context(), reviewContext.PR, reviewContext.Files, postRequest)
	if err != nil {
		writeError(writer, http.StatusBadGateway, err.Error())
		return
	}
	server.lastPost = result
	writeJSON(writer, http.StatusOK, result)
}

// alreadyPosted is the 409 body for a repost without confirmation
type alreadyPosted struct {
	Error       string `json:"error"`
	PreviousURL string `json:"previous_url"`
}

// originHost strips the scheme from an Origin header value
func originHost(origin string) string {
	for _, prefix := range []string{"http://", "https://"} {
		if len(origin) > len(prefix) && origin[:len(prefix)] == prefix {
			return origin[len(prefix):]
		}
	}
	return ""
}
