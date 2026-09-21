package hookah

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"hookah/server/internal/domain"
	"hookah/server/internal/store"
)

type Repository struct{}

func NewRepository() *Repository { return &Repository{} }

// Row is a hookah with the plan snapshot it runs on.
type Row struct {
	domain.Hookah
	Plan domain.Plan
}

const hookahColumns = `h.id, h.table_id, h.hall_id, h.state, h.started_at, h.expires_at, h.finished_at,
	h.started_by, st.display_name, h.plan_snapshot`

func scanRow(row pgx.CollectableRow) (Row, error) {
	var r Row
	err := row.Scan(&r.ID, &r.TableID, &r.HallID, &r.State, &r.StartedAt, &r.ExpiresAt, &r.FinishedAt,
		&r.StartedBy, &r.StartedByName, &r.Plan)
	return r, err
}

func (Repository) ListLive(ctx context.Context, q store.Querier, hallID string) ([]Row, error) {
	rows, err := q.Query(ctx,
		`select `+hookahColumns+` from hookahs h join staff st on st.id = h.started_by
		 where h.hall_id = $1 and h.state in ('active', 'expired') order by h.started_at`, hallID)
	if err != nil {
		return nil, fmt.Errorf("list live hookahs: %w", err)
	}
	out, err := pgx.CollectRows(rows, scanRow)
	if err != nil {
		return nil, fmt.Errorf("list live hookahs: %w", err)
	}
	return out, nil
}

const eventColumns = `e.id, e.hookah_id, e.seq, e.kind, e.title, e.due_at, e.state,
	e.claimed_by, cl.display_name, e.claimed_at, e.confirmed_by, cf.display_name, e.confirmed_at, e.confirmed_early`

const eventJoins = ` from service_events e
	left join staff cl on cl.id = e.claimed_by
	left join staff cf on cf.id = e.confirmed_by`

func scanEvent(row pgx.CollectableRow) (domain.ServiceEvent, error) {
	var e domain.ServiceEvent
	err := row.Scan(&e.ID, &e.HookahID, &e.Seq, &e.Kind, &e.Title, &e.DueAt, &e.State,
		&e.ClaimedBy, &e.ClaimedByName, &e.ClaimedAt, &e.ConfirmedBy, &e.ConfirmedByName, &e.ConfirmedAt, &e.ConfirmedEarly)
	return e, err
}

// ListEvents returns events of the given hookahs ordered by hookah and seq.
func (Repository) ListEvents(ctx context.Context, q store.Querier, hookahIDs []string) ([]domain.ServiceEvent, error) {
	rows, err := q.Query(ctx,
		`select `+eventColumns+eventJoins+` where e.hookah_id = any($1::uuid[]) order by e.hookah_id, e.seq`, hookahIDs)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	out, err := pgx.CollectRows(rows, scanEvent)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	return out, nil
}

// StartInput is a new hookah row.
type StartInput struct {
	VenueID        string
	HallID         string
	TableID        string
	StartedBy      string
	StartedAt      time.Time
	ExpiresAt      time.Time
	Plan           domain.Plan
	IdempotencyKey string
}

// Insert creates the hookah. inserted is false when a unique index refused it:
// the table already holds a live hookah, or the idempotency key was used
// before. The caller tells the two apart by looking the key up. Skipping the
// row with ON CONFLICT keeps the transaction usable, a raised unique violation
// would abort it and the lookup that follows would fail with SQLSTATE 25P02.
func (Repository) Insert(ctx context.Context, q store.Querier, in StartInput) (id string, inserted bool, err error) {
	err = q.QueryRow(ctx,
		`insert into hookahs (venue_id, hall_id, table_id, state, started_at, expires_at, plan_snapshot, started_by, idempotency_key)
		 values ($1, $2, $3, 'active', $4, $5, $6, $7, $8)
		 on conflict do nothing
		 returning id`,
		in.VenueID, in.HallID, in.TableID, in.StartedAt, in.ExpiresAt, in.Plan, in.StartedBy, in.IdempotencyKey).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("insert hookah: %w", err)
	}
	return id, true, nil
}

// InsertEvents writes the materialized plan of a hookah.
func (Repository) InsertEvents(ctx context.Context, q store.Querier, hookahID string, events []domain.Event) error {
	batch := &pgx.Batch{}
	for _, e := range events {
		batch.Queue(
			`insert into service_events (hookah_id, seq, kind, title, due_at, state)
			 values ($1, $2, $3, $4, $5, 'pending')`, hookahID, e.Seq, e.Kind, e.Title, e.DueAt)
	}
	if err := q.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("insert service events: %w", err)
	}
	return nil
}

// ByIdempotencyKey finds the hookah a repeated request already created.
func (r Repository) ByIdempotencyKey(ctx context.Context, q store.Querier, venueID, key string) (Row, bool, error) {
	rows, err := q.Query(ctx,
		`select `+hookahColumns+` from hookahs h join staff st on st.id = h.started_by
		 where h.venue_id = $1 and h.idempotency_key = $2`, venueID, key)
	if err != nil {
		return Row{}, false, fmt.Errorf("hookah by key: %w", err)
	}
	return oneRow(rows)
}

// Get loads one hookah of the venue and locks it for the transaction.
func (Repository) GetForUpdate(ctx context.Context, q store.Querier, venueID, hookahID string) (Row, bool, error) {
	rows, err := q.Query(ctx,
		`select `+hookahColumns+` from hookahs h join staff st on st.id = h.started_by
		 where h.id = $1 and h.venue_id = $2 for update of h`, hookahID, venueID)
	if err != nil {
		return Row{}, false, fmt.Errorf("get hookah: %w", err)
	}
	return oneRow(rows)
}

func (Repository) Get(ctx context.Context, q store.Querier, venueID, hookahID string) (Row, bool, error) {
	rows, err := q.Query(ctx,
		`select `+hookahColumns+` from hookahs h join staff st on st.id = h.started_by
		 where h.id = $1 and h.venue_id = $2`, hookahID, venueID)
	if err != nil {
		return Row{}, false, fmt.Errorf("get hookah: %w", err)
	}
	return oneRow(rows)
}

func oneRow(rows pgx.Rows) (Row, bool, error) {
	list, err := pgx.CollectRows(rows, scanRow)
	if err != nil {
		return Row{}, false, fmt.Errorf("scan hookah: %w", err)
	}
	if len(list) == 0 {
		return Row{}, false, nil
	}
	return list[0], true, nil
}

// Move changes the table of a live hookah and records the transfer. occupied
// is true when the destination already holds a live hookah.
func (Repository) Move(ctx context.Context, q store.Querier, hookahID, fromTableID, toTableID, toHallID, movedBy string) (occupied bool, err error) {
	_, err = q.Exec(ctx,
		`update hookahs set table_id = $2, hall_id = $3, updated_at = now() where id = $1`, hookahID, toTableID, toHallID)
	if store.IsUniqueViolation(err, "hookahs_active_table_key") {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("move hookah: %w", err)
	}
	if _, err := q.Exec(ctx,
		`insert into hookah_transfers (hookah_id, from_table_id, to_table_id, moved_by) values ($1, $2, $3, $4)`,
		hookahID, fromTableID, toTableID, movedBy); err != nil {
		return false, fmt.Errorf("record transfer: %w", err)
	}
	return false, nil
}

// Finish closes a hookah and cancels its pending events.
func (Repository) Finish(ctx context.Context, q store.Querier, hookahID, finishedBy string, reason domain.FinishReason, now time.Time) error {
	if _, err := q.Exec(ctx,
		`update hookahs set state = 'finished', finished_at = $2, finished_by = $3, finish_reason = $4, updated_at = now()
		 where id = $1`, hookahID, now, finishedBy, reason); err != nil {
		return fmt.Errorf("finish hookah: %w", err)
	}
	if _, err := q.Exec(ctx,
		`update service_events set state = 'cancelled' where hookah_id = $1 and state = 'pending'`, hookahID); err != nil {
		return fmt.Errorf("cancel events: %w", err)
	}
	return nil
}

// Events loads the events of one hookah in seq order.
func (r Repository) Events(ctx context.Context, q store.Querier, hookahID string) ([]domain.ServiceEvent, error) {
	return r.ListEvents(ctx, q, []string{hookahID})
}

// TableRef is the table a hookah stands on.
type TableRef struct {
	ID     string
	HallID string
	Label  string
}

// LockTable takes the table row so two starts on one table queue up.
func (Repository) LockTable(ctx context.Context, q store.Querier, venueID, tableID string) (TableRef, bool, error) {
	var t TableRef
	err := q.QueryRow(ctx,
		`select t.id, t.hall_id, t.label from tables t join halls h on h.id = t.hall_id
		 where t.id = $1 and h.venue_id = $2 and t.archived_at is null for update of t`, tableID, venueID,
	).Scan(&t.ID, &t.HallID, &t.Label)
	if errors.Is(err, pgx.ErrNoRows) {
		return TableRef{}, false, nil
	}
	if err != nil {
		return TableRef{}, false, fmt.Errorf("lock table: %w", err)
	}
	return t, true, nil
}

func (Repository) TableLabel(ctx context.Context, q store.Querier, tableID string) (string, error) {
	var label string
	if err := q.QueryRow(ctx, `select label from tables where id = $1`, tableID).Scan(&label); err != nil {
		return "", fmt.Errorf("table label: %w", err)
	}
	return label, nil
}

// MarkDue moves ripe events to due, tech.md section 7 query 1. The state
// change itself makes the query idempotent: a second tick finds no rows.
func (Repository) MarkDue(ctx context.Context, q store.Querier, now time.Time, limit int) ([]domain.ServiceEvent, error) {
	rows, err := q.Query(ctx, `
		update service_events set state = 'due', fired_at = $1
		where id in (
			select e.id from service_events e
			join hookahs h on h.id = e.hookah_id
			where e.state = 'pending' and e.due_at <= $1 and h.state <> 'finished'
			order by e.due_at
			for update skip locked
			limit $2
		)
		returning id, hookah_id, seq, kind, title, due_at, state, claimed_by, null::text, claimed_at,
		          confirmed_by, null::text, confirmed_at, confirmed_early`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("mark due: %w", err)
	}
	out, err := pgx.CollectRows(rows, scanEvent)
	if err != nil {
		return nil, fmt.Errorf("mark due: %w", err)
	}
	return out, nil
}

// MarkMissed closes events nobody confirmed in time, query 2. The deadline
// comes from the plan snapshot of the hookah, tech.md section 19.8.
func (Repository) MarkMissed(ctx context.Context, q store.Querier, now time.Time, limit int) ([]domain.ServiceEvent, error) {
	rows, err := q.Query(ctx, `
		update service_events set state = 'missed'
		where id in (
			select e.id from service_events e
			join hookahs h on h.id = e.hookah_id
			where e.state = 'due'
			  and e.due_at + (((h.plan_snapshot->>'missed_after_sec')::int) * interval '1 second') <= $1
			order by e.due_at
			for update skip locked
			limit $2
		)
		returning id, hookah_id, seq, kind, title, due_at, state, claimed_by, null::text, claimed_at,
		          confirmed_by, null::text, confirmed_at, confirmed_early`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("mark missed: %w", err)
	}
	out, err := pgx.CollectRows(rows, scanEvent)
	if err != nil {
		return nil, fmt.Errorf("mark missed: %w", err)
	}
	return out, nil
}

// MarkExpired moves hookahs that ran out of resource, query 3.
func (Repository) MarkExpired(ctx context.Context, q store.Querier, now time.Time, limit int) ([]string, error) {
	rows, err := q.Query(ctx, `
		update hookahs set state = 'expired', updated_at = now()
		where id in (
			select id from hookahs where state = 'active' and expires_at <= $1
			order by expires_at
			for update skip locked
			limit $2
		)
		returning id`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("mark expired: %w", err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("mark expired: %w", err)
	}
	return out, nil
}

// Context is everything publishing an event needs about its hookah.
type Context struct {
	Row        Row
	VenueID    string
	TableLabel string
	Events     []domain.ServiceEvent
}

// Contexts loads hookahs by id together with their events and table label.
func (r Repository) Contexts(ctx context.Context, q store.Querier, ids []string) (map[string]Context, error) {
	if len(ids) == 0 {
		return map[string]Context{}, nil
	}
	rows, err := q.Query(ctx,
		`select `+hookahColumns+`, h.venue_id, t.label from hookahs h
		 join staff st on st.id = h.started_by
		 join tables t on t.id = h.table_id
		 where h.id = any($1::uuid[])`, ids)
	if err != nil {
		return nil, fmt.Errorf("hookah contexts: %w", err)
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Context, error) {
		var c Context
		r := &c.Row
		err := row.Scan(&r.ID, &r.TableID, &r.HallID, &r.State, &r.StartedAt, &r.ExpiresAt, &r.FinishedAt,
			&r.StartedBy, &r.StartedByName, &r.Plan, &c.VenueID, &c.TableLabel)
		return c, err
	})
	if err != nil {
		return nil, fmt.Errorf("hookah contexts: %w", err)
	}
	events, err := r.ListEvents(ctx, q, ids)
	if err != nil {
		return nil, err
	}
	byHookah := map[string][]domain.ServiceEvent{}
	for _, e := range events {
		byHookah[e.HookahID] = append(byHookah[e.HookahID], e)
	}
	out := make(map[string]Context, len(list))
	for _, c := range list {
		c.Events = byHookah[c.Row.ID]
		out[c.Row.ID] = c
	}
	return out, nil
}

// EventRow is a service event with the hookah it belongs to.
type EventRow struct {
	Event  domain.ServiceEvent
	Hookah Row
}

// GetEventForUpdate locks the event and its hookah for the transaction.
func (r Repository) GetEventForUpdate(ctx context.Context, q store.Querier, venueID, eventID string) (EventRow, bool, error) {
	var out EventRow
	e, h := &out.Event, &out.Hookah
	err := q.QueryRow(ctx,
		`select e.id, e.hookah_id, e.seq, e.kind, e.title, e.due_at, e.state,
		        e.claimed_by, e.claimed_at, e.confirmed_by, e.confirmed_at, e.confirmed_early,
		        h.id, h.table_id, h.hall_id, h.state, h.started_at, h.expires_at, h.finished_at,
		        h.started_by, st.display_name, h.plan_snapshot
		 from service_events e
		 join hookahs h on h.id = e.hookah_id
		 join staff st on st.id = h.started_by
		 where e.id = $1 and h.venue_id = $2
		 for update of e, h`, eventID, venueID,
	).Scan(&e.ID, &e.HookahID, &e.Seq, &e.Kind, &e.Title, &e.DueAt, &e.State,
		&e.ClaimedBy, &e.ClaimedAt, &e.ConfirmedBy, &e.ConfirmedAt, &e.ConfirmedEarly,
		&h.ID, &h.TableID, &h.HallID, &h.State, &h.StartedAt, &h.ExpiresAt, &h.FinishedAt,
		&h.StartedBy, &h.StartedByName, &h.Plan)
	if errors.Is(err, pgx.ErrNoRows) {
		return EventRow{}, false, nil
	}
	if err != nil {
		return EventRow{}, false, fmt.Errorf("get event: %w", err)
	}
	return out, true, nil
}

// Claim takes a due task. ok is false when another master was first.
func (Repository) Claim(ctx context.Context, q store.Querier, eventID, staffID string, now time.Time) (bool, error) {
	tag, err := q.Exec(ctx,
		`update service_events set claimed_by = $2, claimed_at = $3
		 where id = $1 and state = 'due' and claimed_by is null`, eventID, staffID, now)
	if err != nil {
		return false, fmt.Errorf("claim event: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Confirm closes a task. The condition makes two simultaneous confirmations
// safe: the loser changes nothing and reads the winner's row.
func (Repository) Confirm(ctx context.Context, q store.Querier, eventID, staffID string, early bool, now time.Time) (bool, error) {
	tag, err := q.Exec(ctx,
		`update service_events set state = 'confirmed', confirmed_by = $2, confirmed_at = $3, confirmed_early = $4
		 where id = $1 and state in ('due', 'pending') and confirmed_at is null`, eventID, staffID, now, early)
	if err != nil {
		return false, fmt.Errorf("confirm event: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Event reads one event with the staff names filled in.
func (Repository) Event(ctx context.Context, q store.Querier, eventID string) (domain.ServiceEvent, error) {
	rows, err := q.Query(ctx, `select `+eventColumns+eventJoins+` where e.id = $1`, eventID)
	if err != nil {
		return domain.ServiceEvent{}, fmt.Errorf("read event: %w", err)
	}
	e, err := pgx.CollectExactlyOneRow(rows, scanEvent)
	if err != nil {
		return domain.ServiceEvent{}, fmt.Errorf("read event: %w", err)
	}
	return e, nil
}
