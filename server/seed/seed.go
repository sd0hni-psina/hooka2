// Package seed fills a database with one realistic venue. The same fixture
// serves local development, integration tests and fakes.
package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"hookah/server/internal/auth"
	"hookah/server/internal/domain"
	"hookah/server/internal/store"
)

const (
	JoinCode      = "123456"
	OwnerEmail    = "owner@hookah.dev"
	OwnerPassword = "hookah-dev-pass" //nolint:gosec // public dev fixture, never used in prod
	OwnerName     = "Арсен"
)

var MasterNames = []string{"Ерлан", "Айдос", "Тимур"}

type Member struct {
	StaffID   string
	SessionID string
	Name      string
	Role      domain.Role
}

type Fixture struct {
	VenueID  string
	Owner    Member
	Masters  []Member
	HallIDs  []string
	TableIDs map[string]string // label -> id
	// FreshHookahID started 3 minutes ago and has nothing due.
	FreshHookahID string
	// BusyHookahID started 16 minutes ago: check confirmed, shake due and claimed.
	BusyHookahID string
}

type tableSpec struct {
	label      string
	shape      domain.Shape
	x, y, w, h int
	seats      int
}

// Two halls, 18 tables: booths along the walls, round tables in the middle.
var halls = []struct {
	name   string
	tables []tableSpec
}{
	{"Основной зал", []tableSpec{
		{"1", domain.ShapeRect, 4, 4, 14, 8, 6}, {"2", domain.ShapeRect, 22, 4, 14, 8, 6},
		{"3", domain.ShapeRect, 40, 4, 14, 8, 6}, {"4", domain.ShapeRect, 58, 4, 14, 8, 6},
		{"5", domain.ShapeRect, 76, 4, 14, 8, 6},
		{"6", domain.ShapeCircle, 12, 34, 10, 10, 4}, {"7", domain.ShapeCircle, 34, 34, 10, 10, 4},
		{"8", domain.ShapeCircle, 56, 34, 10, 10, 4}, {"9", domain.ShapeCircle, 78, 34, 10, 10, 4},
		{"10", domain.ShapeRect, 4, 84, 20, 10, 8},
	}},
	{"Второй зал", []tableSpec{
		{"11", domain.ShapeRect, 6, 6, 12, 8, 4}, {"12", domain.ShapeRect, 26, 6, 12, 8, 4},
		{"13", domain.ShapeRect, 46, 6, 12, 8, 4}, {"14", domain.ShapeCircle, 16, 40, 10, 10, 4},
		{"15", domain.ShapeRect, 40, 40, 12, 8, 4}, {"16", domain.ShapeCircle, 66, 40, 10, 10, 4},
		{"17", domain.ShapeRect, 10, 78, 16, 10, 6}, {"18", domain.ShapeRect, 60, 78, 16, 10, 6},
	}},
}

// Reset empties every application table. Callers guard it from production.
func Reset(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `truncate push_deliveries, hookah_transfers, service_events, hookahs,
		join_attempts, auth_attempts, staff_sessions, staff, tables, halls, venues, accounts restart identity cascade`)
	if err != nil {
		return fmt.Errorf("reset: %w", err)
	}
	return nil
}

// Seed inserts the fixture into an empty database.
func Seed(ctx context.Context, pool *pgxpool.Pool) (Fixture, error) {
	var fx Fixture
	err := store.WithTx(ctx, pool, func(tx pgx.Tx) error {
		var err error
		fx, err = seed(ctx, tx, domain.DBTime(time.Now()))
		return err
	})
	if err != nil {
		return Fixture{}, fmt.Errorf("seed: %w", err)
	}
	return fx, nil
}

func seed(ctx context.Context, tx pgx.Tx, now time.Time) (Fixture, error) {
	fx := Fixture{TableIDs: map[string]string{}}
	plan := domain.DefaultPlan()
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return fx, err
	}

	// A public fixture, so a cheap hash is fine. The first login upgrades it.
	hash, err := auth.HashPassword(OwnerPassword, bcrypt.MinCost)
	if err != nil {
		return fx, err
	}
	var accountID string
	if err := tx.QueryRow(ctx,
		`insert into accounts (email, password_hash, display_name) values ($1, $2, $3) returning id`,
		OwnerEmail, hash, OwnerName).Scan(&accountID); err != nil {
		return fx, fmt.Errorf("account: %w", err)
	}
	if err := tx.QueryRow(ctx,
		`insert into venues (owner_id, name, join_code, service_plan) values ($1, $2, $3, $4) returning id`,
		accountID, "Кальянная на Абая", JoinCode, planJSON).Scan(&fx.VenueID); err != nil {
		return fx, fmt.Errorf("venue: %w", err)
	}

	addMember := func(name string, role domain.Role, account *string, device int) (Member, error) {
		m := Member{Name: name, Role: role}
		if err := tx.QueryRow(ctx,
			`insert into staff (venue_id, account_id, display_name, role) values ($1, $2, $3, $4) returning id`,
			fx.VenueID, account, name, role).Scan(&m.StaffID); err != nil {
			return m, fmt.Errorf("staff %s: %w", name, err)
		}
		_, refreshHash, err := auth.NewRefreshToken()
		if err != nil {
			return m, err
		}
		if err := tx.QueryRow(ctx,
			`insert into staff_sessions (staff_id, device_id, platform, refresh_token_hash)
			 values ($1, $2, 'android', $3) returning id`,
			m.StaffID, fmt.Sprintf("seed-device-%02d", device), refreshHash).Scan(&m.SessionID); err != nil {
			return m, fmt.Errorf("session %s: %w", name, err)
		}
		return m, nil
	}
	if fx.Owner, err = addMember(OwnerName, domain.RoleOwner, &accountID, 0); err != nil {
		return fx, err
	}
	for i, name := range MasterNames {
		m, err := addMember(name, domain.RoleMaster, nil, i+1)
		if err != nil {
			return fx, err
		}
		fx.Masters = append(fx.Masters, m)
	}

	hallOf := map[string]string{}
	for i, h := range halls {
		var hallID string
		if err := tx.QueryRow(ctx,
			`insert into halls (venue_id, name, sort_order) values ($1, $2, $3) returning id`,
			fx.VenueID, h.name, i).Scan(&hallID); err != nil {
			return fx, fmt.Errorf("hall %s: %w", h.name, err)
		}
		fx.HallIDs = append(fx.HallIDs, hallID)
		for _, t := range h.tables {
			var id string
			if err := tx.QueryRow(ctx,
				`insert into tables (hall_id, label, shape, x, y, w, h, seats) values ($1, $2, $3, $4, $5, $6, $7, $8) returning id`,
				hallID, t.label, t.shape, t.x, t.y, t.w, t.h, t.seats).Scan(&id); err != nil {
				return fx, fmt.Errorf("table %s: %w", t.label, err)
			}
			fx.TableIDs[t.label] = id
			hallOf[t.label] = hallID
		}
	}

	erlan, aidos := fx.Masters[0], fx.Masters[1]
	fx.FreshHookahID, err = addHookah(ctx, tx, fx.VenueID, hallOf["3"], fx.TableIDs["3"], erlan.StaffID, plan, planJSON, now.Add(-3*time.Minute), nil)
	if err != nil {
		return fx, err
	}
	fx.BusyHookahID, err = addHookah(ctx, tx, fx.VenueID, hallOf["15"], fx.TableIDs["15"], aidos.StaffID, plan, planJSON, now.Add(-16*time.Minute),
		func(e domain.Event) eventRow {
			switch e.Seq {
			case 1:
				at := e.DueAt.Add(40 * time.Second)
				return eventRow{state: domain.EventConfirmed, firedAt: &e.DueAt, confirmedBy: &erlan.StaffID, confirmedAt: &at}
			case 2:
				at := e.DueAt.Add(20 * time.Second)
				return eventRow{state: domain.EventDue, firedAt: &e.DueAt, claimedBy: &aidos.StaffID, claimedAt: &at}
			}
			return eventRow{state: domain.EventPending}
		})
	return fx, err
}

type eventRow struct {
	state       domain.EventState
	firedAt     *time.Time
	claimedBy   *string
	claimedAt   *time.Time
	confirmedBy *string
	confirmedAt *time.Time
}

func addHookah(ctx context.Context, tx pgx.Tx, venueID, hallID, tableID, staffID string, plan domain.Plan, planJSON []byte,
	startedAt time.Time, phase func(domain.Event) eventRow) (string, error) {
	events := domain.MaterializePlan(plan, startedAt)
	expiresAt := events[len(events)-1].DueAt
	var id string
	if err := tx.QueryRow(ctx,
		`insert into hookahs (venue_id, hall_id, table_id, state, started_at, expires_at, plan_snapshot, started_by)
		 values ($1, $2, $3, 'active', $4, $5, $6, $7) returning id`,
		venueID, hallID, tableID, startedAt, expiresAt, planJSON, staffID).Scan(&id); err != nil {
		return "", fmt.Errorf("hookah: %w", err)
	}
	for _, e := range events {
		row := eventRow{state: domain.EventPending}
		if phase != nil {
			row = phase(e)
		}
		if _, err := tx.Exec(ctx,
			`insert into service_events (hookah_id, seq, kind, title, due_at, state, fired_at, claimed_by, claimed_at, confirmed_by, confirmed_at)
			 values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			id, e.Seq, e.Kind, e.Title, e.DueAt, row.state, row.firedAt, row.claimedBy, row.claimedAt, row.confirmedBy, row.confirmedAt); err != nil {
			return "", fmt.Errorf("service event %d: %w", e.Seq, err)
		}
	}
	return id, nil
}
