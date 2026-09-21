-- +goose Up
-- Attempts of POST /auth/login and POST /auth/register, tech.md section 19.16.
create table auth_attempts (
  id         bigserial primary key,
  kind       text not null check (kind in ('login','register')),
  ip         inet,
  email      citext,
  success    bool not null default false,
  created_at timestamptz not null default now()
);
create index auth_attempts_ip_idx on auth_attempts (kind, ip, created_at desc);
create index auth_attempts_email_idx on auth_attempts (email, created_at desc) where kind = 'login';

-- +goose Down
-- Forward-only migrations, see tech.md section 3.
