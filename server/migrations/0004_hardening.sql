-- +goose Up
-- Server hardening of tech.md section 19.16.

-- Six digit join codes. Existing codes keep their four digits and gain two
-- random ones, so masters who know the old code recognise the new one and the
-- unique index cannot collide.
alter table venues
  alter column join_code type char(6)
  using (join_code || lpad((floor(random() * 100))::int::text, 2, '0'));

alter table join_attempts alter column code type char(6);

-- Per ip limits count attempts by address. Rows written before this migration
-- have no address and never match a limit.
alter table join_attempts add column ip inet;
create index join_attempts_ip_idx on join_attempts (ip, created_at desc);

-- A queued push whose event was closed is cancelled, not sent.
alter table push_deliveries drop constraint push_deliveries_status_check;
alter table push_deliveries add constraint push_deliveries_status_check
  check (status in ('queued','sent','failed','invalid_token','cancelled'));

-- +goose Down
-- Forward-only migrations, see tech.md section 3.
