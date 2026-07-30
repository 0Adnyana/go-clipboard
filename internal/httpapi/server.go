package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/0adnyana/go-clipboard/internal/clips"
)

type Server struct {
	logger *slog.Logger
	mux    *http.ServeMux
	routes []apiRoute
}

// apiRoute is one registered endpoint. Both the method-qualified patterns and
// the path-only patterns that answer 405 are derived from the same list, so a
// route cannot be added without its path being recognised as real.
type apiRoute struct {
	method  string
	path    string
	handler http.HandlerFunc
}

// Dependencies carries what the server cannot serve without. A zero required
// field here is a wiring bug, not a runtime condition.
type Dependencies struct {
	Clips  *clips.Service
	Health HealthDependencies
}

func (d Dependencies) validate() error {
	var missing []string
	if d.Clips == nil {
		missing = append(missing, "Clips")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("missing required dependencies: %s", strings.Join(missing, ", "))
}

func NewServer(logger *slog.Logger, deps Dependencies) (*Server, error) {
	if err := deps.validate(); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	routes := healthRoutes(deps.Health)
	routes = append(routes, clipRoutes(logger, deps.Clips, deps.Health)...)

	allowed := make(map[string][]string, len(routes))
	for _, route := range routes {
		mux.HandleFunc(route.method+" "+route.path, route.handler)
		allowed[route.path] = append(allowed[route.path], route.method)
		if route.method == http.MethodGet {
			// A GET pattern also serves HEAD, so the Allow header must say so.
			allowed[route.path] = append(allowed[route.path], http.MethodHead)
		}
	}

	// Every registered path is registered a second time without a method. Such a
	// pattern matches a superset of its method-qualified siblings, so ServeMux
	// prefers those and this one is reached exactly when the path exists but the
	// method is unserved. Deciding whether a request matches a wildcard path such
	// as /api/clips/{slug} is left to the mux, which is the only thing that can
	// compare a request against a pattern.
	for path, methods := range allowed {
		mux.HandleFunc(path, handleMethodNotAllowed(strings.Join(methods, ", ")))
	}

	mux.HandleFunc("/api/", handleAPIFallback)
	mux.HandleFunc("/", handleRootFallback)

	return &Server{
		logger: logger,
		mux:    mux,
		routes: routes,
	}, nil
}

// handleAPIFallback answers every /api/ request that neither a method-qualified
// pattern nor a path-only pattern matched. Reaching it means no registered path
// matched, so the method is irrelevant: this is a 404 whatever the caller tried
// to do, and 405 is reserved for the path-only patterns.
func handleAPIFallback(w http.ResponseWriter, r *http.Request) {
	handleNotFound(w, r)
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
