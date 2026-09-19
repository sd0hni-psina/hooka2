package staff

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hookah/server/internal/auth"
	"hookah/server/internal/domain"
	"hookah/server/internal/layout"
	"hookah/server/internal/store"
	"hookah/server/internal/venue"
)

type Me struct {
	Staff domain.Staff  `json:"staff"`
	Venue domain.Venue  `json:"venue"`
	Halls []domain.Hall `json:"halls"`
}

type Service struct {
	pool       *pgxpool.Pool
	repo       *Repository
	venues     *venue.Repository
	layout     *layout.Repository
	tokens     *auth.Tokens
	refreshTTL time.Duration
	now        func() time.Time
}

func NewService(pool *pgxpool.Pool, repo *Repository, venues *venue.Repository, layout *layout.Repository,
	tokens *auth.Tokens, refreshTTL time.Duration) *Service {
	return &Service{pool: pool, repo: repo, venues: venues, layout: layout, tokens: tokens, refreshTTL: refreshTTL, now: time.Now}
}

// Me returns the bootstrap payload of the signed-in device.
func (s *Service) Me(ctx context.Context, c auth.Claims) (Me, error) {
	st, err := s.repo.Get(ctx, s.pool, c.VenueID, c.StaffID)
	if err != nil {
		return Me{}, fmt.Errorf("me: %w", err)
	}
	v, err := s.venues.Get(ctx, s.pool, c.VenueID)
	if err != nil {
		return Me{}, fmt.Errorf("me: %w", err)
	}
	halls, err := s.layout.ListHalls(ctx, s.pool, c.VenueID)
	if err != nil {
		return Me{}, fmt.Errorf("me: %w", err)
	}
	if halls == nil {
		halls = []domain.Hall{}
	}
	// Role comes from the row, not the token, so a demotion applies before the token expires.
	return Me{Staff: st, Venue: v.ForRole(st.Role), Halls: halls}, nil
}

// CheckSession implements auth.SessionChecker.
func (s *Service) CheckSession(ctx context.Context, c auth.Claims) error {
	found, revoked, blocked, err := s.repo.SessionState(ctx, s.pool, c.SessionID, c.StaffID, c.VenueID)
	if err != nil {
		return err
	}
	if !found || revoked {
		return fmt.Errorf("session %s not live: %w", c.SessionID, domain.ErrUnauthorized)
	}
	if blocked {
		return fmt.Errorf("staff %s blocked: %w", c.StaffID, domain.ErrForbidden)
	}
	return nil
}

// IssueTokens opens or revives the device session of st and signs a token
// pair. It runs inside the caller's transaction.
func (s *Service) IssueTokens(ctx context.Context, q store.Querier, st domain.Staff, d domain.Device, now time.Time) (domain.Tokens, error) {
	refresh, hash, err := auth.NewRefreshToken()
	if err != nil {
		return domain.Tokens{}, err
	}
	sessionID, err := s.repo.UpsertSession(ctx, q, st.ID, d, hash, now)
	if err != nil {
		return domain.Tokens{}, err
	}
	return s.sign(st, sessionID, refresh, now)
}

func (s *Service) sign(st domain.Staff, sessionID, refresh string, now time.Time) (domain.Tokens, error) {
	access, exp, err := s.tokens.Issue(auth.Claims{StaffID: st.ID, VenueID: st.VenueID, Role: st.Role, SessionID: sessionID}, now)
	if err != nil {
		return domain.Tokens{}, err
	}
	return domain.Tokens{AccessToken: access, RefreshToken: refresh, AccessExpiresAt: exp.UTC()}, nil
}

type JoinResult struct {
	Tokens domain.Tokens `json:"tokens"`
	Venue  domain.Venue  `json:"venue"`
	Halls  []domain.Hall `json:"halls"`
}

// Join signs a master in by venue code, tech.md sections 8, 19.1 and 19.4.
// A wrong code is recorded before the error returns, so the attempt counts
// toward the rate limit.
func (s *Service) Join(ctx context.Context, in domain.JoinInput) (JoinResult, error) {
	in, err := domain.ValidateJoin(in)
	if err != nil {
		return JoinResult{}, err
	}
	now := domain.DBTime(s.now())

	var res JoinResult
	var outcome error
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := s.repo.LockDevice(ctx, tx, in.DeviceID); err != nil {
			return err
		}
		attempts, err := s.repo.RecentAttempts(ctx, tx, in.DeviceID, now.Add(-time.Hour))
		if err != nil {
			return err
		}
		if retry, limited := domain.JoinRetryAfter(attempts, now); limited {
			outcome = &domain.RateLimitError{RetryAfterSec: retry}
			return nil
		}

		v, found, err := s.venues.GetByCode(ctx, tx, in.Code)
		if err != nil {
			return err
		}
		if err := s.repo.RecordAttempt(ctx, tx, in.DeviceID, in.Code, found, now); err != nil {
			return err
		}
		if !found {
			outcome = fmt.Errorf("join %s: %w", in.Code, domain.ErrInvalidJoinCode)
			return nil
		}

		st, err := s.findOrCreateMaster(ctx, tx, v.ID, in.DisplayName)
		if err != nil {
			return err
		}
		if st.Blocked {
			outcome = fmt.Errorf("join as blocked %s: %w", st.ID, domain.ErrForbidden)
			return nil
		}
		tokens, err := s.IssueTokens(ctx, tx, st, in.Device, now)
		if err != nil {
			return err
		}
		halls, err := s.layout.ListHalls(ctx, tx, v.ID)
		if err != nil {
			return err
		}
		if halls == nil {
			halls = []domain.Hall{}
		}
		res = JoinResult{Tokens: tokens, Venue: v.ForRole(st.Role), Halls: halls}
		return nil
	})
	if err != nil {
		return JoinResult{}, fmt.Errorf("join: %w", err)
	}
	if outcome != nil {
		return JoinResult{}, outcome
	}
	return res, nil
}

func (s *Service) findOrCreateMaster(ctx context.Context, q store.Querier, venueID, name string) (domain.Staff, error) {
	st, found, err := s.repo.FindMaster(ctx, q, venueID, name)
	if err != nil || found {
		return st, err
	}
	st, created, err := s.repo.CreateMaster(ctx, q, venueID, name)
	if err != nil || created {
		return st, err
	}
	// A concurrent join created the same name between the two statements.
	st, found, err = s.repo.FindMaster(ctx, q, venueID, name)
	if err != nil {
		return domain.Staff{}, err
	}
	if !found {
		return domain.Staff{}, errors.New("master vanished after conflict")
	}
	return st, nil
}

// Refresh rotates the refresh token of a live session, tech.md section 19.4.
func (s *Service) Refresh(ctx context.Context, refresh string) (domain.Tokens, error) {
	if refresh == "" {
		v := &domain.ValidationError{}
		v.Add("refresh_token", domain.ReasonRequired)
		return domain.Tokens{}, v
	}
	now := domain.DBTime(s.now())
	oldHash := auth.HashRefreshToken(refresh)
	sess, found, err := s.repo.SessionByRefresh(ctx, s.pool, oldHash)
	if err != nil {
		return domain.Tokens{}, fmt.Errorf("refresh: %w", err)
	}
	if !found || sess.Revoked || now.Sub(sess.IssuedAt) > s.refreshTTL {
		return domain.Tokens{}, fmt.Errorf("refresh: stale token: %w", domain.ErrUnauthorized)
	}
	if sess.Staff.Blocked {
		return domain.Tokens{}, fmt.Errorf("refresh: blocked staff: %w", domain.ErrForbidden)
	}
	next, newHash, err := auth.NewRefreshToken()
	if err != nil {
		return domain.Tokens{}, err
	}
	ok, err := s.repo.RotateRefresh(ctx, s.pool, sess.SessionID, oldHash, newHash, now)
	if err != nil {
		return domain.Tokens{}, fmt.Errorf("refresh: %w", err)
	}
	if !ok {
		return domain.Tokens{}, fmt.Errorf("refresh: lost rotation race: %w", domain.ErrUnauthorized)
	}
	return s.sign(sess.Staff, sess.SessionID, next, now)
}

func (s *Service) Logout(ctx context.Context, c auth.Claims) error {
	return s.repo.RevokeSession(ctx, s.pool, c.SessionID)
}

func (s *Service) SetPushToken(ctx context.Context, c auth.Claims, token string) error {
	if err := domain.ValidatePushToken(token); err != nil {
		return err
	}
	return s.repo.SetPushToken(ctx, s.pool, c.SessionID, token)
}
