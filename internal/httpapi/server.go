package httpapi

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/0adnyana/go-clipboard/internal/clips"
)

type Server struct {
	logger *slog.Logger
	mux    *http.ServeMux
}

// apiRoute is one registered endpoint. Registration and the fallback's notion of
// which paths exist are derived from the same list, so a route cannot be added
// without the fallback learning that its path is real.
type apiRoute struct {
	method  string
	path    string
	handler http.HandlerFunc
}

func NewServer(logger *slog.Logger, deps Dependencies) *Server {
	mux := http.NewServeMux()
	s := &Server{
		logger: logger,
		mux:    mux,
	}

	clipSvc := deps.ClipService
	if clipSvc == nil && deps.Queries != nil {
		clipSvc = clips.NewService(clips.NewPGStore(deps.Queries))
	}

	routes := []apiRoute{
		{http.MethodGet, "/api/health", handleHealth(deps)},
	}
	if clipSvc != nil {
		routes = append(routes,
			apiRoute{http.MethodPost, "/api/clips", handleCreateClip(logger, clipSvc)},
			apiRoute{http.MethodGet, "/api/clips/{slug}", handleReadClip(logger, clipSvc)},
		)
	}

	allowed := make(map[string][]string, len(routes))
	for _, route := range routes {
		mux.HandleFunc(route.method+" "+route.path, route.handler)
		allowed[route.path] = append(allowed[route.path], route.method)
		if route.method == http.MethodGet {
			// A GET pattern also serves HEAD, so the Allow header must say so.
			allowed[route.path] = append(allowed[route.path], http.MethodHead)
		}
	}

	mux.HandleFunc("/api/", handleAPIFallback(allowed))
	mux.HandleFunc("/", handleRootFallback)

	return s
}

// handleAPIFallback answers every /api/ request that no method-qualified pattern
// matched. It resolves the path before the method: a path nobody registered is a
// 404 whatever the method, and 405 is reserved for a path that exists but does
// not answer this method.
func handleAPIFallback(allowed map[string][]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		methods, registered := allowed[r.URL.Path]
		if !registered {
			handleNotFound(w, r)
			return
		}
		w.Header().Set("Allow", strings.Join(methods, ", "))
		handleMethodNotAllowed(w, r)
	}
}

// handleRootFallback exists only so paths outside /api/ are refused in this
// server's JSON error format instead of ServeMux's plain-text 404. It serves no
// content: static assets and SPA fallback belong to the proxy.
func handleRootFallback(w http.ResponseWriter, r *http.Request) {
	handleNotFound(w, r)
}

func (s *Server) Handler() http.Handler {
	return Chain(s.mux,
		RequestLogging(s.logger),
		PanicRecovery(s.logger),
	)
}
