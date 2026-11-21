package reviewui

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/alexcollie/kaizen/pkg/review"
)

//go:embed static
var staticFiles embed.FS

const (
	defaultAddr        = "127.0.0.1:0"
	tokenHeader        = "X-Kaizen-Token"
	tokenQueryParam    = "t"
	defaultHeartbeat   = 15 * time.Second
	defaultPostTimeout = 2 * time.Minute
	readHeaderTimeout  = 10 * time.Second

	contentSecurityPolicy = "default-src 'self'; " +
		"script-src 'self'; " +
		"style-src 'self'; img-src 'self' data:; connect-src 'self'; font-src 'self'; " +
		"object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'"
)

// Options configures a review UI server
type Options struct {
	Context *review.ReviewContext // required
	Events  *EventLog             // optional; an empty log is created when nil
	Poster  review.Poster         // optional; POST /api/post returns 501 when nil
	Addr    string                // listen address, default 127.0.0.1:0
	// StatePath is the JSON file GET/PUT /api/state read and write: the reviewer's
	// triage, notes, comments and viewed state. Optional; empty disables the endpoints.
	StatePath string
	// PostTimeout bounds one POST /api/post call to the Poster; default 2 minutes
	PostTimeout time.Duration
}

// Server serves the review UI and its JSON/SSE API on a loopback address
type Server struct {
	options      Options
	token        string
	allowedFiles map[string]bool
	heartbeat    time.Duration

	listener     net.Listener
	httpServer   *http.Server
	allowedHosts map[string]bool
	cancelBase   context.CancelFunc

	// postMutex serialises posts so lastPost is checked and set atomically: two
	// clicks (or tabs) can never both post a review
	postMutex sync.Mutex
	lastPost  *review.PostResult // the session's successful post, nil until one
	// postOutcomeUnknown is set when a post timed out: GitHub may hold the review even
	// though no result came back, so the next attempt needs confirm_repost too
	postOutcomeUnknown bool

	stateMutex sync.Mutex // serialises state file reads and writes
}

// New validates the options and prepares a server; call Start to listen
func New(options Options) (*Server, error) {
	if options.Context == nil {
		return nil, errors.New("reviewui: Options.Context is required")
	}
	if options.Events == nil {
		options.Events = NewEventLog()
	}
	if options.Addr == "" {
		options.Addr = defaultAddr
	}
	if options.PostTimeout <= 0 {
		options.PostTimeout = defaultPostTimeout
	}
	token, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	return &Server{
		options:      options,
		token:        token,
		allowedFiles: allowedFilePaths(options.Context),
		heartbeat:    defaultHeartbeat,
	}, nil
}

// Token returns the per-session secret that every API call must carry
func (server *Server) Token() string {
	return server.token
}

// Start listens on the configured address and serves in the background. It
// returns the browser URL, which carries the session token.
func (server *Server) Start() (string, error) {
	listener, err := net.Listen("tcp", server.options.Addr)
	if err != nil {
		return "", fmt.Errorf("reviewui: listen on %s: %w", server.options.Addr, err)
	}
	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()
		return "", errors.New("reviewui: listener is not TCP")
	}

	urlHost := loopbackURLHost(tcpAddr)
	server.allowedHosts = map[string]bool{
		urlHost: true,
		fmt.Sprintf("localhost:%d", tcpAddr.Port): true,
	}

	baseContext, cancel := context.WithCancel(context.Background())
	server.cancelBase = cancel
	server.listener = listener
	server.httpServer = &http.Server{
		Handler:           server.Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
		BaseContext:       func(net.Listener) context.Context { return baseContext },
	}
	go func() { _ = server.httpServer.Serve(listener) }()

	return fmt.Sprintf("http://%s/?%s=%s", urlHost, tokenQueryParam, server.token), nil
}

// Shutdown ends open event streams and stops the server
func (server *Server) Shutdown(ctx context.Context) error {
	if server.httpServer == nil {
		return nil
	}
	server.cancelBase()
	return server.httpServer.Shutdown(ctx)
}

// Handler returns the full route tree with host checking and security headers
func (server *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", server.handleIndex)
	mux.Handle("GET /static/", server.staticHandler())
	mux.Handle("GET /api/context", server.requireToken(http.HandlerFunc(server.handleContext)))
	mux.Handle("GET /api/events", server.requireToken(http.HandlerFunc(server.handleEvents)))
	mux.Handle("GET /api/file", server.requireToken(http.HandlerFunc(server.handleFile)))
	mux.HandleFunc("POST /api/post", server.handlePost)
	mux.HandleFunc("GET /api/state", server.handleGetState)
	mux.HandleFunc("PUT /api/state", server.handlePutState)
	return server.checkHost(securityHeaders(mux))
}

// handleIndex serves the single-page app shell
func (server *Server) handleIndex(writer http.ResponseWriter, request *http.Request) {
	page, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		http.Error(writer, "index missing", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	_, _ = writer.Write(page)
}

// staticHandler serves embedded assets under /static/ without directory listings
func (server *Server) staticHandler() http.Handler {
	fileServer := http.FileServerFS(staticFiles)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, "/") {
			http.NotFound(writer, request)
			return
		}
		if _, err := fs.Stat(staticFiles, strings.TrimPrefix(request.URL.Path, "/")); err != nil {
			http.NotFound(writer, request)
			return
		}
		writer.Header().Set("Cache-Control", "no-cache")
		fileServer.ServeHTTP(writer, request)
	})
}

// handleContext returns the review context as JSON
func (server *Server) handleContext(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, server.options.Context)
}

// checkHost rejects requests whose Host header is not our loopback address (DNS rebinding)
func (server *Server) checkHost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !server.allowedHosts[strings.ToLower(request.Host)] {
			writeError(writer, http.StatusForbidden, "host not allowed")
			return
		}
		next.ServeHTTP(writer, request)
	})
}

// requireToken accepts the session token as the t query parameter or the X-Kaizen-Token header
func (server *Server) requireToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		supplied := request.Header.Get(tokenHeader)
		if supplied == "" {
			supplied = request.URL.Query().Get(tokenQueryParam)
		}
		if !server.tokenMatches(supplied) {
			writeError(writer, http.StatusForbidden, "invalid or missing session token")
			return
		}
		next.ServeHTTP(writer, request)
	})
}

// tokenMatches compares in constant time
func (server *Server) tokenMatches(supplied string) bool {
	return supplied != "" && subtle.ConstantTimeCompare([]byte(supplied), []byte(server.token)) == 1
}

// securityHeaders adds CSP and related headers to every response
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		headers := writer.Header()
		headers.Set("Content-Security-Policy", contentSecurityPolicy)
		headers.Set("X-Content-Type-Options", "nosniff")
		headers.Set("Referrer-Policy", "no-referrer")
		headers.Set("X-Frame-Options", "DENY")
		headers.Set("Cross-Origin-Opener-Policy", "same-origin")
		if strings.HasPrefix(request.URL.Path, "/api/") {
			headers.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(writer, request)
	})
}

// loopbackURLHost picks the host:port the browser should use for the listener
func loopbackURLHost(addr *net.TCPAddr) string {
	if addr.IP == nil || addr.IP.IsUnspecified() || addr.IP.To4() != nil {
		return fmt.Sprintf("127.0.0.1:%d", addr.Port)
	}
	return net.JoinHostPort(addr.IP.String(), fmt.Sprint(addr.Port))
}

// newSessionToken returns 32 random bytes, hex encoded
func newSessionToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("reviewui: generate token: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

// writeJSON encodes value with the given status
func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

// writeError sends {"error": message}
func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]string{"error": message})
}
