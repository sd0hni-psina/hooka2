package venue

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"github.com/jackc/pgx/v5/pgxpool"

	"hookah/server/internal/auth"
	"hookah/server/internal/domain"
	"hookah/server/internal/realtime"
	"hookah/server/internal/store"
)

// codeAttempts bounds the search for a free code, tech.md section 19.4.
const codeAttempts = 20

var errNoFreeCode = errors.New("no free join code")

type Service struct {
	pool *pgxpool.Pool
	repo *Repository
	hub  realtime.Broadcaster
}

func NewService(pool *pgxpool.Pool, repo *Repository, hub realtime.Broadcaster) *Service {
	return &Service{pool: pool, repo: repo, hub: hub}
}

func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", fmt.Errorf("random code: %w", err)
	}
	return domain.FormatJoinCode(int(n.Int64())), nil
}

// Create opens a venue with the default plan and a free join code. It runs
// in the caller's transaction.
func (s *Service) Create(ctx context.Context, q store.Querier, ownerID, name string) (domain.Venue, error) {
	for range codeAttempts {
		code, err := randomCode()
		if err != nil {
			return domain.Venue{}, err
		}
		v, ok, err := s.repo.Create(ctx, q, ownerID, name, code, domain.DefaultPlan())
		if err != nil || ok {
			return v, err
		}
	}
	return domain.Venue{}, errNoFreeCode
}

// RotateCode gives the venue a new code. Live sessions keep working: they
// never depend on the code after join.
func (s *Service) RotateCode(ctx context.Context, c auth.Claims) (string, error) {
	if err := auth.RequireManager(c); err != nil {
		return "", err
	}
	for range codeAttempts {
		code, err := randomCode()
		if err != nil {
			return "", err
		}
		ok, err := s.repo.SetCode(ctx, s.pool, c.VenueID, code)
		if err != nil {
			return "", fmt.Errorf("rotate code: %w", err)
		}
		if ok {
			return code, nil
		}
	}
	return "", fmt.Errorf("rotate code: %w", errNoFreeCode)
}

// PlanUpdated is the payload of plan.updated, tech.md section 9.
type PlanUpdated struct {
	Plan domain.Plan `json:"plan"`
}

// Plan returns the active service plan of the venue.
func (s *Service) Plan(ctx context.Context, c auth.Claims) (domain.Plan, error) {
	if err := auth.RequireManager(c); err != nil {
		return domain.Plan{}, err
	}
	return s.repo.GetPlan(ctx, s.pool, c.VenueID)
}

// SetPlan saves a new service plan. Live hookahs keep their snapshot, so the
// change reaches only hookahs started after it, tech.md section 7.
func (s *Service) SetPlan(ctx context.Context, c auth.Claims, plan domain.Plan) (domain.Plan, error) {
	if err := auth.RequireManager(c); err != nil {
		return domain.Plan{}, err
	}
	if err := domain.ValidatePlan(plan); err != nil {
		return domain.Plan{}, err
	}
	if err := s.repo.SetPlan(ctx, s.pool, c.VenueID, plan); err != nil {
		return domain.Plan{}, err
	}
	s.hub.Publish(c.VenueID, nil, realtime.TypePlanUpdated, PlanUpdated{Plan: plan})
	return plan, nil
}
