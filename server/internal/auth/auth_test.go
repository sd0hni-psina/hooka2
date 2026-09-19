package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"hookah/server/internal/domain"
)

var now = time.Date(2026, 9, 19, 18, 0, 0, 0, time.UTC)

func sample() Claims {
	return Claims{StaffID: "staff-1", VenueID: "venue-1", Role: domain.RoleMaster, SessionID: "session-1"}
}

func TestAccessToken(t *testing.T) {
	tokens := NewTokens("secret", 30*time.Minute)

	t.Run("issued token parses back to the same claims", func(t *testing.T) {
		raw, exp, err := tokens.Issue(sample(), now)
		require.NoError(t, err)
		require.Equal(t, now.Add(30*time.Minute), exp)
		got, err := tokens.Parse(raw, now.Add(29*time.Minute))
		require.NoError(t, err)
		require.Equal(t, sample(), got)
	})

	t.Run("expired token is unauthorized", func(t *testing.T) {
		raw, _, err := tokens.Issue(sample(), now)
		require.NoError(t, err)
		_, err = tokens.Parse(raw, now.Add(31*time.Minute))
		require.ErrorIs(t, err, domain.ErrUnauthorized)
	})

	t.Run("token signed with another secret is unauthorized", func(t *testing.T) {
		raw, _, err := NewTokens("other", time.Minute).Issue(sample(), now)
		require.NoError(t, err)
		_, err = tokens.Parse(raw, now)
		require.ErrorIs(t, err, domain.ErrUnauthorized)
	})

	t.Run("none algorithm is unauthorized", func(t *testing.T) {
		raw, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
			"sub": "x", "venue_id": "v", "role": "owner", "session_id": "s", "exp": now.Add(time.Hour).Unix(),
		}).SignedString(jwt.UnsafeAllowNoneSignatureType)
		require.NoError(t, err)
		_, err = tokens.Parse(raw, now)
		require.ErrorIs(t, err, domain.ErrUnauthorized)
	})

	t.Run("unknown role is unauthorized", func(t *testing.T) {
		c := sample()
		c.Role = "root"
		raw, _, err := tokens.Issue(c, now)
		require.NoError(t, err)
		_, err = tokens.Parse(raw, now)
		require.ErrorIs(t, err, domain.ErrUnauthorized)
	})

	t.Run("garbage is unauthorized", func(t *testing.T) {
		_, err := tokens.Parse("not.a.jwt", now)
		require.ErrorIs(t, err, domain.ErrUnauthorized)
	})
}

func TestPassword(t *testing.T) {
	hash, err := HashPassword("correct horse")
	require.NoError(t, err)
	require.NotContains(t, hash, "correct horse")

	ok, err := CheckPassword(hash, "correct horse")
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = CheckPassword(hash, "wrong horse")
	require.NoError(t, err)
	require.False(t, ok)

	_, err = CheckPassword("not-a-hash", "x")
	require.Error(t, err)
}

func TestRefreshToken(t *testing.T) {
	a, hashA, err := NewRefreshToken()
	require.NoError(t, err)
	b, _, err := NewRefreshToken()
	require.NoError(t, err)
	require.NotEqual(t, a, b)
	require.Equal(t, hashA, HashRefreshToken(a))
	require.NotContains(t, hashA, a)
}

type checkerFunc func(context.Context, Claims) error

func (f checkerFunc) CheckSession(ctx context.Context, c Claims) error { return f(ctx, c) }

func TestGuard(t *testing.T) {
	tokens := NewTokens("secret", 30*time.Minute)
	valid, _, err := tokens.Issue(sample(), time.Now())
	require.NoError(t, err)

	run := func(header string, checkErr error) (int, *Claims) {
		var seen *Claims
		g := NewGuard(tokens, checkerFunc(func(context.Context, Claims) error { return checkErr }),
			func(w http.ResponseWriter, _ *http.Request, err error) {
				if strings.Contains(err.Error(), "forbidden") {
					w.WriteHeader(http.StatusForbidden)
					return
				}
				w.WriteHeader(http.StatusUnauthorized)
			})
		h := g.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c, _ := FromContext(r.Context())
			seen = &c
		}))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code, seen
	}

	t.Run("valid bearer token reaches the handler with claims", func(t *testing.T) {
		code, c := run("Bearer "+valid, nil)
		require.Equal(t, http.StatusOK, code)
		require.Equal(t, sample(), *c)
	})
	t.Run("missing header is unauthorized", func(t *testing.T) {
		code, c := run("", nil)
		require.Equal(t, http.StatusUnauthorized, code)
		require.Nil(t, c)
	})
	t.Run("non bearer scheme is unauthorized", func(t *testing.T) {
		code, _ := run("Basic "+valid, nil)
		require.Equal(t, http.StatusUnauthorized, code)
	})
	t.Run("blocked staff is forbidden", func(t *testing.T) {
		code, c := run("Bearer "+valid, domain.ErrForbidden)
		require.Equal(t, http.StatusForbidden, code)
		require.Nil(t, c)
	})
}
