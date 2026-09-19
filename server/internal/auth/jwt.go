package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"hookah/server/internal/domain"
)

// Claims of the access token, tech.md section 8.
type Claims struct {
	StaffID   string      `json:"sub"`
	VenueID   string      `json:"venue_id"`
	Role      domain.Role `json:"role"`
	SessionID string      `json:"session_id"`
}

type jwtClaims struct {
	VenueID   string      `json:"venue_id"`
	Role      domain.Role `json:"role"`
	SessionID string      `json:"session_id"`
	jwt.RegisteredClaims
}

type Tokens struct {
	secret    []byte
	accessTTL time.Duration
}

func NewTokens(secret string, accessTTL time.Duration) *Tokens {
	return &Tokens{secret: []byte(secret), accessTTL: accessTTL}
}

// Issue signs an access token valid from now for the configured TTL.
func (t *Tokens) Issue(c Claims, now time.Time) (string, time.Time, error) {
	exp := now.Add(t.accessTTL)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		VenueID:   c.VenueID,
		Role:      c.Role,
		SessionID: c.SessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   c.StaffID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	})
	s, err := tok.SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return s, exp, nil
}

// Parse verifies signature, algorithm and expiry at now. Any failure is
// domain.ErrUnauthorized.
func (t *Tokens) Parse(raw string, now time.Time) (Claims, error) {
	var c jwtClaims
	_, err := jwt.ParseWithClaims(raw, &c, func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(func() time.Time { return now }),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return Claims{}, fmt.Errorf("parse access token: %v: %w", err, domain.ErrUnauthorized)
	}
	if c.Subject == "" || c.VenueID == "" || c.SessionID == "" || !c.Role.Valid() {
		return Claims{}, fmt.Errorf("parse access token: %w", errors.Join(errors.New("missing claims"), domain.ErrUnauthorized))
	}
	return Claims{StaffID: c.Subject, VenueID: c.VenueID, Role: c.Role, SessionID: c.SessionID}, nil
}

// NewRefreshToken returns an opaque 32-byte token and its storage hash.
func NewRefreshToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("refresh token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashRefreshToken(token), nil
}

// HashRefreshToken is SHA-256: the token already has 256 bits of entropy, so
// a slow hash adds nothing and a fast one allows lookup by hash.
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
