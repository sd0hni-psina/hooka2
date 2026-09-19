package account

import (
	"context"
	"fmt"
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
	pool      *pgxpool.Pool
	repo      *Repository
	staffRepo *staff.Repository
	staff     *staff.Service
	venueRepo *venue.Repository
	venues    *venue.Service
	now       func() time.Time
}

func NewService(pool *pgxpool.Pool, repo *Repository, staffRepo *staff.Repository, staffSvc *staff.Service,
	venueRepo *venue.Repository, venues *venue.Service) *Service {
	return &Service{pool: pool, repo: repo, staffRepo: staffRepo, staff: staffSvc, venueRepo: venueRepo, venues: venues, now: time.Now}
}

// Register creates the owner account, the venue with the default plan and
// the owner's device session in one transaction.
func (s *Service) Register(ctx context.Context, in domain.RegisterInput) (Result, error) {
	in, err := domain.ValidateRegister(in)
	if err != nil {
		return Result{}, err
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return Result{}, err
	}
	now := domain.DBTime(s.now())
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
		return nil
	})
	if err != nil {
		return Result{}, fmt.Errorf("register: %w", err)
	}
	return res, nil
}

// Login signs an owner or admin account in on a device. Wrong email and
// wrong password look the same to the caller.
func (s *Service) Login(ctx context.Context, in domain.LoginInput) (Result, error) {
	in, err := domain.ValidateLogin(in)
	if err != nil {
		return Result{}, err
	}
	acc, found, err := s.repo.GetByEmail(ctx, s.pool, in.Email)
	if err != nil {
		return Result{}, fmt.Errorf("login: %w", err)
	}
	if !found {
		// Burn comparable time so response latency does not reveal registered emails.
		if _, err := auth.CheckPassword(dummyHash, in.Password); err != nil {
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
		return nil
	})
	if err != nil {
		return Result{}, fmt.Errorf("login: %w", err)
	}
	return res, nil
}

// dummyHash is bcrypt("not-a-password") at cost 10.
const dummyHash = "$2a$10$t9jWBGXx7/eDl9lMni12ReZ11Gri0NVEh76499cV0ezTslb1/Mk.O"
