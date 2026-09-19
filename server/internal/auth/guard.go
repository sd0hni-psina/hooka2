package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"hookah/server/internal/domain"
)

// SessionChecker confirms that the session is live and its staff member is
// not blocked. It returns domain.ErrUnauthorized or domain.ErrForbidden.
type SessionChecker interface {
	CheckSession(ctx context.Context, c Claims) error
}

type Guard struct {
	tokens  *Tokens
	checker SessionChecker
	now     func() time.Time
	onError func(w http.ResponseWriter, r *http.Request, err error)
}

func NewGuard(tokens *Tokens, checker SessionChecker, onError func(http.ResponseWriter, *http.Request, error)) *Guard {
	return &Guard{tokens: tokens, checker: checker, now: time.Now, onError: onError}
}

// Authenticate resolves claims from a raw access token and checks the session.
func (g *Guard) Authenticate(ctx context.Context, raw string) (Claims, error) {
	if raw == "" {
		return Claims{}, fmt.Errorf("no access token: %w", domain.ErrUnauthorized)
	}
	c, err := g.tokens.Parse(raw, g.now())
	if err != nil {
		return Claims{}, err
	}
	if err := g.checker.CheckSession(ctx, c); err != nil {
		return Claims{}, err
	}
	return c, nil
}

func (g *Guard) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !found {
			raw = ""
		}
		c, err := g.Authenticate(r.Context(), strings.TrimSpace(raw))
		if err != nil {
			g.onError(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithClaims(r.Context(), c)))
	})
}

// RequireManager allows owner and admin only.
func RequireManager(c Claims) error {
	if !c.Role.CanManage() {
		return domain.ErrForbidden
	}
	return nil
}
