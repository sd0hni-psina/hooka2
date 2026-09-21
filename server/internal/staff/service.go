package staff

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
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
	evictor    Evictor
	now        func() time.Time
}

func NewService(pool *pgxpool.Pool, repo *Repository, venues *venue.Repository, layout *layout.Repository,
	tokens *auth.Tokens, refreshTTL time.Duration, evictor Evictor) *Service {
	return &Service{
		pool: pool, repo: repo, venues: venues, layout: layout,
		tokens: tokens, refreshTTL: refreshTTL, evictor: evictor, now: time.Now,
	}
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
	// Blocked comes first: blocking revokes the sessions too, and the master
	// must learn that access is closed, not that the token went stale.
	if found && blocked {
		return fmt.Errorf("staff %s blocked: %w", c.StaffID, domain.ErrForbidden)
	}
	if !found || revoked {
		return fmt.Errorf("session %s not live: %w", c.SessionID, domain.ErrUnauthorized)
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

// Join signs a master in by venue code, tech.md sections 8, 19.1, 19.4 and
// 19.16. A wrong code is recorded before the error returns, so the attempt
// counts toward the limits of its device, its address and the service.
//
// The device and the address are locked, in that order, so parallel requests
// of one client cannot pass the check before the first one is recorded. The
// service budget takes no lock of its own: a lock shared by every join would
// queue all clients behind one, and the overshoot is bounded by the pool size.
func (s *Service) Join(ctx context.Context, in domain.JoinInput, ip netip.Addr) (JoinResult, error) {
	in, err := domain.ValidateJoin(in)
	if err != nil {
		return JoinResult{}, err
	}
	now := domain.DBTime(s.now())

	var res JoinResult
	var outcome error
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := store.LockKey(ctx, tx, "join:"+in.DeviceID); err != nil {
			return err
		}
		since := now.Add(-time.Hour)
		var attempts domain.JoinAttempts
		var err error
		if attempts.Device, err = s.repo.RecentAttempts(ctx, tx, in.DeviceID, since); err != nil {
			return err
		}
		if ip.IsValid() {
			if err := store.LockKey(ctx, tx, "join-ip:"+ip.String()); err != nil {
				return err
			}
			if attempts.IP, err = s.repo.RecentIPAttempts(ctx, tx, ip, since); err != nil {
				return err
			}
		}
		if attempts.Failed, err = s.repo.RecentFailedAttempts(ctx, tx, since); err != nil {
			return err
		}
		if retry, limited := domain.JoinDecision(attempts, now); limited {
			outcome = &domain.RateLimitError{RetryAfterSec: retry}
			return nil
		}

		v, found, err := s.venues.GetByCode(ctx, tx, in.Code)
		if err != nil {
			return err
		}
		if err := s.repo.RecordAttempt(ctx, tx, in.DeviceID, in.Code, found, ip, now); err != nil {
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

// Logout revokes the session and cuts its open sockets. The sockets close
// after the revoke: the access token is checked once at the upgrade, so
// without this a signed out device would keep listening.
func (s *Service) Logout(ctx context.Context, c auth.Claims) error {
	if err := s.repo.RevokeSession(ctx, s.pool, c.SessionID); err != nil {
		return err
	}
	s.evictor.CloseSession(c.VenueID, c.SessionID)
	return nil
}

func (s *Service) SetPushToken(ctx context.Context, c auth.Claims, token string) error {
	if err := domain.ValidatePushToken(token); err != nil {
		return err
	}
	return s.repo.SetPushToken(ctx, s.pool, c.SessionID, token)
}

// Evictor closes open connections of a staff member or of one session. The
// realtime hub implements it.
type Evictor interface {
	CloseStaff(venueID, staffID string)
	CloseSession(venueID, sessionID string)
}

// List returns the whole staff of the venue, tech.md section 19.11.
func (s *Service) List(ctx context.Context, c auth.Claims) ([]domain.Staff, error) {
	if err := auth.RequireManager(c); err != nil {
		return nil, err
	}
	list, err := s.repo.ListVenue(ctx, s.pool, c.VenueID)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []domain.Staff{}
	}
	return list, nil
}

// Patch changes a role or blocks a staff member. The rules of tech.md
// section 19.11: only the owner moves roles, nobody touches the owner, and
// nobody blocks themselves.
func (s *Service) Patch(ctx context.Context, c auth.Claims, staffID string, role *domain.Role, blocked *bool) (domain.Staff, error) {
	if err := auth.RequireManager(c); err != nil {
		return domain.Staff{}, err
	}
	if !domain.ValidUUID(staffID) {
		return domain.Staff{}, fmt.Errorf("staff %q: %w", staffID, domain.ErrNotFound)
	}
	if role == nil && blocked == nil {
		v := &domain.ValidationError{}
		v.Add("body", domain.ReasonRequired)
		return domain.Staff{}, v
	}
	if role != nil && *role != domain.RoleAdmin && *role != domain.RoleMaster {
		v := &domain.ValidationError{}
		v.Add("role", domain.ReasonInvalid)
		return domain.Staff{}, v
	}
	if role != nil && c.Role != domain.RoleOwner {
		return domain.Staff{}, fmt.Errorf("only the owner changes roles: %w", domain.ErrForbidden)
	}
	if blocked != nil && *blocked && staffID == c.StaffID {
		return domain.Staff{}, fmt.Errorf("blocking yourself: %w", domain.ErrForbidden)
	}
	now := domain.DBTime(s.now())

	var out domain.Staff
	var evict bool
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		target, err := s.repo.Get(ctx, tx, c.VenueID, staffID)
		if err != nil {
			return err
		}
		if target.Role == domain.RoleOwner {
			return fmt.Errorf("the owner cannot be changed: %w", domain.ErrForbidden)
		}
		if role != nil {
			if err := s.repo.SetRole(ctx, tx, staffID, *role); err != nil {
				return err
			}
		}
		if blocked != nil {
			if err := s.repo.SetBlocked(ctx, tx, staffID, *blocked, now); err != nil {
				return err
			}
			evict = *blocked
		}
		out, err = s.repo.Get(ctx, tx, c.VenueID, staffID)
		return err
	})
	if err != nil {
		return domain.Staff{}, err
	}
	if evict {
		s.evictor.CloseStaff(c.VenueID, staffID)
	}
	return out, nil
}
