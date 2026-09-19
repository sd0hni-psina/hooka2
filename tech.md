# tech.md — Кальянный таймер

**Версия ядра: v7** (2026-09-19)

Changelog:

- v1 — первая версия. Стек, схема БД, доменные правила, HTTP и WS контракты, UI-кит, стратегия тестов, дорожная карта из 6 слайсов.
- v2 — раздел 19.1: типы `Venue`, `Hall`, `Staff`, `Tokens`, `Me`; `device_id` и `platform` в register и login; правила валидации входа; поиск мастера по имени и миграция `0002`; `seq` и `hall_id` в прямых ответах WS; ответ `/healthz`; зависимость `golang.org/x/crypto`; команды `cmd/migrate`, `cmd/seed`, `cmd/devtoken`; папка `/deploy`; `TEST_DATABASE_URL`.
- v3 — раздел 19.2: зависимости клиента на Expo SDK 57, переменные `EXPO_PUBLIC_*`, раскладка роутов, доставка событий хабом всем подпискам заведения, пакет `storetest`, UTC в ответах.
- v4 — раздел 19.3: примитив `Icon`, дополнительные пропсы `Text`, `HallCanvas`, семантика `Countdown`, ресинк по `subscribe`, файлы клиента `decode.ts`, `session.ts`, `auth.ts`.
- v5 — раздел 19.4: подсчёт попыток входа и `retry_after_sec`, срок refresh по `last_seen_at`, формат push-токена, роли ротации кода, вход владельца на экране `join.tsx`, `domain/join.go`.
- v6 — раздел 19.5: проп `Input.secure`, фикстура `me.json`, пакет `apitest`, клиентские `device.ts`, `features/join/form.ts`, `registerPush.ts`.
- v7 — раздел 19.6: валидация залов и столов, восстановление стола через `PATCH archived`, удаление зала, правила публикации `layout.updated`, `domain/layout.go`, `src/domain/layout.ts`.

Правила работы с этим файлом:

- Файл — единственный источник истины по контрактам: схема БД, формат эндпоинтов, payload WS-событий, общие типы, перечень UI-примитивов, структура папок, конвенции.
- Правки только append-only. Любое изменение контракта (поле, тип, эндпоинт, событие, таблица) бампает версию ядра и добавляет строку в changelog.
- Сессия, которой не хватает контракта, выдаёт блок `CONTRACT GAP` и не пишет код с выдуманным типом. Протокол описан в `CLAUDE.md`.

Допущение, которое ты можешь отменить: мобильный стек не был зафиксирован, я взял Expo + React Native + TypeScript (один кодовый базис на iOS и Android, готовый пуш-слой, привычный тебе JS). Если выбираешь Flutter, скажи — тогда меняются разделы 3, 5, 11, 12 и клиентская часть слайсов; серверная половина документа остаётся как есть.

---

## 1. Проект

Мобильное приложение для кальянных мастеров в заведениях с большой посадкой. Заменяет память мастера общим синхронизированным таймером на каждый кальян.

Проблема: при 20+ занятых столах мастер забывает проверить кальян, стряхнуть или заменить угли. Когда на смене несколько мастеров, один поставил кальян, а второй не знает, когда этому столу нужен следующий сервис.

Пользователи:

- **Мастер** — работает в зале, ставит кальяны, получает уведомления, подтверждает действия. Держит телефон одной рукой, смотрит в экран по 3 секунды, в полутьме.
- **Администратор** — рисует планировку зала, правит интервалы сервиса, управляет персоналом.
- **Владелец** — то же плюс создание заведения и залов, ротация кода входа.

Цель v1: мастер за несколько секунд с главного экрана понимает, какие кальяны работают, сколько они работают, какой стол требует внимания, какое действие нужно и кто уже взял задачу.

Границы v1. Не делаем: оплаты, склад табака, интеграции с POS, чаты, аналитику и отчёты по смене, веб-панель администратора (планировка редактируется в приложении), офлайн-запуск кальяна без сети (мутации требуют сети, чтение работает из кэша).

---

## 2. Сценарии

**Вход мастера.** Открыл приложение, ввёл 4-значный код заведения и своё имя, выбрал зал. Видит схему зала со столами.

**Запуск кальяна.** Поставил кальян на стол 15, тапнул по столу 15 в приложении, подтвердил. Стол стал зелёным, на нём пошёл счётчик времени жизни.

**Сервис по интервалам.** На 5 минуте приходит уведомление «Стол 15. Проверьте кальян и узнайте у гостя, всё ли хорошо». Стол жёлтый. Мастер тапнул «Беру» — второй мастер видит, что задачу взяли. Подошёл, проверил, нажал подтверждение. Стол снова зелёный, таймер кальяна продолжает идти с той же отметки. То же на 15, 30 минуте и дальше по плану заведения.

**Синхронизация.** Мастер 1 запустил кальян на столе 15. У мастера 2 этот кальян появился сразу. Через 15 минут оба получили уведомление. Мастер 1 подтвердил — у мастера 2 задача отмечена выполненной без его действий.

**Перенос.** Гость пересел со стола 15 на стол 22 на 20 минуте. Мастер: стол 15 → Перенести → стол 22. Кальян продолжает с 20 минуты, все будущие интервалы сохраняют свои абсолютные дедлайны.

**Исчерпание ресурса.** На 1:20 стол становится красным для всех мастеров. Кальян не завершается сам, красный держится, пока мастер не завершит его или не заменит.

**Завершение.** Гость ушёл, мастер нажал «Завершить кальян». Таймер остановлен, будущие задачи отменены, стол свободен.

---

## 3. Стек

Сервер:

- Go 1.22+, `net/http` + `http.ServeMux` из стандартной библиотеки (роутинг с методами и путевыми параметрами), без веб-фреймворка.
- PostgreSQL 16, драйвер `github.com/jackc/pgx/v5` + `pgxpool`. SQL руками в репозиториях, без ORM.
- Миграции: `github.com/pressly/goose/v3`, только вперёд, применяются в деплой-шаге.
- WebSocket: `github.com/coder/websocket`.
- JWT: `github.com/golang-jwt/jwt/v5`.
- Валидация: руками в слое сервиса, отдельной библиотеки нет.
- Логи: `log/slog`, JSON-хендлер, уровень из конфига.
- Тесты: `testing` (table-driven), `github.com/stretchr/testify/require` для ассертов, `pgtestdb` не используем — интеграционные тесты идут против эфемерного Postgres из CI-сервиса и локального docker-compose по `TEST_DATABASE_URL`.
- Property-based: `pgregory.net/rapid` на чистой доменной логике.

Клиент:

- Expo SDK 52+, React Native, TypeScript в strict.
- Навигация: `expo-router`.
- Состояние: `zustand` (один стор на зал) + свой WS-клиент. TanStack Query не берём: живое состояние приходит по WS, REST используется точечно.
- Схема зала: `react-native-svg`, жесты — `react-native-gesture-handler` + `react-native-reanimated`.
- Уведомления: `expo-notifications` (получение и категории с кнопками), токены — Expo Push (FCM + APNs под капотом).
- Хранение токенов: `expo-secure-store`.
- Тесты: `jest` + `@testing-library/react-native`, чистая логика — обычные unit-тесты.

Инфраструктура:

- Docker + docker-compose для локальной разработки (postgres + api).
- GitHub Actions: гейт на PR, деплой на мёрдж в main.
- Тестовый VPS: docker-compose, Caddy как reverse-proxy с TLS, автодеплой из main.
- Сборки приложения: EAS Build, каналы `preview` и `production`.

Запрет: новая зависимость добавляется только после записи в этот файл с бампом версии ядра.

---

## 4. Архитектура

Монорепо, два приложения и общие контракты:

```
/server     Go API, WS-хаб, планировщик, отправка пушей
/app        Expo-приложение
/contracts  golden-фикстуры контрактов, общие для Go- и JS-тестов
```

Сервер — слоистый, зависимости направлены внутрь:

```
handler (HTTP/WS)  →  service (сценарии, транзакции)  →  repository (SQL)
                              ↓
                        domain (чистые правила, без БД и HTTP)
```

- **handler** парсит запрос, зовёт сервис, сериализует ответ и ошибку. Не знает про SQL и про бизнес-правила.
- **service** держит сценарий целиком: валидация входа, транзакция, вызовы репозиториев, вычисление доменных решений, публикация событий в хаб и постановка пушей.
- **repository** — только SQL и маппинг в структуры. Никаких бизнес-правил, никаких ветвлений по ролям.
- **domain** — чистые функции и типы: материализация плана сервиса, вычисление статуса стола, инварианты переходов состояний. Без импортов `pgx`, `net/http`, времени из `time.Now()` (время приходит аргументом).

Три фоновых компонента:

- **realtime.Hub** — реестр подключений, комнаты по `hall_id`, монотонный `seq` на заведение, рассылка событий. Интерфейс `Broadcaster` — шов: в v1 хаб в процессе, горизонтальное масштабирование потребует Postgres LISTEN/NOTIFY за тем же интерфейсом.
- **scheduler** — тикер 1 секунда: переводит созревшие `service_events` из `pending` в `due`, помечает просроченные как `missed`, переводит кальян в `expired`, публикует события в хаб и ставит пуши.
- **push.Sender** — интерфейс с двумя реализациями: `expo` (реальный Expo Push API) и `fake` (пишет в `push_deliveries` и в лог). Разработка идёт против `fake` с первого дня, реальные ключи не блокируют ничего.

Клиент — вертикальные фичи вокруг общего ядра:

```
src/api      REST-клиент, типы запросов и ответов
src/ws       WS-клиент, реконнект, разбор конверта, синхронизация времени
src/store    zustand-стор зала, редьюсер WS-событий
src/domain   чистая логика: статус стола, форматирование таймера, план сервиса
src/ui       примитивы и токены
src/features одна папка на фичу
```

---

## 5. Структура папок

```
/server
  cmd/api/main.go                 сборка зависимостей, graceful shutdown
  internal/config/config.go       чтение env, дефолты, валидация
  internal/httpx/                 router.go, middleware.go, errors.go, respond.go
  internal/auth/                  jwt.go, password.go, context.go, guard.go
  internal/domain/                plan.go, status.go, state.go, errors.go, time.go
  internal/account/               service.go, repository.go, handler.go
  internal/venue/                 service.go, repository.go, handler.go
  internal/layout/                service.go, repository.go, handler.go
  internal/staff/                 service.go, repository.go, handler.go
  internal/hookah/                service.go, repository.go, handler.go
  internal/realtime/              hub.go, client.go, envelope.go, handler.go
  internal/scheduler/             scheduler.go
  internal/push/                  sender.go, expo.go, fake.go, worker.go
  internal/store/                 pool.go, tx.go
  migrations/                     0001_init.sql, ...
  seed/seed.go                    общие фикстуры для разработки и тестов
  testdata/                       golden JSON для контрактных тестов
/app
  app/                            expo-router: _layout.tsx, join.tsx, (hall)/index.tsx, (hall)/table/[id].tsx, settings/*
  src/api/                        client.ts, endpoints.ts, types.ts, errors.ts
  src/ws/                         client.ts, envelope.ts, reconnect.ts, clock.ts
  src/store/                      hall.ts, reducer.ts, selectors.ts
  src/domain/                     status.ts, plan.ts, countdown.ts
  src/ui/                         tokens.ts, Button.tsx, ...
  src/features/hookah/            компоненты и хуки фичи
  src/features/layout-editor/
  src/features/notifications/
/contracts
  fixtures/plan_cases.json        вход плана и ожидаемые события
  fixtures/status_cases.json      состояние кальяна и ожидаемый цвет стола
  fixtures/ws_events.json         примеры конвертов каждого типа
docker-compose.yml
Makefile
.env.example
tech.md
CLAUDE.md
```

Файлы в `/contracts/fixtures` читают и Go-тесты, и Jest-тесты. Один набор кейсов держит серверную и клиентскую реализацию доменных правил в согласии.

---

## 6. Схема БД

Postgres 16. Все `id` — `uuid` (`gen_random_uuid()` из `pgcrypto`). Все временные поля — `timestamptz`, хранятся в UTC. Денежных полей нет.

```sql
create extension if not exists pgcrypto;
create extension if not exists citext;

-- Аккаунты владельцев и администраторов
create table accounts (
  id            uuid primary key default gen_random_uuid(),
  email         citext not null unique,
  password_hash text   not null,
  display_name  text   not null,
  created_at    timestamptz not null default now()
);

-- Заведение. service_plan — активный план сервиса, см. раздел 7
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

-- Столы. x,y,w,h в условных единицах планировки (см. раздел 7, «Система координат»)
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

-- Персонал заведения. Мастер создаётся при входе по коду
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

-- Сессия устройства. Один staff может держать несколько устройств
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

-- Попытки входа по коду, для rate limit
create table join_attempts (
  id         bigserial primary key,
  device_id  text not null,
  code       char(4) not null,
  success    bool not null,
  created_at timestamptz not null default now()
);
create index join_attempts_device_idx on join_attempts (device_id, created_at desc);

-- Кальян. plan_snapshot фиксирует план на момент старта
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
-- Один живой кальян на стол
create unique index hookahs_active_table_key on hookahs (table_id)
  where state in ('active','expired');
create unique index hookahs_idempotency_key on hookahs (venue_id, idempotency_key)
  where idempotency_key is not null;
create index hookahs_hall_live_idx on hookahs (hall_id) where state in ('active','expired');

-- Задачи сервиса. Материализуются целиком при старте кальяна
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

-- Дедупликация и аудит отправки пушей
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
```

Владение схемой: миграции пишет только ведущая сессия скелета, файл миграции после применения на тестовом VPS не редактируется. Новое изменение — новый файл.

---

## 7. Доменные правила и инварианты

### План сервиса

`venues.service_plan` и `hookahs.plan_snapshot` держат один JSON:

```json
{
  "version": 1,
  "steps": [
    { "kind": "check",   "title": "Проверьте кальян и узнайте у гостя, всё ли хорошо", "offset_sec": 300 },
    { "kind": "shake",   "title": "Стряхните угли",  "offset_sec": 900,  "repeat_every_sec": 900 },
    { "kind": "replace", "title": "Замените угли",   "offset_sec": 1800, "repeat_every_sec": 1800 },
    { "kind": "expire",  "title": "Кальян отработал ресурс", "offset_sec": 4800 }
  ],
  "warn_before_sec": 60,
  "grace_sec": 120,
  "missed_after_sec": 600,
  "push_repeat_sec": 120,
  "push_max_attempts": 3
}
```

Ограничения плана, проверяет сервис при сохранении:

- Ровно один шаг с `kind = "expire"`, у него нет `repeat_every_sec`, его `offset_sec` строго больше всех остальных.
- `offset_sec` от 60 до 86400, `repeat_every_sec` от 120 до 86400 либо отсутствует.
- Шагов от 1 до 12. `title` до 120 символов.
- `grace_sec` от 30 до 900, `warn_before_sec` от 0 до 300, `missed_after_sec` больше `grace_sec`.

### Материализация плана

Чистая функция `domain.MaterializePlan(plan Plan, startedAt time.Time) []Event`. Правила:

1. Каждый шаг даёт события на `startedAt + offset_sec`, дальше через `repeat_every_sec`, пока `due_at` строго меньше момента `expire`.
2. Событие `expire` — последнее, всегда ровно одно.
3. Если два события попадают в одну секунду, остаётся одно, с приоритетом `expire > replace > shake > check > custom`.
4. События сортируются по `due_at`, затем по приоритету. `seq` присваивается с 1 по порядку.
5. Функция детерминирована: тот же вход даёт тот же выход, включая `seq`.

Кейсы и ожидаемые выходы лежат в `/contracts/fixtures/plan_cases.json`. Go-тест и Jest-тест читают один файл.

### Статус стола

Чистая функция, одинаковая на сервере (`domain.TableStatus`) и на клиенте (`src/domain/status.ts`). Вход: кальян, его события, `now`, поля плана. Выход: `free | green | yellow | red`.

Порядок проверок, первый сработавший выигрывает:

1. Кальяна нет или `state = finished` → `free`.
2. Существует событие в `due` с `now >= due_at + grace_sec` → `red`.
3. `hookah.state = expired` → `red`.
4. Существует событие в `due` → `yellow`.
5. Существует событие в `pending` с `due_at - now <= warn_before_sec` → `yellow`.
6. Иначе → `green`.

`claimed_by` цвет не меняет, добавляет бейдж с именем мастера. Кейсы — `/contracts/fixtures/status_cases.json`.

Сервер кладёт вычисленный `status` в снапшот и в события. Клиент пересчитывает статус локально между сообщениями (дедлайн проходит без входящего события) и при следующем сообщении берёт серверный. Расхождение двух реализаций ловит общая фикстура.

### Переходы состояний

Кальян:

```
active ──expire fired──> expired ──finish──> finished
   └────────finish──────────────────────────> finished
```

Событие:

```
pending ──due_at достигнут──> due ──confirm──> confirmed
   │                           └──missed_after_sec──> missed
   ├──confirm раньше срока──> confirmed (confirmed_early = true)
   └──кальян завершён──> cancelled
```

Инварианты, которые держит сервис в одной транзакции:

- **Один живой кальян на стол.** Обеспечивает partial unique index, сервис ловит нарушение и отдаёт `table_occupied`.
- **Подтверждение не сдвигает таймер.** `due_at` будущих событий и `expires_at` кальяна неизменны после старта. Подтверждение пишет только `confirmed_at` и `confirmed_by`.
- **Перенос сохраняет время.** Меняется `table_id` и `hall_id`, запись в `hookah_transfers`. `started_at`, `expires_at`, все `due_at` остаются.
- **Завершение отменяет будущее.** Все события в `pending` уходят в `cancelled`, события в `due` остаются как есть для истории, очередь пушей по ним останавливается.
- **Раннее подтверждение разрешено** только для ближайшего события каждого `kind` в состоянии `pending`. Флаг `confirmed_early = true`.
- **Смена плана заведения не влияет на живые кальяны.** Новый план применяется к кальянам, запущенным после сохранения. Живые продолжают по своему `plan_snapshot`.
- **Стол с живым кальяном нельзя удалить.** Архивация стола отдаёт `table_busy`.
- **Гонка подтверждения.** Два мастера жмут подтверждение одновременно: обновление идёт с условием `state = 'due' and confirmed_at is null`, проигравший получает 200 и текущее состояние события, не 409. Действие идемпотентно с точки зрения мастера.

### Планировщик

Тикер 1 секунда, каждый тик три запроса, каждый идемпотентен по состоянию:

```sql
-- 1. Созревшие события
update service_events set state = 'due', fired_at = now()
where id in (
  select e.id from service_events e
  join hookahs h on h.id = e.hookah_id
  where e.state = 'pending' and e.due_at <= now() and h.state != 'finished'
  order by e.due_at
  for update skip locked
  limit 200
)
returning *;

-- 2. Просроченные
update service_events set state = 'missed'
where state = 'due' and due_at + (:missed_after_sec * interval '1 second') <= now()
returning *;

-- 3. Исчерпавшие ресурс кальяны
update hookahs set state = 'expired', updated_at = now()
where state = 'active' and expires_at <= now()
returning *;
```

Переход состояния служит защитой от повторной обработки: второй прогон того же тика не найдёт строк. Тест идемпотентности обязателен для каждого из трёх запросов.

После каждого запроса сервис публикует WS-события и вставляет строки `push_deliveries` со `status = 'queued'`. Отправку делает отдельный воркер (тикер 5 секунд) через `push.Sender`.

### Время

Сервер — единственный источник времени. `now` в домен передаётся аргументом, `time.Now()` живёт только в слое сервиса и планировщика.

Клиент при подключении получает `server_time`, считает `offset = server_time - Date.now()` и дальше использует только `serverNow()`. Счётчики и цвета считаются от `serverNow()`. Прямой `Date.now()` в отсчётах запрещён.

### Код входа

4 цифры, уникальны среди всех заведений. Риск подбора закрывается тремя мерами: rate limit 5 попыток в минуту и 20 в час на `device_id` (таблица `join_attempts`), вход создаёт запись `staff` с именем и требует подтверждения администратором для доступа к настройкам, владелец может сменить код одной кнопкой (все активные сессии остаются, новые входы идут по новому коду).

### Система координат планировки

Зал — сетка 100×100 условных единиц, начало в левом верхнем углу, шаг привязки 1 единица. Стол задаётся `x, y` (левый верхний угол), `w, h`, `rotation` в градусах. Клиент масштабирует сетку под экран, сохраняя пропорцию, и разрешает pan и zoom. Пересечение столов сервер не запрещает, редактор подсвечивает его как предупреждение.

---

## 8. HTTP API

База: `/api/v1`. Формат — JSON, `snake_case` в полях. Авторизация — заголовок `Authorization: Bearer <access_token>`.

Токены: access — JWT на 30 минут с клеймами `sub` (staff_id), `venue_id`, `role`, `session_id`. Refresh — opaque строка 32 байта, хэш в `staff_sessions.refresh_token_hash`, срок 90 дней, ротация при каждом обновлении.

### Ошибки

Единый формат, HTTP-код плюс машинный код:

```json
{ "error": { "code": "table_occupied", "message": "На этом столе уже есть активный кальян", "details": { "table_id": "..." } } }
```

Закрытый список кодов:

| code | HTTP | когда |
|---|---|---|
| `validation_failed` | 400 | тело или параметры не прошли валидацию, `details.fields` |
| `unauthorized` | 401 | нет или истёк access-токен |
| `forbidden` | 403 | роль не позволяет действие |
| `not_found` | 404 | объект не найден или не принадлежит заведению |
| `invalid_join_code` | 404 | кода нет среди активных заведений |
| `rate_limited` | 429 | превышен лимит попыток входа, `details.retry_after_sec` |
| `table_occupied` | 409 | на столе живой кальян |
| `table_busy` | 409 | попытка архивировать стол с живым кальяном |
| `hookah_finished` | 409 | действие над завершённым кальяном |
| `event_not_actionable` | 409 | подтверждение события не в `due` и не ближайшего `pending` |
| `plan_invalid` | 422 | план сервиса нарушает ограничения раздела 7, `details.reason` |
| `internal` | 500 | всё остальное |

### Эндпоинты

Аккаунты и заведение:

```
POST   /auth/register            {email, password, display_name, venue_name} → {tokens, venue}
POST   /auth/login               {email, password} → {tokens, venue}
POST   /auth/refresh             {refresh_token} → {tokens}
POST   /auth/logout              → 204
```

Вход мастера:

```
POST   /join                     {code, display_name, device_id, platform} → {tokens, venue, halls}
POST   /sessions/push-token      {expo_push_token} → 204
DELETE /sessions/current         → 204
```

Бутстрап и снапшот:

```
GET    /me                       → {staff, venue, halls}
GET    /halls/{hall_id}/state    → HallState (см. раздел 9, тот же тип, что в state.snapshot)
```

Планировка (роли owner, admin):

```
POST   /halls                    {name, sort_order} → Hall
PATCH  /halls/{hall_id}          {name?, sort_order?} → Hall
DELETE /halls/{hall_id}          → 204
POST   /halls/{hall_id}/tables   {label, shape, x, y, w, h, rotation, seats} → Table
PATCH  /tables/{table_id}        частичное обновление → Table
DELETE /tables/{table_id}        архивация → 204
PUT    /halls/{hall_id}/layout   {tables: [{id, x, y, w, h, rotation}]} → [Table]   батч после перетаскивания
```

Настройки (роли owner, admin):

```
GET    /venues/current/service-plan       → Plan
PUT    /venues/current/service-plan       Plan → Plan
POST   /venues/current/rotate-code        → {join_code}
GET    /venues/current/staff              → [Staff]
PATCH  /staff/{staff_id}                  {role?, blocked?} → Staff
```

Кальяны (все роли):

```
POST   /tables/{table_id}/hookah      заголовок Idempotency-Key обязателен → Hookah
POST   /hookahs/{hookah_id}/transfer  {to_table_id} → Hookah
POST   /hookahs/{hookah_id}/finish    {reason} → Hookah
POST   /service-events/{event_id}/claim    → ServiceEvent
POST   /service-events/{event_id}/confirm  → ServiceEvent
```

`Idempotency-Key` — UUID, генерирует клиент перед запросом и переиспользует при ретраях. Повтор с тем же ключом отдаёт 200 и тот же кальян, не создаёт второй.

---

## 9. WebSocket

Адрес: `GET /api/v1/ws?access_token=<jwt>`. Токен в query, потому что RN не даёт ставить заголовки на WS.

Конверт, один формат на все сообщения:

```json
{
  "v": 1,
  "type": "service.due",
  "seq": 1487,
  "ts": "2026-09-19T18:04:05.123Z",
  "venue_id": "uuid",
  "hall_id": "uuid",
  "data": { }
}
```

- `seq` монотонно растёт в рамках заведения. Клиент видит разрыв — запрашивает снапшот через `GET /halls/{id}/state` и переинициализирует стор.
- `ts` — серверное время отправки, клиент обновляет по нему свой offset.

Клиент → сервер:

| type | data | эффект |
|---|---|---|
| `subscribe` | `{hall_id}` | подписка на комнату зала, ответ — `state.snapshot` |
| `ping` | `{}` | ответ `pong` с `server_time` |

Сервер → клиент:

| type | data |
|---|---|
| `state.snapshot` | `HallState` целиком |
| `hookah.started` | `{hookah: Hookah, events: ServiceEvent[], status: TableStatus}` |
| `hookah.transferred` | `{hookah_id, from_table_id, to_table_id, moved_by, status}` |
| `hookah.expired` | `{hookah_id, table_id, status}` |
| `hookah.finished` | `{hookah_id, table_id, reason, finished_by}` |
| `service.due` | `{event: ServiceEvent, hookah_id, table_id, table_label, status}` |
| `service.claimed` | `{event_id, hookah_id, claimed_by, claimed_by_name, claimed_at}` |
| `service.confirmed` | `{event_id, hookah_id, confirmed_by, confirmed_by_name, confirmed_at, confirmed_early, status}` |
| `service.missed` | `{event_id, hookah_id, table_id, status}` |
| `plan.updated` | `{plan: Plan}` |
| `layout.updated` | `{hall_id, tables: Table[]}` |
| `pong` | `{server_time}` |
| `error` | `{code, message}` |

`HallState`:

```json
{
  "server_time": "2026-09-19T18:04:05.123Z",
  "hall": { "id": "...", "name": "Второй зал" },
  "plan": { },
  "tables": [ { "id": "...", "label": "15", "shape": "rect", "x": 20, "y": 30, "w": 8, "h": 6, "rotation": 0, "seats": 4 } ],
  "hookahs": [
    {
      "id": "...", "table_id": "...", "state": "active",
      "started_at": "...", "expires_at": "...",
      "status": "yellow",
      "events": [ { "id": "...", "seq": 2, "kind": "shake", "title": "Стряхните угли", "due_at": "...", "state": "due", "claimed_by": "...", "claimed_by_name": "Ерлан", "confirmed_at": null } ]
    }
  ],
  "staff_online": [ { "id": "...", "display_name": "Ерлан" } ]
}
```

Правила соединения:

- Реконнект с экспоненциальной задержкой: 1, 2, 4, 8, 16, дальше 30 секунд, с джиттером до 30%.
- Пинг клиента раз в 20 секунд, сервер закрывает соединение без пинга 60 секунд.
- После реконнекта клиент всегда шлёт `subscribe` и заменяет стор пришедшим снапшотом, а не мержит.
- Сервер держит на соединение буфер 64 сообщения. Переполнение — соединение закрывается с кодом 1011, клиент переподключается и берёт снапшот.

Примеры конвертов каждого типа лежат в `/contracts/fixtures/ws_events.json` и проверяются контрактным тестом с обеих сторон.

---

## 10. Уведомления

Доставка идёт двумя путями, оба обязательны:

- **Приложение открыто.** WS-событие `service.due` поднимает баннер внутри приложения и меняет цвет стола. Локальное уведомление не шлём, чтобы не дублировать.
- **Приложение в фоне или закрыто.** Планировщик ставит `push_deliveries` на каждую активную сессию персонала этого заведения, воркер отправляет пуш через Expo.

Локальные запланированные уведомления не используем: другой мастер подтвердил задачу, а отменить уже запланированное локальное уведомление на закрытом приложении нельзя. Сервер решает в момент дедлайна, отправлять или нет.

Формат пуша:

```json
{
  "to": "ExponentPushToken[...]",
  "title": "Стол 15",
  "body": "Стряхните угли",
  "sound": "default",
  "priority": "high",
  "categoryId": "service_action",
  "data": { "event_id": "uuid", "hookah_id": "uuid", "table_id": "uuid", "hall_id": "uuid", "kind": "shake", "due_at": "..." }
}
```

Категория `service_action` даёт кнопки «Беру» и «Выполнено» прямо в уведомлении, обе бьют в соответствующий эндпоинт.

Правила отправки:

- Дедупликация — unique `(service_event_id, session_id, attempt)`. Повторный тик планировщика не отправит второй пуш.
- Повтор, пока событие не подтверждено: раз в `push_repeat_sec`, максимум `push_max_attempts` попыток. Подтверждение или переход в `missed` останавливает повторы.
- Ответ Expo `DeviceNotRegistered` → `status = 'invalid_token'`, `expo_push_token` в сессии обнуляется.
- Транспортная ошибка → `status = 'failed'`, повтор через 5, 15, 45 секунд, потом сдача.
- Мастеру, который сам подтвердил задачу, повторы не идут.

Реальные ключи Expo не блокируют разработку: `push.Sender` в dev и в тестах — `fake`, он пишет в `push_deliveries` и в лог. Переключение через env `PUSH_PROVIDER=expo|fake`.

---

## 11. Общие типы

Go-структуры в `internal/domain` и TS-типы в `src/api/types.ts` описывают одно и то же. Имена полей в JSON — `snake_case`, в TS — тоже `snake_case`, без конвертации на границе.

```ts
export type Role = 'owner' | 'admin' | 'master';
export type Shape = 'rect' | 'circle';
export type TableStatus = 'free' | 'green' | 'yellow' | 'red';
export type HookahState = 'active' | 'expired' | 'finished';
export type EventKind = 'check' | 'shake' | 'replace' | 'expire' | 'custom';
export type EventState = 'pending' | 'due' | 'confirmed' | 'missed' | 'cancelled';
export type FinishReason = 'guest_left' | 'replaced' | 'mistake' | 'shift_end';

export type PlanStep = {
  kind: EventKind;
  title: string;
  offset_sec: number;
  repeat_every_sec?: number;
};

export type Plan = {
  version: number;
  steps: PlanStep[];
  warn_before_sec: number;
  grace_sec: number;
  missed_after_sec: number;
  push_repeat_sec: number;
  push_max_attempts: number;
};

export type Table = {
  id: string; hall_id: string; label: string; shape: Shape;
  x: number; y: number; w: number; h: number; rotation: number; seats: number;
};

export type ServiceEvent = {
  id: string; hookah_id: string; seq: number; kind: EventKind; title: string;
  due_at: string; state: EventState;
  claimed_by: string | null; claimed_by_name: string | null; claimed_at: string | null;
  confirmed_by: string | null; confirmed_by_name: string | null; confirmed_at: string | null;
  confirmed_early: boolean;
};

export type Hookah = {
  id: string; table_id: string; hall_id: string; state: HookahState;
  started_at: string; expires_at: string; finished_at: string | null;
  started_by: string; started_by_name: string;
  status: TableStatus;
  events: ServiceEvent[];
};
```

Правило: поле появляется сначала здесь и в разделе 6, потом в коде. Сессия, которой не хватает поля, останавливается и выдаёт `CONTRACT GAP`.

---

## 12. UI

### Токены

Тёмная тема по умолчанию, светлую в v1 не делаем: кальянная работает в полутьме.

```ts
export const color = {
  bg: '#0E0F13', surface: '#171A21', surfaceAlt: '#1F2430', border: '#2A3040',
  text: '#F2F4F8', textMuted: '#9AA3B2',
  green: '#2ECC71', yellow: '#F1C40F', red: '#E74C3C',
  accent: '#4C8DFF', danger: '#E74C3C',
};
export const space = [0, 4, 8, 12, 16, 24, 32, 48];
export const radius = { sm: 6, md: 12, lg: 20, pill: 999 };
export const font = {
  h1: { size: 28, weight: '700' }, h2: { size: 22, weight: '600' },
  body: { size: 16, weight: '400' }, label: { size: 14, weight: '500' },
  mono: { size: 18, weight: '600' },
};
export const hit = { min: 48 };
```

Правила интерфейса: тап-таргет от 48dp, счётчики моноширинным начертанием, основные действия в нижней трети экрана под большой палец, цвет никогда не единственный носитель смысла (рядом с цветом стола идёт иконка и текст статуса).

### Примитивы

Собираются до начала фич, лежат в `src/ui`, рендерятся в служебном экране `app/kitchen-sink.tsx`.

| Компонент | Пропсы |
|---|---|
| `Screen` | `children, scroll?, footer?` |
| `AppBar` | `title, subtitle?, left?, right?` |
| `Button` | `title, onPress, variant: primary\|secondary\|danger\|ghost, size: md\|lg, loading?, disabled?, icon?` |
| `IconButton` | `icon, onPress, variant?, accessibilityLabel` |
| `Text` | `variant: h1\|h2\|body\|label\|mono, color?, children` |
| `Input` | `value, onChangeText, label?, placeholder?, error?, keyboardType?` |
| `NumberField` | `value, onChange, min, max, step, suffix?` |
| `CodeInput` | `length: 4, value, onChange, autoFocus?` |
| `Card` | `children, onPress?, status?: TableStatus` |
| `Badge` | `title, tone: neutral\|green\|yellow\|red\|accent, icon?` |
| `ListRow` | `title, subtitle?, right?, onPress?` |
| `Sheet` | `visible, onClose, title, children, footer?` |
| `ConfirmDialog` | `visible, title, message, confirmTitle, onConfirm, onCancel, destructive?` |
| `SegmentedControl` | `options, value, onChange` |
| `Countdown` | `target: string, from?: string, mode: up\|down, status?` |
| `TableShape` | `table: Table, status: TableStatus, label, badge?, selected?, onPress, onLongPress?` |
| `HallCanvas` | `tables, statuses, onTablePress, editable?, onTableMove?` |
| `Toast` | `message, tone, actionTitle?, onAction?` |
| `EmptyState` | `icon, title, message, actionTitle?, onAction?` |
| `Spinner` | `size?` |

Запрет: своя вёрстка кнопок и карточек внутри фичи. Не хватает примитива — добавляешь его в `src/ui`, записываешь в эту таблицу, бампаешь версию ядра.

### Экраны

| Роут | Назначение |
|---|---|
| `app/join.tsx` | ввод кода, имени, выбор зала |
| `app/(hall)/index.tsx` | главный экран: схема зала, столы со статусами и счётчиками, полоса активных задач |
| `app/(hall)/table/[id].tsx` | шит стола: таймер кальяна, список задач, кнопки Беру, Выполнено, Перенести, Завершить |
| `app/(hall)/tasks.tsx` | список всех задач зала по срочности, для мастера в движении |
| `app/settings/layout.tsx` | редактор планировки: создание, перемещение, форма, размер столов |
| `app/settings/plan.tsx` | редактор интервалов сервиса |
| `app/settings/staff.tsx` | персонал, роли, блокировка, ротация кода |
| `app/kitchen-sink.tsx` | проверка примитивов, не попадает в production-сборку |

Главный экран держит одно требование: мастер за 3 секунды видит, какие столы требуют внимания. Жёлтые и красные столы поднимаются в полосу задач сверху, схема ниже.

---

## 13. Стратегия тестов

Тесты привязаны к слайсу и PR, отдельной сущности «тесты на стадию» нет. Слайс мёрджится только с тестами.

Главное правило: **тесты выводятся из критериев приёмки задачи, не из реализации.** Тест кодирует контракт. Запрещено писать код, а потом тест, повторяющий то, что код делает, вместе с багами.

Обязательные типы на каждый слайс:

1. **Контрактные на стыках.** Payload WS-события и ответ эндпоинта сверяются с фикстурой из `/contracts/fixtures`. Фейковый `push.Sender` валидирует payload и падает на мусоре.
2. **Идемпотентность.** Каждый запрос планировщика и каждая мутация с `Idempotency-Key` прогоняются дважды с тем же входом, проверяется ровно один эффект. Без этого получишь два кальяна на столе и двойные пуши.
3. **Путь ошибки.** Occupied-стол, завершённый кальян, гонка двух подтверждений, обрыв WS с разрывом `seq`, Expo вернул 500, токен устройства невалиден.
4. **Property-based на чистой логике** (`rapid` в Go): `MaterializePlan` даёт монотонные `due_at`, `seq` без дыр, `expire` последним, ни одного события после `expire` при любом валидном плане. `TableStatus` не выдаёт `free` при живом кальяне.
5. **Общие фикстуры.** Go-тест и Jest-тест читают один JSON из `/contracts/fixtures`. Расхождение серверной и клиентской реализации статуса красит один и тот же тест в обоих проектах.

Уровни:

- Unit — весь `internal/domain` и `src/domain`, без БД и сети.
- Интеграционные — сервисы против эфемерного Postgres (`TEST_DATABASE_URL`), миграции применяются перед прогоном.
- API — `httptest` на хендлерах с реальным сервисом и БД, по одному happy path и всем кодам ошибок эндпоинта.
- WS — тест на хаб: подписка, снапшот, рассылка в комнату, разрыв `seq`, переполнение буфера.
- Клиент — редьюсер WS-событий (последовательность событий даёт ожидаемый стор), `status.ts`, `countdown.ts`, экраны через `@testing-library/react-native` только там, где логика не вынесена.

E2E на устройстве в v1 не автоматизируем. Вместо этого в каждом слайсе есть ручной чек-лист из критериев приёмки, который прогоняешь на двух телефонах.

---

## 14. Инфраструктура

### Конфиг

Один модуль `internal/config`, всё через env, дефолты для разработки. `.env.example` держит полный список:

```
APP_ENV=dev
HTTP_ADDR=:8080
DATABASE_URL=postgres://hookah:hookah@localhost:5432/hookah?sslmode=disable
JWT_SECRET=dev-secret-change-me
ACCESS_TTL=30m
REFRESH_TTL=2160h
SCHEDULER_TICK=1s
PUSH_TICK=5s
PUSH_PROVIDER=fake
EXPO_ACCESS_TOKEN=
LOG_LEVEL=debug
CORS_ORIGINS=
```

Приложение падает на старте, если обязательная переменная пуста в `APP_ENV=prod`.

### Миграции

`goose` в `server/migrations`, нумерация по порядку. Применяются командой в деплой-шаге, в PR-гейте прогоняются на эфемерном Postgres. Применённый файл не редактируется.

### Сид

`server/seed/seed.go` — одно заведение, два зала, 18 столов с реалистичной расстановкой, три мастера, план сервиса из ТЗ, два живых кальяна в разных фазах. Команда `make seed`. Один набор фикстур для локальной разработки, тестов и фейков.

### CI

Гейт на PR (без деплоя):

- `go vet`, `golangci-lint run`
- `go test ./... -race` с сервисом postgres
- `goose up` на эфемерном Postgres
- `tsc --noEmit`, `eslint`, `jest` в `/app`
- сборка бинарника сервера

Деплой на мёрдж в main: сборка образа, пуш, `docker compose up -d` на тестовом VPS, `goose up` в деплой-шаге. Ветка main защищена: только через PR, мёрдж при зелёном гейте.

### Сборки приложения

EAS, профиль `preview` для внутреннего тестирования (TestFlight, Android internal track), `production` для релиза. Версионирование: `expo.version` семантический, `runtimeVersion` по policy `appVersion`.

---

## 15. Конвенции

### Код на Go

- Пакет — по домену, не по слою (`internal/hookah`, не `internal/services`).
- Ошибки оборачиваются с контекстом: `fmt.Errorf("start hookah: %w", err)`. Доменные ошибки — переменные в `internal/domain/errors.go`, хендлер мапит их в коды из раздела 8 через `errors.Is`.
- `context.Context` первым аргументом во всех методах сервиса и репозитория.
- Транзакции открывает сервис через `store.WithTx`, репозиторий получает готовый `pgx.Tx`.
- Глобального состояния нет, зависимости через конструкторы, сборка в `cmd/api/main.go`.
- `panic` только в `main` при невалидном конфиге.
- Экспортируем минимум. Структуры запросов и ответов лежат рядом с хендлером.
- Тесты table-driven, имя кейса читается как предложение.

### Код на TypeScript

- `strict: true`, `any` запрещён, `unknown` с разбором на границе.
- Чистая логика в `src/domain`, без React и без сети. Компоненты её вызывают.
- Стили только из `src/ui/tokens.ts`, литералов цвета и отступов в компонентах нет.
- WS-события разбираются в одном месте (`src/store/reducer.ts`), компоненты читают стор через селекторы.
- Никакого `Date.now()` в отсчётах, только `serverNow()` из `src/ws/clock.ts`.

### Язык

Код, коммиты, PR, комментарии, имена файлов и ветки — английский. Пользовательские строки в приложении — русский, вынесены в один файл `src/i18n/ru.ts`.

### Коммиты

Conventional Commits, формат фиксированный: `type(scope): summary`.

- `type` из закрытого набора: `feat|fix|test|refactor|chore|docs`.
- `scope` — домен или пакет: `hookah`, `layout`, `ws`, `push`, `ui`, `ci`.
- `summary` в императиве, со строчной буквы, без точки в конце, до 50 символов.
- Тело только когда нужно объяснить **почему**, не **что**.

Коммить по ходу работы, маленькими логическими шагами после каждого осмысленного куска, не сваливать слайс одним коммитом в конце. Каждый коммит по возможности проходит тайпчек.

Примеры:

```
feat(hookah): materialize service events on start
fix(ws): resend snapshot after seq gap
test(scheduler): cover double tick idempotency
```

### Ветки и PR

- Ветка на слайс: `slice/03-hookah-lifecycle`. Внутри слайса — коммиты, не подветки.
- PR в main на каждый слайс, тело коротко: что делает слайс, какие контракты затрагивает, чем покрыт тестами, что проверил руками на телефонах.
- Мёрдж только при зелёном гейте. Self-review по чек-листу из `CLAUDE.md` перед мёрджем.

### Комментарии

Коротко и по делу, объясняют **почему**. Пересказ очевидного кода не нужен. Закомментированный код в PR не оставляем.

### Проза

Документы, описания PR, тексты в приложении: активный залог, императив, конкретика вместо общих фраз, без филлеров, без em-dash, без эмодзи.

---

## 16. Definition of Done

Задача закрыта, когда сходится всё:

- `golangci-lint`, `go vet`, `tsc --noEmit`, `eslint` чистые.
- `go test ./... -race` и `jest` зелёные локально и в CI.
- Тесты выведены из критериев приёмки задачи. Для каждого запроса планировщика и каждой идемпотентной мутации есть тест повторного прогона. На стыке слайса есть контрактный тест против фикстуры.
- Миграции применяются на чистой БД и на БД с сидом.
- Ручной чек-лист слайса пройден на двух устройствах, синхронизация проверена глазами.
- Изменение контракта отражено в `tech.md` с бампом версии и строкой changelog.
- Коммиты по конвенции, PR смёрджен, автодеплой прошёл, сервер на тестовом VPS ответил на `/healthz`.

---

## 17. Дорожная карта

Шесть слайсов, каждый — вертикаль от SQL до экрана. Слайс не начинается, пока предыдущий не в main.

### Слайс 0. Скелет

Цель: рабочий каркас, на который дальше только доливаются фичи. Фичи не начинаем, пока чек-лист не зелёный целиком.

Состав:

- Репозиторий, монорепо, `Makefile`, `docker-compose.yml`, `.env.example`.
- Сервер: конфиг, пул pgx, роутер, middleware (request id, логи, recover, auth guard), единый формат ошибок, `/healthz`.
- Миграция `0001_init.sql` со всей схемой раздела 6.
- Доменное ядро: `MaterializePlan`, `TableStatus`, переходы состояний, доменные ошибки. С property-тестами и фикстурами `/contracts/fixtures/plan_cases.json`, `status_cases.json`.
- WS-хаб с комнатами и `seq`, эхо тестового события, клиентский WS-модуль с реконнектом и синхронизацией часов.
- Планировщик и push-воркер с фейковым `Sender`, без реальных событий.
- Сид-скрипт.
- Expo-приложение: навигация, гард авторизации, `src/ui` со всеми примитивами раздела 12, `kitchen-sink`, токены.
- CI: гейт на PR, деплой на VPS из main.
- Эталонная вертикаль: `/healthz` плюс `GET /me` с реальным токеном, отрисованный на экране. Дальше все слайсы повторяют её раскладку файлов.

Чек-лист «скелет готов», проверяется по пунктам:

- CI зелёный на тривиальном PR.
- Миграции проходят на эфемерном Postgres в CI.
- Навигация, гард авторизации и tab-раскладка в main.
- Все примитивы `src/ui` отрендерены в `kitchen-sink` на телефоне.
- WS эхает тестовое событие в приложение, клиент считает offset времени.
- Планировщик крутится и логирует пустой тик.
- Фейковый push-sender пишет в `push_deliveries`.
- Сид наполняет БД, приложение показывает `GET /me` из реальной БД.
- Сервер задеплоен на тестовый VPS, `/healthz` отвечает по HTTPS.

### Слайс 1. Вход и заведение

Цель: владелец создаёт заведение, мастер входит по коду и видит свои залы.

Состав: регистрация и логин аккаунта, создание заведения с планом по умолчанию из ТЗ, генерация и ротация 4-значного кода, `POST /join` с созданием `staff` и `staff_sessions`, rate limit по `join_attempts`, access и refresh токены с ротацией, регистрация expo push-токена, `GET /me`, экран `join.tsx` с `CodeInput` и выбором зала, безопасное хранение токенов, автологин при запуске.

Критерии приёмки:

- Владелец регистрируется и получает заведение с планом по умолчанию.
- Мастер входит по коду, вводит имя, попадает в выбор зала. Второе устройство с тем же именем создаёт вторую сессию, не второго мастера.
- Неверный код отдаёт `invalid_join_code`, шестая попытка за минуту отдаёт `rate_limited` с `retry_after_sec`.
- Истёкший access обновляется по refresh прозрачно для экрана, старый refresh после ротации не работает.
- Заблокированный мастер получает `forbidden` на любом эндпоинте.
- Ротация кода не разлогинивает активные сессии.

Тесты: идемпотентность `join` с тем же `device_id`, ротация refresh, rate limit на границе, контрактный тест ответа `/me`, unit на хэширование пароля и парсинг клеймов.

### Слайс 2. Планировка зала

Цель: администратор рисует зал, все мастера видят одну схему.

Состав: CRUD залов и столов, батч-обновление позиций, архивация столов с проверкой занятости, редактор `settings/layout.tsx` (создание стола, перетаскивание с привязкой к сетке, форма, размер, поворот, номер), `HallCanvas` и `TableShape` на SVG, pan и zoom, подсветка пересечений, WS-событие `layout.updated`, применение планировки на главном экране без перезапуска.

Критерии приёмки:

- Администратор создаёт зал, добавляет 18 столов, задаёт номера, тащит их в расстановку реального зала, всё сохраняется и выживает перезапуск приложения.
- Два номера столов в одном зале не дублируются, повтор отдаёт `validation_failed`.
- Мастер на другом устройстве видит новую планировку сразу по `layout.updated`, без перелогина.
- Стол с живым кальяном не архивируется, отдаёт `table_busy`.
- Мастер без роли admin не открывает редактор и получает `forbidden` на эндпоинтах планировки.
- Батч-обновление 18 столов после перетаскивания идёт одним запросом.

Тесты: валидация координат и размеров, уникальность номера при архивации и восстановлении, контрактный тест `layout.updated`, unit на привязку к сетке и пересечения, интеграционный на батч в транзакции.

### Слайс 3. Жизненный цикл кальяна

Цель: ядро продукта. Кальян запускается, тикает, переносится, завершается, все мастера видят одно состояние.

Состав: `POST /tables/{id}/hookah` с `Idempotency-Key` и материализацией всех событий из снапшота плана, `transfer`, `finish`, `GET /halls/{id}/state`, планировщик (три запроса раздела 7), WS-события `hookah.*` и `service.due`, `service.missed`, главный экран со схемой, цветами, счётчиками и полосой задач, шит стола с таймером и действиями, экран `tasks.tsx`, редьюсер WS-событий и восстановление по снапшоту при разрыве `seq`.

Критерии приёмки:

- Тап по столу запускает кальян, стол зеленеет, счётчик идёт от нуля, все события плана лежат в БД сразу.
- Повторный тап по занятому столу отдаёт `table_occupied`, второго кальяна нет.
- Ретрай запроса с тем же `Idempotency-Key` отдаёт тот же кальян.
- Второе устройство видит новый кальян в течение секунды.
- На 5, 15, 30 минуте стол становится жёлтым у обоих мастеров, задача попадает в полосу задач.
- На 1:20 кальян уходит в `expired`, стол красный у всех.
- Перенос со стола 15 на 22 на 20 минуте сохраняет счётчик и все будущие дедлайны, стол 15 освобождается.
- Перенос на занятый стол отдаёт `table_occupied`.
- Завершение освобождает стол, будущие события уходят в `cancelled`, пуши по ним не идут.
- Убитый и переоткрытый WS восстанавливает состояние по снапшоту, разрыв `seq` тоже.
- Клиент, у которого часы уехали на 10 минут, показывает верные счётчики.

Тесты: property-тесты `MaterializePlan` на сгенерированных планах, идемпотентность каждого запроса планировщика (двойной тик — один эффект), гонка двух стартов на одном столе, инвариант неизменности `due_at` после подтверждения и переноса, контрактные тесты всех `hookah.*` событий против фикстур, тест редьюсера на последовательности событий, тест `status.ts` и `TableStatus` против общей фикстуры.

### Слайс 4. Уведомления и подтверждения

Цель: мастер узнаёт о задаче, когда приложение закрыто, и видит, кто задачу взял.

Состав: `claim` и `confirm` с обработкой гонки, раннее подтверждение, WS-события `service.claimed` и `service.confirmed`, push-воркер на реальном Expo (переключатель провайдера), категория `service_action` с кнопками в уведомлении, обработка тапа по уведомлению с переходом на стол, повторы и дедупликация пушей, обработка `DeviceNotRegistered`, бейджи «взял Ерлан» на столе и в задаче, in-app баннер при открытом приложении.

Критерии приёмки:

- Приложение закрыто, дедлайн наступил, пуш пришёл на оба телефона за секунды.
- Кнопка «Беру» в уведомлении ставит `claimed_by`, у второго мастера появляется бейдж с именем.
- Кнопка «Выполнено» подтверждает задачу, у второго мастера она гаснет, стол зеленеет.
- Подтверждение не сдвигает следующий дедлайн.
- Два мастера жмут подтверждение одновременно, оба видят успех, в БД один `confirmed_by`.
- Неподтверждённая задача даёт повтор пуша через `push_repeat_sec`, максимум `push_max_attempts`, дальше тишина и `missed`.
- Мастер, подтвердивший задачу, повторов не получает.
- Приложение открыто — приходит только in-app баннер, дубля пуша нет.
- Удалённое приложение помечает токен `invalid_token` и не ломает отправку остальным.

Тесты: идемпотентность вставки `push_deliveries` (двойной тик — одна строка), гонка подтверждений в транзакции, путь ошибки Expo (500, таймаут, `DeviceNotRegistered`) через фейк, контрактный тест payload пуша, тест раннего подтверждения (`confirmed_early = true`, дедлайны на месте).

### Слайс 5. Настройки, роли, релиз

Цель: заведение настраивает интервалы под себя, персонал управляем, приложение уходит в сторы.

Состав: редактор плана сервиса `settings/plan.tsx` с валидацией раздела 7 и предпросмотром расписания, `PUT /venues/current/service-plan` с `plan.updated`, экран персонала (роли, блокировка, ротация кода), несколько залов с переключателем, пустые состояния и ошибки сети на всех экранах, иконка и сплеш, EAS-профили, сборки `preview` и `production`, прогон на двух реальных устройствах в живой смене.

Критерии приёмки:

- Администратор меняет интервал стряхивания с 15 на 12 минут, новый кальян живёт по новому плану, уже запущенные — по старому.
- План, нарушающий ограничения (два `expire`, `offset_sec` больше `expire`, шаг в 10 секунд), отдаёт `plan_invalid` с причиной, приложение показывает её на поле.
- Предпросмотр показывает первые 10 задач по времени до сохранения.
- Изменение плана прилетает на устройства по `plan.updated`.
- Владелец блокирует мастера, тот теряет доступ в течение секунды и не может войти заново по коду.
- Переключение зала перерисовывает главный экран и переподписывает WS на новую комнату.
- Обрыв сети показывает баннер «нет связи», кнопки мутаций блокируются, после возврата связи состояние восстанавливается снапшотом.
- Сборки `preview` устанавливаются на iOS и Android, проходят живую смену без перезапуска.

Тесты: валидация плана на границах, контрактный тест `plan.updated`, интеграционный на изоляцию живых кальянов от смены плана, тест блокировки мастера (отзыв сессий, отказ повторного входа), тест переподписки WS при смене зала.

---

## 18. Что дальше v1

Записано, чтобы не тянуть в v1: история смены и отчёты по мастерам, среднее время реакции, несколько заведений на одном аккаунте, веб-панель администратора, офлайн-очередь мутаций, горизонтальное масштабирование WS через LISTEN/NOTIFY, интеграция с POS для автозапуска кальяна по чеку.

---

## 19. Дополнения контракта

Раздел пополняется только дописыванием. Каждый подраздел закрывает `CONTRACT GAP`, найденный при реализации, и помечен версией ядра.

### 19.1. v2: бутстрап, вход, скелет

**Общие типы** (дополняют раздел 11):

```ts
export type Platform = 'ios' | 'android';

export type Venue = {
  id: string; name: string; timezone: string;
  join_code: string | null; // null для роли master
};

export type Hall = { id: string; venue_id: string; name: string; sort_order: number };

export type Staff = {
  id: string; venue_id: string; display_name: string; role: Role;
  blocked: boolean; created_at: string; last_seen_at: string | null;
};

export type Tokens = { access_token: string; refresh_token: string; access_expires_at: string };

export type Me = { staff: Staff; venue: Venue; halls: Hall[] };
```

`halls` везде отсортированы по `sort_order`, затем по `name`.

**Аккаунт и сессия устройства.** Владелец тоже `staff` (роль `owner`, `account_id` заполнен), его устройство тоже держит строку `staff_sessions`. Поэтому register и login принимают устройство:

```
POST   /auth/register   {email, password, display_name, venue_name, device_id, platform} → {tokens, venue}
POST   /auth/login      {email, password, device_id, platform} → {tokens, venue}
```

Повторный login или join с тем же `device_id` обновляет существующую сессию (новый refresh, `revoked_at = null`), вторую строку не создаёт. Неверный email или пароль отдаёт `unauthorized`. Занятый email отдаёт `validation_failed` с `details.fields.email = "taken"`.

**Валидация входа.** Строки обрезаются по пробелам до проверки.

| поле | правило |
|---|---|
| `email` | 3..254 символа, содержит `@` |
| `password` | 8..72 байта (предел bcrypt) |
| `display_name` | 1..40 символов |
| `venue_name` | 1..80 символов |
| `device_id` | 8..128 символов |
| `platform` | `ios` или `android` |
| `code` | ровно 4 цифры |

`details.fields` — объект `{поле: причина}`, причины: `required`, `too_short`, `too_long`, `invalid`, `taken`.

**Поиск мастера при входе по коду.** `POST /join` ищет в заведении `staff` без `account_id` с тем же `display_name` без учёта регистра. Нашёл — создаёт или обновляет сессию этого `staff`. Не нашёл — создаёт `staff` с ролью `master`. Найденный `staff` с `blocked_at` отдаёт `forbidden`. Гонку двух входов с одним именем закрывает миграция `0002_staff_name.sql`:

```sql
create unique index staff_venue_name_key on staff (venue_id, lower(display_name))
  where account_id is null;
```

**Пароль.** bcrypt, cost 10, пакет `golang.org/x/crypto/bcrypt`.

**`/healthz`.** Без префикса `/api/v1` и без авторизации. `200 {"status":"ok"}`, если пинг БД прошёл, иначе `503 {"status":"unavailable"}`. Формат ошибок раздела 8 на него не распространяется.

**WS: прямые ответы.** `state.snapshot`, `pong` и `error` уходят одному соединению и `seq` не увеличивают. В них `seq` равен последнему выданному `seq` заведения на момент отправки. Клиент берёт `seq` снапшота базой и игнорирует `seq` в `pong` и `error`. Следующее широковещательное событие должно прийти с `seq = база + 1`, любое другое значение, включая меньшее после рестарта сервера, считается разрывом.

`hall_id` в конверте равен `null` у событий уровня заведения (`plan.updated`) и у `pong` и `error`.

Невалидный или истёкший токен в `GET /api/v1/ws` отдаёт `401 unauthorized` до апгрейда, заблокированный `staff` или отозванная сессия отдают `403 forbidden`. `subscribe` на чужой или несуществующий зал отдаёт сообщение `error` с `code = not_found`.

**Проверка сессии.** Auth guard на каждом запросе проверяет JWT и то, что сессия `session_id` не отозвана, а `staff` не заблокирован. Отозванная сессия отдаёт `unauthorized`, заблокированный `staff` отдаёт `forbidden`.

**Структура папок** (дополняет раздел 5):

```
/server
  cmd/migrate/main.go     goose up/status на встроенных файлах migrations/*.sql
  cmd/seed/main.go        запускает seed.Seed против DATABASE_URL
  cmd/devtoken/main.go    печатает access-токен для staff из сида, только APP_ENV=dev
  migrations/embed.go     //go:embed *.sql
  Dockerfile
/deploy
  docker-compose.prod.yml api + postgres + caddy на VPS
  Caddyfile               TLS и reverse-proxy на api
.github/workflows/
  ci.yml                  гейт на PR
  deploy.yml              деплой из main
```

`seed` — пакет с функцией `Seed(ctx, pool) (Fixture, error)`, ей пользуются и `cmd/seed`, и интеграционные тесты.

**Конфиг** (дополняет раздел 14): `.env.example` получает `TEST_DATABASE_URL=postgres://hookah:hookah@localhost:5432/hookah_test?sslmode=disable`. Интеграционные тесты пропускаются через `t.Skip`, если переменная пуста, в CI она задана всегда.

### 19.2. v3: клиент скелета, доставка WS

**Зависимости клиента** (дополняют раздел 3). Expo SDK 57, React Native 0.86, React 19.2, TypeScript 6. Версии ставит `npx expo install`, совместимые с SDK.

- Рантайм: `expo-crypto` (UUID для `device_id` и `Idempotency-Key`), `expo-device` (пуш-токен только на физическом устройстве), `expo-constants`, `expo-linking`, `expo-status-bar`, `react-native-safe-area-context`, `react-native-screens` (обязательные пиры `expo-router`), `react-dom` (пир `expo-router`, в сборку под iOS и Android не попадает).
- Разработка: `typescript`, `@types/react`, `jest`, `jest-expo` (пресет), `@types/jest`, `eslint`, `eslint-config-expo`.

**Конфиг клиента.** `app/.env.example`:

```
EXPO_PUBLIC_API_URL=http://192.168.1.10:8080
EXPO_PUBLIC_DEV_ACCESS_TOKEN=
```

WS-адрес выводится из `EXPO_PUBLIC_API_URL` заменой схемы `http` на `ws`. `EXPO_PUBLIC_DEV_ACCESS_TOKEN` читается только при `__DEV__` и только если в `expo-secure-store` нет сессии: это вход слайса 0 до появления `join`. Значение печатает `make devtoken`.

**Роуты** (уточняют раздел 12):

```
app/_layout.tsx               корневой Stack, гард авторизации, запуск WS
app/join.tsx
app/(hall)/_layout.tsx        Tabs: index «Зал», tasks «Задачи»
app/(hall)/index.tsx
app/(hall)/tasks.tsx
app/(hall)/table/[id].tsx     модальный экран поверх табов
app/settings/_layout.tsx      Stack, только owner и admin
app/kitchen-sink.tsx          доступен только при __DEV__
```

Гард: нет сессии → `join`, есть сессия → `(hall)`.

**Доставка WS-событий.** Хаб рассылает каждое событие заведения всем соединениям заведения, у которых была подписка, независимо от зала. Поэтому `seq` у каждого соединения идёт без дыр. Клиент отслеживает `seq` по всем событиям, а в стор применяет только события своего зала и события уровня заведения (`hall_id = null`). Комната `hall_id` определяет, чей снапшот получает соединение и кто попадает в `staff_online`.

События, опубликованные, пока сервер читает снапшот, приходят сразу после снапшота, если их `seq` больше `seq` снапшота. Эффект такого события уже может быть в снапшоте, поэтому редьюсер применяет события идемпотентно.

**Интеграционные тесты сервера.** Пакет `internal/store/storetest` создаёт на каждый тест отдельную БД клоном мигрированного шаблона `hookah_template`. Пакеты гоняются параллельно без общей БД.

**Время в ответах.** Все временные поля в HTTP и WS сериализуются в UTC с суффиксом `Z`.

### 19.3. v4: UI-кит и ресинк

**Примитив `Icon`** (дополняет таблицу раздела 12):

| Компонент | Пропсы |
|---|---|
| `Icon` | `name: IconName, size?, color?: ColorToken` |

Иконки рисуются через `react-native-svg`, отдельной библиотеки иконок нет. Набор имён растёт вместе с экранами. Пропсы `icon` у `Button`, `IconButton`, `Badge`, `EmptyState` принимают `IconName`.

**Дополнительные пропсы.**

- `Text`: `numberOfLines?`.
- `HallCanvas`: `badges?: Record<table_id, string>` (подпись на столе: счётчик или имя взявшего задачу), `selectedId?`.
- `IconButton.variant`: `plain | filled`.
- `SegmentedControl.options`: `{value, label}[]`.
- `Toast.tone`: `neutral | success | warning | danger`.

**`Countdown`.** `mode: up` показывает время с момента `target`, `mode: down` показывает время до `target`. `from` замораживает счётчик на указанном моменте (кальян завершён). Без `from` счётчик тикает от `serverNow()` раз в секунду.

**Статус без цвета.** `src/ui/status.ts` сопоставляет каждому `TableStatus` цвет, иконку и слово из `ru.ts`. Каждый примитив со статусом берёт все три.

**Ресинк.** Разрыв `seq` клиент закрывает повторным `subscribe` на тот же зал, а не `GET /halls/{id}/state`: ответ на `subscribe` несёт `seq`, от которого считаются следующие события. `GET /halls/{id}/state` остаётся для REST-клиентов и тестов.

**Файлы клиента** (дополняют раздел 5):

```
src/api/decode.ts     разбор unknown в типы контракта на границе сети
src/api/session.ts    хранение токенов в expo-secure-store
src/store/auth.ts     сессия и /me, гард читает отсюда
src/ui/status.ts      цвет, иконка и слово статуса
src/ui/useServerNow.ts хук, который перерисовывает компонент по serverNow()
src/env.d.ts          типы EXPO_PUBLIC_*
```

ESLint запрещает `Date.now()` во всём клиенте, кроме `src/ws/clock.ts`.

### 19.4. v5: вход и сессии

**Rate limit входа по коду.** В `join_attempts` пишется каждая попытка с неверным кодом (`success = false`) и каждая с верным кодом (`success = true`). Попытка, отбитая самим лимитом, не пишется, иначе спам продлевал бы блокировку бесконечно. Лимит считает все записи устройства: не больше 5 за последние 60 секунд и не больше 20 за последние 3600 секунд. Шестая попытка за минуту отдаёт `rate_limited`.

`retry_after_sec` — через сколько секунд из окна выйдет самая старая попытка, округление вверх. Если превышены оба окна, берётся большее значение. Проверка и запись идут в одной транзакции под `pg_advisory_xact_lock` по `device_id`, параллельные запросы одного устройства лимит не обходят.

Чистая логика лимита и валидации входа лежит в `internal/domain/join.go`.

**Срок refresh.** Отдельной колонки нет. Refresh живёт `REFRESH_TTL` с момента выдачи, момент выдачи — `staff_sessions.last_seen_at`, его обновляют только выдача токенов (register, login, join, refresh). Refresh старше `REFRESH_TTL`, отозванная сессия или хэш, не совпавший после ротации, отдают `unauthorized`. Заблокированный `staff` на refresh получает `forbidden`. Ротация идёт условным `update ... where id = $1 and refresh_token_hash = $2`: из двух параллельных refresh с одним токеном выигрывает один, второй получает `unauthorized`.

**Выход.** `POST /auth/logout` и `DELETE /sessions/current` делают одно и то же: пишут `revoked_at = now()` и обнуляют `expo_push_token` текущей сессии.

**Push-токен.** `expo_push_token` имеет вид `ExponentPushToken[...]` или `ExpoPushToken[...]`, до 200 символов, иначе `validation_failed` с `details.fields.expo_push_token = "invalid"`.

**Код входа.** Генерируется `crypto/rand`, 4 цифры с ведущими нулями. Занятый код выбирается заново, до 20 попыток, дальше `internal`. `POST /venues/current/rotate-code` доступен ролям `owner` и `admin`, как в разделе 8.

**Вход владельца.** `app/join.tsx` держит три режима через `SegmentedControl`: «Код» для мастера, «Вход» и «Регистрация» для владельца. Отдельных роутов нет. Выбранный зал хранится на устройстве в `expo-secure-store` и переживает перезапуск.

**Обновление access на клиенте.** Клиент обновляет access заранее, если до `access_expires_at` по `serverNow()` осталось меньше 60 секунд, и один раз повторяет запрос после `unauthorized`. Параллельные запросы делят один вызов `/auth/refresh`. Неудачный refresh чистит сессию и отправляет на `join`.

### 19.5. v6: клиент входа

**`Input.secure?`** прячет ввод и выключает автозамену и заглавные буквы. Для `keyboardType = email-address` автозамена тоже выключена.

**Фикстура `/contracts/fixtures/me.json`** держит пример ответа `GET /me`. Go-тест сверяет с ней тип и форму живого ответа, Jest-тест прогоняет её через `decodeMe`.

**Пакет `internal/apitest`** поднимает настоящий роутер на отдельной БД из `storetest`. API-тесты ходят в него по HTTP, как приложение.

**Файлы клиента:**

```
src/api/device.ts                    device_id: UUID из expo-crypto, живёт в expo-secure-store до переустановки
src/features/join/form.ts            проверка полей до запроса и разбор ошибок API в текст на поле
src/features/notifications/registerPush.ts  разрешение, Expo push-токен, POST /sessions/push-token
```

Push-токен запрашивается только на физическом устройстве и только при заданном `extra.eas.projectId`. Без него приложение работает через WS, пока открыто.

### 19.6. v7: планировка

**Валидация.** Строки обрезаются по пробелам.

| поле | правило |
|---|---|
| `halls.name` | 1..60 символов, уникально в заведении без учёта регистра (`taken`) |
| `halls.sort_order` | 0..1000 |
| `tables.label` | 1..8 символов, уникально среди неархивных столов зала (`taken`) |
| `tables.shape` | `rect` или `circle` |
| `tables.x`, `tables.y` | 0..99 |
| `tables.w`, `tables.h` | 1..40, стол целиком внутри сетки: `x + w <= 100`, `y + h <= 100` (иначе `out_of_bounds` на `x` или `y`) |
| `tables.rotation` | 0..359 |
| `tables.seats` | 1..30 |

Поворот стол за сетку не проверяет: границы считаются по неповёрнутому прямоугольнику. Чистая проверка лежит в `internal/domain/layout.go`.

**Частичное обновление стола.** `PATCH /tables/{table_id}` принимает любое подмножество `{label, shape, x, y, w, h, rotation, seats, archived}`. Проверка идёт по итоговому столу после слияния. `archived: true` равносильно `DELETE`, `archived: false` восстанавливает стол. Восстановление с номером, который уже занят живым столом зала, отдаёт `validation_failed` с `label = taken`. Архивные столы в v1 видны только в БД, UI восстановления нет.

**Батч.** `PUT /halls/{hall_id}/layout` принимает любое непустое подмножество неархивных столов зала. Стол из другого зала, архивный или несуществующий отдаёт `not_found`, повтор `id` в одном запросе отдаёт `validation_failed` с `tables = invalid`. Ошибка в одном столе откатывает весь батч, поле ошибки называется `tables[i].x` и так далее. Ответ — все неархивные столы зала.

**Удаление зала.** Зал с живым кальяном отдаёт `table_busy`. Иначе в одной транзакции удаляются кальяны зала с их событиями и переносами, переносы других кальянов со столов зала и сам зал со столами. История смен в v1 не хранится (раздел 18), поэтому потеря истории допустима.

**Публикация.** Каждая мутация столов (создание, `PATCH`, архивация, восстановление, батч) после коммита публикует `layout.updated {hall_id, tables}` со всеми неархивными столами зала. Клиент заменяет столы зала целиком. Мутации залов событий не публикуют: список залов приходит в `GET /me`.

**Клиент.** `src/domain/layout.ts` держит привязку к сетке и поиск пересечений. Пересечение считается по повёрнутым прямоугольникам (теорема о разделяющей оси), круг заменяется описанным квадратом. Результат только подсвечивает столы в редакторе.
