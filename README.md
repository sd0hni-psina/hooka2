# Кальянный таймер

Мобильное приложение для кальянных мастеров: общий синхронизированный таймер на каждый кальян, задачи сервиса, пуши. Клиент на Expo и React Native, сервер на Go и Postgres.

Контракты, схема БД и правила работы живут в [`tech.md`](tech.md). Правила разработки в [`CLAUDE.md`](CLAUDE.md). План работ до релиза в [`tz.md`](tz.md), аудит сервера и логики в [`PLAN.md`](PLAN.md), разбор клиента в [`REVIEW.md`](REVIEW.md).

## Что нужно на машине

- Go (версия из `server/go.mod`), Node 20 и новее, Docker Desktop.
- Телефон с Expo Go для быстрой проверки или dev-сборка для пушей.

## Запуск локально

Порт Postgres по умолчанию 5432. Если он занят другим проектом, задай `POSTGRES_PORT` в каждой команде (у автора это 5433).

```bash
export POSTGRES_PORT=5433
make up      # postgres в docker
make seed    # миграции и сид: заведение, 2 зала, 18 столов, 3 мастера, 2 живых кальяна
make api     # API на :8080, провайдер пушей fake
```

Сид создаёт заведение с кодом входа `1234`. Владелец входит как `owner@hookah.dev` с паролем `hookah-dev-pass`. `cmd/seed` отказывается работать при `APP_ENV=prod`.

Клиент:

```bash
cd app
npm ci
npx expo start
```

`app/.env` оставь с пустым `EXPO_PUBLIC_API_URL`: клиент берёт адрес API из хоста Metro и порта 8080 (`tech.md` 19.12). Телефон и ноутбук должны быть в одной сети без изоляции клиентов.

## Проверки

```bash
make check      # golangci-lint (docker), go vet, go test -race, tsc, eslint, jest
```

Серверные тесты читают `TEST_DATABASE_URL`. Без него 81 интеграционный тест пропускается молча, а `go test` печатает `ok`, поэтому зелёный прогон без базы почти ничего не доказывает:

```bash
export TEST_DATABASE_URL="postgres://hookah:hookah@localhost:5433/hookah_test?sslmode=disable"
```

## Структура

| Путь | Что там |
|---|---|
| `server/` | Go API: `cmd/` (api, migrate, seed, devtoken), `internal/` (domain, слои handler, service, repository), `migrations/` |
| `app/` | Expo: `app/` (роуты), `src/` (api, domain, store, ws, ui, features) |
| `contracts/fixtures/` | общие фикстуры для Go и Jest |
| `deploy/` | prod compose, Caddy, шаблон `.env` |
| `.github/workflows/` | `ci.yml`, `deploy.yml` |

## Ветки и коммиты

Ветка на слайс: `slice/NN-name`, в `main` через PR. Коммиты по формату `type(scope): summary` на английском, подробности в `CLAUDE.md` раздел 8.
