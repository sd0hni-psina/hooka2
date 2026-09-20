-- +goose Up
-- Transport retries of tech.md section 19.9.
alter table push_deliveries
  add column retries  int not null default 0,
  add column retry_at timestamptz;
create index push_deliveries_retry_idx on push_deliveries (retry_at) where status = 'queued';

-- +goose Down
-- Forward-only migrations, see tech.md section 3.
