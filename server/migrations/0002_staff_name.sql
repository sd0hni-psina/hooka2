-- +goose Up
-- Two joins with one name must land on one master, tech.md section 19.1.
create unique index staff_venue_name_key on staff (venue_id, lower(display_name))
  where account_id is null;

-- +goose Down
-- Forward-only migrations, see tech.md section 3.
