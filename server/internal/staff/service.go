package staff

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"hookah/server/internal/auth"
	"hookah/server/internal/domain"
	"hookah/server/internal/layout"
	"hookah/server/internal/venue"
)

type Me struct {
	Staff domain.Staff  `json:"staff"`
	Venue domain.Venue  `json:"venue"`
	Halls []domain.Hall `json:"halls"`
}

type Service struct {
	pool   *pgxpool.Pool
	repo   *Repository
	venues *venue.Repository
	layout *layout.Repository
}

func NewService(pool *pgxpool.Pool, repo *Repository, venues *venue.Repository, layout *layout.Repository) *Service {
	return &Service{pool: pool, repo: repo, venues: venues, layout: layout}
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
