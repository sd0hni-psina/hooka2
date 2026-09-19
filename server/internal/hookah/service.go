package hookah

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"hookah/server/internal/domain"
	"hookah/server/internal/layout"
	"hookah/server/internal/staff"
	"hookah/server/internal/venue"
)

type HallRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// HallState is the snapshot of one hall, tech.md section 9.
type HallState struct {
	ServerTime  time.Time       `json:"server_time"`
	Hall        HallRef         `json:"hall"`
	Plan        domain.Plan     `json:"plan"`
	Tables      []domain.Table  `json:"tables"`
	Hookahs     []domain.Hookah `json:"hookahs"`
	StaffOnline []staff.Online  `json:"staff_online"`
}

type Service struct {
	pool   *pgxpool.Pool
	repo   *Repository
	layout *layout.Repository
	venues *venue.Repository
	staff  *staff.Repository
}

func NewService(pool *pgxpool.Pool, repo *Repository, layout *layout.Repository, venues *venue.Repository, staffRepo *staff.Repository) *Service {
	return &Service{pool: pool, repo: repo, layout: layout, venues: venues, staff: staffRepo}
}

// HallState reads the snapshot of a hall. online lists staff connected to it.
func (s *Service) HallState(ctx context.Context, venueID, hallID string, online []string, now time.Time) (HallState, error) {
	if !domain.ValidUUID(hallID) {
		return HallState{}, fmt.Errorf("hall state %q: %w", hallID, domain.ErrNotFound)
	}
	hall, err := s.layout.GetHall(ctx, s.pool, venueID, hallID)
	if err != nil {
		return HallState{}, fmt.Errorf("hall state: %w", err)
	}
	plan, err := s.venues.GetPlan(ctx, s.pool, venueID)
	if err != nil {
		return HallState{}, fmt.Errorf("hall state: %w", err)
	}
	tables, err := s.layout.ListTables(ctx, s.pool, hallID)
	if err != nil {
		return HallState{}, fmt.Errorf("hall state: %w", err)
	}
	rows, err := s.repo.ListLive(ctx, s.pool, hallID)
	if err != nil {
		return HallState{}, fmt.Errorf("hall state: %w", err)
	}
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	events, err := s.repo.ListEvents(ctx, s.pool, ids)
	if err != nil {
		return HallState{}, fmt.Errorf("hall state: %w", err)
	}
	byHookah := map[string][]domain.ServiceEvent{}
	for _, e := range events {
		byHookah[e.HookahID] = append(byHookah[e.HookahID], e)
	}
	hookahs := make([]domain.Hookah, len(rows))
	for i, r := range rows {
		h := r.Hookah
		h.Events = byHookah[h.ID]
		if h.Events == nil {
			h.Events = []domain.ServiceEvent{}
		}
		h.Status = domain.HookahStatus(&h, r.Plan.StatusPlan(), now)
		hookahs[i] = h
	}
	staffOnline := []staff.Online{}
	if len(online) > 0 {
		if staffOnline, err = s.staff.ListOnline(ctx, s.pool, venueID, online); err != nil {
			return HallState{}, fmt.Errorf("hall state: %w", err)
		}
	}
	if tables == nil {
		tables = []domain.Table{}
	}
	return HallState{
		ServerTime:  now.UTC(),
		Hall:        HallRef{ID: hall.ID, Name: hall.Name},
		Plan:        plan,
		Tables:      tables,
		Hookahs:     hookahs,
		StaffOnline: staffOnline,
	}, nil
}

// Snapshots adapts Service to realtime.Snapshotter.
type Snapshots struct{ Svc *Service }

func (a Snapshots) HallState(ctx context.Context, venueID, hallID string, online []string, now time.Time) (any, error) {
	return a.Svc.HallState(ctx, venueID, hallID, online, now)
}
