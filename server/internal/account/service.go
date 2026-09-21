package account

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hookah/server/internal/auth"
	"hookah/server/internal/domain"
	"hookah/server/internal/staff"
	"hookah/server/internal/store"
	"hookah/server/internal/venue"
)

type Result struct {
	Tokens domain.Tokens `json:"tokens"`
	Venue  domain.Venue  `json:"venue"`
}

type Service struct {
	pool         *pgxpool.Pool
	repo         *Repository
	staffRepo    *staff.Repository
	staff        *staff.Service
	venueRepo    *venue.Repository
	venues       *venue.Service
	passwordCost int
	// dummyHash is checked when the email is unknown, so the answer takes as
	// long as for a real account. It is made at passwordCost: a fixed hash
	// would drift from the cost the day the cost changes.
	dummyHash string
	now       func() time.Time
}

// NewService takes the bcrypt cost of new hashes: auth.PasswordCost in the
// api, a cheap one in tests.
func NewService(pool *pgxpool.Pool, repo *Repository, staffRepo *staff.Repository, staffSvc *staff.Service,
	venueRepo *venue.Repository, venues *venue.Service, passwordCost int) (*Service, error) {
	dummy, err := auth.HashPassword("not-a-password", passwordCost)
	if err != nil {
		return nil, fmt.Errorf("account service: %w", err)
	}
	return &Service{
		pool: pool, repo: repo, staffRepo: staffRepo, staff: staffSvc, venueRepo: venueRepo, venues: venues,
		passwordCost: passwordCost, dummyHash: dummy, now: time.Now,
	}, nil
}

// reserve applies the limits of tech.md section 19.16 to one login or
// register attempt and records it as a failure. The address is locked first and
// the email second, always in that order, so a parallel request of the same
// client cannot pass the check before this attempt is visible. A limited
// attempt is not recorded, otherwise spam would extend its own block.
func (s *Service) reserve(ctx context.Context, kind string, ip netip.Addr, email string, now time.Time) (int64, error) {
	var id int64
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var byIP, failed []time.Time
		var err error
		if ip.IsValid() {
			if err = store.LockKey(ctx, tx, kind+"-ip:"+ip.String()); err != nil {
				return err
			}
			if byIP, err = s.repo.RecentByIP(ctx, tx, kind, ip, now.Add(-time.Hour)); err != nil {
				return err
			}
		}
		var retry int
		var limited bool
		if kind == kindLogin {
			if err = store.LockKey(ctx, tx, "login-email:"+strings.ToLower(email)); err != nil {
				return err
			}
			if failed, err = s.repo.RecentFailedLogins(ctx, tx, email, now.Add(-time.Hour)); err != nil {
				return err
			}
			retry, limited = domain.LoginDecision(domain.LoginAttempts{IP: byIP, EmailFailed: failed}, now)
		} else {
			retry, limited = domain.RegisterDecision(byIP, now)
		}
		if limited {
			return &domain.RateLimitError{RetryAfterSec: retry}
		}
		id, err = s.repo.Reserve(ctx, tx, kind, ip, email, now)
		return err
	})
	return id, err
}

// Register creates the owner account, the venue with the default plan and
// the owner's device session in one transaction.
func (s *Service) Register(ctx context.Context, in domain.RegisterInput, ip netip.Addr) (Result, error) {
	in, err := domain.ValidateRegister(in)
	if err != nil {
		return Result{}, err
	}
	now := domain.DBTime(s.now())
	attemptID, err := s.reserve(ctx, kindRegister, ip, "", now)
	if err != nil {
		return Result{}, fmt.Errorf("register: %w", err)
	}
	hash, err := auth.HashPassword(in.Password, s.passwordCost)
	if err != nil {
		return Result{}, err
	}
	var res Result
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		accountID, ok, err := s.repo.Create(ctx, tx, in.Email, hash, in.DisplayName)
		if err != nil {
			return err
		}
		if !ok {
			v := &domain.ValidationError{}
			v.Add("email", domain.ReasonTaken)
			return v
		}
		ven, err := s.venues.Create(ctx, tx, accountID, in.VenueName)
		if err != nil {
			return err
		}
		owner, err := s.staffRepo.CreateOwner(ctx, tx, ven.ID, accountID, in.DisplayName)
		if err != nil {
			return err
		}
		tokens, err := s.staff.IssueTokens(ctx, tx, owner, in.Device, now)
		if err != nil {
			return err
		}
		res = Result{Tokens: tokens, Venue: ven}
		return s.repo.Succeed(ctx, tx, attemptID)
	})
	if err != nil {
		return Result{}, fmt.Errorf("register: %w", err)
	}
	return res, nil
}

// Login signs an owner or admin account in on a device. Wrong email and
// wrong password look the same to the caller.
func (s *Service) Login(ctx context.Context, in domain.LoginInput, ip netip.Addr) (Result, error) {
	in, err := domain.ValidateLogin(in)
	if err != nil {
		return Result{}, err
	}
	// A limited request never reaches bcrypt.
	attemptID, err := s.reserve(ctx, kindLogin, ip, in.Email, domain.DBTime(s.now()))
	if err != nil {
		return Result{}, fmt.Errorf("login: %w", err)
	}
	acc, found, err := s.repo.GetByEmail(ctx, s.pool, in.Email)
	if err != nil {
		return Result{}, fmt.Errorf("login: %w", err)
	}
	if !found {
		// Burn comparable time so response latency does not reveal registered emails.
		if _, err := auth.CheckPassword(s.dummyHash, in.Password); err != nil {
			return Result{}, fmt.Errorf("login: %w", err)
		}
		return Result{}, fmt.Errorf("login: unknown email: %w", domain.ErrUnauthorized)
	}
	ok, err := auth.CheckPassword(acc.PasswordHash, in.Password)
	if err != nil {
		return Result{}, fmt.Errorf("login: %w", err)
	}
	if !ok {
		return Result{}, fmt.Errorf("login: wrong password: %w", domain.ErrUnauthorized)
	}
	s.upgradeHash(ctx, acc, in.Password)
	now := domain.DBTime(s.now())
	var res Result
	err = store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		st, err := s.staffRepo.GetByAccount(ctx, tx, acc.ID)
		if err != nil {
			return err
		}
		if st.Blocked {
			return domain.ErrForbidden
		}
		ven, err := s.venueRepo.Get(ctx, tx, st.VenueID)
		if err != nil {
			return err
		}
		tokens, err := s.staff.IssueTokens(ctx, tx, st, in.Device, now)
		if err != nil {
			return err
		}
		res = Result{Tokens: tokens, Venue: ven.ForRole(st.Role)}
		return s.repo.Succeed(ctx, tx, attemptID)
	})
	if err != nil {
		return Result{}, fmt.Errorf("login: %w", err)
	}
	return res, nil
}

// upgradeHash replaces a hash made at a lower cost with one at the current
// cost. It runs only after the password was proven. A failure is logged and
// the login goes on: the old hash still works, and the next login retries.
func (s *Service) upgradeHash(ctx context.Context, acc Account, password string) {
	needs, err := auth.NeedsRehash(acc.PasswordHash, s.passwordCost)
	if err != nil {
		slog.WarnContext(ctx, "password hash cost", "account_id", acc.ID, "err", err)
		return
	}
	if !needs {
		return
	}
	fresh, err := auth.HashPassword(password, s.passwordCost)
	if err != nil {
		slog.WarnContext(ctx, "rehash password", "account_id", acc.ID, "err", err)
		return
	}
	if err := s.repo.UpgradeHash(ctx, s.pool, acc.ID, acc.PasswordHash, fresh); err != nil {
		slog.WarnContext(ctx, "store rehashed password", "account_id", acc.ID, "err", err)
	}
}
