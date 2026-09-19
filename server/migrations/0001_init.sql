-- +goose Up
create extension if not exists pgcrypto;
create extension if not exists citext;

-- Owner and admin accounts
create table accounts (
  id            uuid primary key default gen_random_uuid(),
  email         citext not null unique,
  password_hash text   not null,
  display_name  text   not null,
  created_at    timestamptz not null default now()
);

-- service_plan is the active service plan, tech.md section 7
create table venues (
  id           uuid primary key default gen_random_uuid(),
  owner_id     uuid not null references accounts(id) on delete restrict,
  name         text not null,
  join_code    char(4) not null,
  timezone     text not null default 'Asia/Atyrau',
  service_plan jsonb not null,
  created_at   timestamptz not null default now(),
  updated_at   timestamptz not null default now()
);
create unique index venues_join_code_key on venues (join_code);

create table halls (
  id         uuid primary key default gen_random_uuid(),
  venue_id   uuid not null references venues(id) on delete cascade,
  name       text not null,
  sort_order int  not null default 0,
  created_at timestamptz not null default now(),
  unique (venue_id, name)
);

-- x, y, w, h are layout grid units, tech.md section 7
create table tables (
  id         uuid primary key default gen_random_uuid(),
  hall_id    uuid not null references halls(id) on delete cascade,
  label      text not null,
  shape      text not null check (shape in ('rect','circle')),
  x          int  not null,
  y          int  not null,
  w          int  not null check (w between 1 and 40),
  h          int  not null check (h between 1 and 40),
  rotation   int  not null default 0 check (rotation between 0 and 359),
  seats      int  not null default 4 check (seats between 1 and 30),
  archived_at timestamptz,
  created_at timestamptz not null default now()
);
create unique index tables_hall_label_key on tables (hall_id, label) where archived_at is null;
create index tables_hall_idx on tables (hall_id) where archived_at is null;

-- A master row is created on join by code
create table staff (
  id           uuid primary key default gen_random_uuid(),
  venue_id     uuid not null references venues(id) on delete cascade,
  account_id   uuid references accounts(id) on delete set null,
  display_name text not null,
  role         text not null check (role in ('owner','admin','master')),
  blocked_at   timestamptz,
  created_at   timestamptz not null default now(),
  last_seen_at timestamptz
);
create index staff_venue_idx on staff (venue_id) where blocked_at is null;

-- One staff member may hold several device sessions
create table staff_sessions (
  id                  uuid primary key default gen_random_uuid(),
  staff_id            uuid not null references staff(id) on delete cascade,
  device_id           text not null,
  platform            text not null check (platform in ('ios','android')),
  refresh_token_hash  text not null,
  expo_push_token     text,
  revoked_at          timestamptz,
  created_at          timestamptz not null default now(),
  last_seen_at        timestamptz not null default now(),
  unique (staff_id, device_id)
);
create index staff_sessions_push_idx on staff_sessions (staff_id)
  where revoked_at is null and expo_push_token is not null;

-- Join attempts feed the rate limit
create table join_attempts (
  id         bigserial primary key,
  device_id  text not null,
  code       char(4) not null,
  success    bool not null,
  created_at timestamptz not null default now()
);
create index join_attempts_device_idx on join_attempts (device_id, created_at desc);

-- plan_snapshot freezes the plan at start
create table hookahs (
  id              uuid primary key default gen_random_uuid(),
  venue_id        uuid not null references venues(id) on delete cascade,
  hall_id         uuid not null references halls(id) on delete cascade,
  table_id        uuid not null references tables(id) on delete restrict,
  state           text not null check (state in ('active','expired','finished')),
  started_at      timestamptz not null,
  expires_at      timestamptz not null,
  finished_at     timestamptz,
  finish_reason   text check (finish_reason in ('guest_left','replaced','mistake','shift_end')),
  plan_snapshot   jsonb not null,
  started_by      uuid not null references staff(id) on delete restrict,
  finished_by     uuid references staff(id) on delete set null,
  idempotency_key text,
  created_at      timestamptz not null default now(),
  updated_at      timestamptz not null default now()
);
-- One live hookah per table
create unique index hookahs_active_table_key on hookahs (table_id)
  where state in ('active','expired');
create unique index hookahs_idempotency_key on hookahs (venue_id, idempotency_key)
  where idempotency_key is not null;
create index hookahs_hall_live_idx on hookahs (hall_id) where state in ('active','expired');

-- All service events are materialized when the hookah starts
create table service_events (
  id            uuid primary key default gen_random_uuid(),
  hookah_id     uuid not null references hookahs(id) on delete cascade,
  seq           int  not null,
  kind          text not null check (kind in ('check','shake','replace','expire','custom')),
  title         text not null,
  due_at        timestamptz not null,
  state         text not null check (state in ('pending','due','confirmed','missed','cancelled')),
  fired_at      timestamptz,
  claimed_by    uuid references staff(id) on delete set null,
  claimed_at    timestamptz,
  confirmed_by  uuid references staff(id) on delete set null,
  confirmed_at  timestamptz,
  confirmed_early bool not null default false,
  push_attempts int not null default 0,
  last_push_at  timestamptz,
  unique (hookah_id, seq)
);
create index service_events_pending_idx on service_events (due_at) where state = 'pending';
create index service_events_due_idx on service_events (state, due_at) where state = 'due';
create index service_events_hookah_idx on service_events (hookah_id, seq);

create table hookah_transfers (
  id            uuid primary key default gen_random_uuid(),
  hookah_id     uuid not null references hookahs(id) on delete cascade,
  from_table_id uuid not null references tables(id) on delete restrict,
  to_table_id   uuid not null references tables(id) on delete restrict,
  moved_by      uuid not null references staff(id) on delete restrict,
  moved_at      timestamptz not null default now()
);

-- Push dedup and delivery audit
create table push_deliveries (
  id               uuid primary key default gen_random_uuid(),
  service_event_id uuid not null references service_events(id) on delete cascade,
  session_id       uuid not null references staff_sessions(id) on delete cascade,
  attempt          int  not null,
  status           text not null check (status in ('queued','sent','failed','invalid_token')),
  provider_receipt text,
  error            text,
  created_at       timestamptz not null default now(),
  unique (service_event_id, session_id, attempt)
);
create index push_deliveries_queued_idx on push_deliveries (created_at) where status = 'queued';

-- +goose Down
-- Forward-only migrations, see tech.md section 3.
