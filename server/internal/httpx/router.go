package httpx

import (
	"net/http"

	"hookah/server/internal/domain"
)

// HandlerFunc returns an error instead of writing it, so every handler maps
// errors through WriteError in one place.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

const APIPrefix = "/api/v1"

type Router struct {
	mux   *http.ServeMux
	guard func(http.Handler) http.Handler
}

// NewRouter takes the auth guard applied to private routes.
func NewRouter(guard func(http.Handler) http.Handler) *Router {
	r := &Router{mux: http.NewServeMux(), guard: guard}
	r.mux.Handle(APIPrefix+"/", wrap(func(http.ResponseWriter, *http.Request) error { return domain.ErrNotFound }))
	return r
}

func wrap(h HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
		if err := h(w, r); err != nil {
			WriteError(w, r, err)
		}
	})
}

func split(pattern string) (string, string) {
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == ' ' {
			return pattern[:i], pattern[i+1:]
		}
	}
	return "", pattern
}

func (r *Router) route(pattern string) string {
	method, path := split(pattern)
	return method + " " + APIPrefix + path
}

// Public registers an API route without auth, e.g. "POST /join".
func (r *Router) Public(pattern string, h HandlerFunc) {
	r.mux.Handle(r.route(pattern), wrap(h))
}

// Private registers an API route behind the auth guard.
func (r *Router) Private(pattern string, h HandlerFunc) {
	r.mux.Handle(r.route(pattern), r.guard(wrap(h)))
}

// Raw registers a handler outside the API prefix, e.g. "GET /healthz".
func (r *Router) Raw(pattern string, h http.Handler) {
	r.mux.Handle(pattern, h)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}
