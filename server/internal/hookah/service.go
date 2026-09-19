package hookah

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"hookah/server/internal/auth"
	"hookah/server/internal/domain"
	"hookah/server/internal/layout"
	"hookah/server/internal/realtime"
	"hookah/server/internal/scheduler"
	"hookah/server/internal/staff"
	"hookah/server/internal/store"
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
	hub    realtime.Broadcaster
	now    func() time.Time
}

func NewService(pool *pgxpool.Pool, repo *Repository, layout *layout.Repository, venues *venue.Repository,
	staffRepo *staff.Repository, hub realtime.Broadcaster) *Service {
	return &Service{pool: pool, repo: repo, layout: layout, venues: venues, staff: staffRepo, hub: hub, now: time.Now}
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

// Event payloads of tech.md sections 9 and 19.8.
type StartedData struct {
	Hookah domain.Hookah         `json:"hookah"`
	Events []domain.ServiceEvent `json:"events"`
	Status domain.Status         `json:"status"`
}

type TransferredData struct {
	Hookah      domain.Hookah `json:"hookah"`
	FromTableID string        `json:"from_table_id"`
	ToTableID   string        `json:"to_table_id"`
	MovedBy     string        `json:"moved_by"`
	MovedByName string        `json:"moved_by_name"`
	Status      domain.Status `json:"status"`
}

type ExpiredData struct {
	HookahID string        `json:"hookah_id"`
	TableID  string        `json:"table_id"`
	Status   domain.Status `json:"status"`
}

type FinishedData struct {
	HookahID   string              `json:"hookah_id"`
	TableID    string              `json:"table_id"`
	Reason     domain.FinishReason `json:"reason"`
	FinishedBy string              `json:"finished_by"`
}

type DueData struct {
	Event      domain.ServiceEvent `json:"event"`
	HookahID   string              `json:"hookah_id"`
	TableID    string              `json:"table_id"`
	TableLabel string              `json:"table_label"`
	Status     domain.Status       `json:"status"`
}

type MissedData struct {
	EventID  string        `json:"event_id"`
	HookahID string        `json:"hookah_id"`
	TableID  string        `json:"table_id"`
	Status   domain.Status `json:"status"`
}

func notFound(what, id string) error {
	return fmt.Errorf("%s %q: %w", what, id, domain.ErrNotFound)
}

// load reads a hookah with its events and the status at now.
func (s *Service) load(ctx context.Context, q store.Querier, row Row, now time.Time) (domain.Hookah, error) {
	events, err := s.repo.Events(ctx, q, row.ID)
	if err != nil {
		return domain.Hookah{}, err
	}
	h := row.Hookah
	h.Events = events
	if h.Events == nil {
		h.Events = []domain.ServiceEvent{}
	}
	h.Status = domain.HookahStatus(&h, row.Plan.StatusPlan(), now)
	return h, nil
}

// Start opens a hookah on a table, tech.md section 19.8. The whole plan is
// materialized in the same transaction, so every deadline exists at once.
func (s *Service) Start(ctx context.Context, c auth.Claims, tableID, idempotencyKey string) (domain.Hookah, error) {
	if err := domain.ValidateIdempotencyKey(idempotencyKey); err != nil {
		return domain.Hookah{}, err
	}
	if !domain.ValidUUID(tableID) {
		return domain.Hookah{}, notFound("table", tableID)
	}
	now := domain.DBTime(s.now())

	var out domain.Hookah
	var fresh bool
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if row, found, err := s.repo.ByIdempotencyKey(ctx, tx, c.VenueID, idempotencyKey); err != nil {
			return err
		} else if found {
			out, err = s.load(ctx, tx, row, now)
			return err
		}

		table, found, err := s.repo.LockTable(ctx, tx, c.VenueID, tableID)
		if err != nil {
			return err
		}
		if !found {
			return notFound("table", tableID)
		}
		plan, err := s.venues.GetPlan(ctx, tx, c.VenueID)
		if err != nil {
			return err
		}
		events := domain.MaterializePlan(plan, now)
		if len(events) == 0 {
			return fmt.Errorf("start hookah: plan of venue %s has no events", c.VenueID)
		}
		id, occupied, repeated, err := s.repo.Insert(ctx, tx, StartInput{
			VenueID: c.VenueID, HallID: table.HallID, TableID: table.ID, StartedBy: c.StaffID,
			StartedAt: now, ExpiresAt: events[len(events)-1].DueAt, Plan: plan, IdempotencyKey: idempotencyKey,
		})
		switch {
		case occupied:
			return fmt.Errorf("start hookah on table %s: %w", tableID, domain.ErrTableOccupied)
		case repeated:
			row, found, err := s.repo.ByIdempotencyKey(ctx, tx, c.VenueID, idempotencyKey)
			if err != nil {
				return err
			}
			if !found {
				return fmt.Errorf("start hookah: idempotency key %s vanished", idempotencyKey)
			}
			out, err = s.load(ctx, tx, row, now)
			return err
		case err != nil:
			return err
		}
		if err := s.repo.InsertEvents(ctx, tx, id, events); err != nil {
			return err
		}
		row, found, err := s.repo.GetForUpdate(ctx, tx, c.VenueID, id)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("start hookah: %s vanished", id)
		}
		if out, err = s.load(ctx, tx, row, now); err != nil {
			return err
		}
		fresh = true
		return nil
	})
	if err != nil {
		return domain.Hookah{}, err
	}
	if fresh {
		hallID := out.HallID
		s.hub.Publish(c.VenueID, &hallID, realtime.TypeHookahStarted, StartedData{Hookah: out, Events: out.Events, Status: out.Status})
	}
	return out, nil
}

// Transfer moves a live hookah to another table keeping every deadline.
func (s *Service) Transfer(ctx context.Context, c auth.Claims, hookahID, toTableID string) (domain.Hookah, error) {
	if !domain.ValidUUID(hookahID) {
		return domain.Hookah{}, notFound("hookah", hookahID)
	}
	if !domain.ValidUUID(toTableID) {
		return domain.Hookah{}, notFound("table", toTableID)
	}
	now := domain.DBTime(s.now())

	var out domain.Hookah
	var moved bool
	var fromTableID string
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		row, found, err := s.repo.GetForUpdate(ctx, tx, c.VenueID, hookahID)
		if err != nil {
			return err
		}
		if !found {
			return notFound("hookah", hookahID)
		}
		if err := domain.CheckMutable(row.State); err != nil {
			return fmt.Errorf("transfer hookah %s: %w", hookahID, err)
		}
		table, found, err := s.repo.LockTable(ctx, tx, c.VenueID, toTableID)
		if err != nil {
			return err
		}
		if !found {
			return notFound("table", toTableID)
		}
		fromTableID = row.TableID
		if fromTableID == table.ID {
			out, err = s.load(ctx, tx, row, now)
			return err
		}
		occupied, err := s.repo.Move(ctx, tx, hookahID, fromTableID, table.ID, table.HallID, c.StaffID)
		if err != nil {
			return err
		}
		if occupied {
			return fmt.Errorf("transfer to table %s: %w", toTableID, domain.ErrTableOccupied)
		}
		row.TableID, row.HallID = table.ID, table.HallID
		if out, err = s.load(ctx, tx, row, now); err != nil {
			return err
		}
		moved = true
		return nil
	})
	if err != nil {
		return domain.Hookah{}, err
	}
	if moved {
		hallID := out.HallID
		s.hub.Publish(c.VenueID, &hallID, realtime.TypeHookahTransfer, TransferredData{
			Hookah: out, FromTableID: fromTableID, ToTableID: out.TableID,
			MovedBy: c.StaffID, MovedByName: s.staffName(ctx, c.VenueID, c.StaffID), Status: out.Status,
		})
	}
	return out, nil
}

// Finish closes a hookah and cancels its future events.
func (s *Service) Finish(ctx context.Context, c auth.Claims, hookahID string, reason domain.FinishReason) (domain.Hookah, error) {
	if !domain.ValidUUID(hookahID) {
		return domain.Hookah{}, notFound("hookah", hookahID)
	}
	if !reason.Valid() {
		v := &domain.ValidationError{}
		v.Add("reason", domain.ReasonInvalid)
		return domain.Hookah{}, v
	}
	now := domain.DBTime(s.now())

	var out domain.Hookah
	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		row, found, err := s.repo.GetForUpdate(ctx, tx, c.VenueID, hookahID)
		if err != nil {
			return err
		}
		if !found {
			return notFound("hookah", hookahID)
		}
		if err := domain.CheckMutable(row.State); err != nil {
			return fmt.Errorf("finish hookah %s: %w", hookahID, err)
		}
		if err := s.repo.Finish(ctx, tx, hookahID, c.StaffID, reason, now); err != nil {
			return err
		}
		row.State = domain.HookahFinished
		row.FinishedAt = &now
		out, err = s.load(ctx, tx, row, now)
		return err
	})
	if err != nil {
		return domain.Hookah{}, err
	}
	hallID := out.HallID
	s.hub.Publish(c.VenueID, &hallID, realtime.TypeHookahFinished, FinishedData{
		HookahID: out.ID, TableID: out.TableID, Reason: reason, FinishedBy: c.StaffID,
	})
	return out, nil
}

// staffName is best effort: an empty name only costs a badge label.
func (s *Service) staffName(ctx context.Context, venueID, staffID string) string {
	list, err := s.staff.ListOnline(ctx, s.pool, venueID, []string{staffID})
	if err != nil || len(list) == 0 {
		return ""
	}
	return list[0].DisplayName
}

// tickLimit bounds one scheduler pass, tech.md section 7.
const tickLimit = 200

// Tick runs the three scheduler queries and publishes what changed. Every
// query is idempotent by state, so a repeated tick has no effect.
func (s *Service) Tick(ctx context.Context, now time.Time) (scheduler.Result, error) {
	now = domain.DBTime(now)
	var res scheduler.Result
	var due, missed []domain.ServiceEvent
	var expired []string

	err := store.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var err error
		if due, err = s.repo.MarkDue(ctx, tx, now, tickLimit); err != nil {
			return err
		}
		if missed, err = s.repo.MarkMissed(ctx, tx, now, tickLimit); err != nil {
			return err
		}
		expired, err = s.repo.MarkExpired(ctx, tx, now, tickLimit)
		return err
	})
	if err != nil {
		return res, fmt.Errorf("scheduler tick: %w", err)
	}
	res = scheduler.Result{Due: len(due), Missed: len(missed), Expired: len(expired)}
	if res.Due+res.Missed+res.Expired == 0 {
		return res, nil
	}

	ids := make([]string, 0, len(due)+len(missed)+len(expired))
	for _, e := range append(append([]domain.ServiceEvent{}, due...), missed...) {
		ids = append(ids, e.HookahID)
	}
	ids = append(ids, expired...)
	contexts, err := s.repo.Contexts(ctx, s.pool, ids)
	if err != nil {
		return res, err
	}

	for _, e := range due {
		c, ok := contexts[e.HookahID]
		if !ok {
			continue
		}
		hallID := c.Row.HallID
		s.hub.Publish(c.VenueID, &hallID, realtime.TypeServiceDue, DueData{
			Event: pick(c.Events, e.ID, e), HookahID: e.HookahID, TableID: c.Row.TableID,
			TableLabel: c.TableLabel, Status: statusOf(c, now),
		})
	}
	for _, e := range missed {
		c, ok := contexts[e.HookahID]
		if !ok {
			continue
		}
		hallID := c.Row.HallID
		s.hub.Publish(c.VenueID, &hallID, realtime.TypeServiceMissed, MissedData{
			EventID: e.ID, HookahID: e.HookahID, TableID: c.Row.TableID, Status: statusOf(c, now),
		})
	}
	for _, id := range expired {
		c, ok := contexts[id]
		if !ok {
			continue
		}
		hallID := c.Row.HallID
		s.hub.Publish(c.VenueID, &hallID, realtime.TypeHookahExpired, ExpiredData{
			HookahID: id, TableID: c.Row.TableID, Status: statusOf(c, now),
		})
	}
	return res, nil
}

// pick returns the freshly read event, falling back to the updated row.
func pick(events []domain.ServiceEvent, id string, fallback domain.ServiceEvent) domain.ServiceEvent {
	for _, e := range events {
		if e.ID == id {
			return e
		}
	}
	return fallback
}

func statusOf(c Context, now time.Time) domain.Status {
	h := c.Row.Hookah
	h.Events = c.Events
	return domain.HookahStatus(&h, c.Row.Plan.StatusPlan(), now)
}
