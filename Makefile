SHELL := /bin/bash
# 5432 is held by another project on the author's machine.
POSTGRES_PORT ?= 5433
API_PORT ?= 8080
DATABASE_URL ?= postgres://hookah:hookah@localhost:$(POSTGRES_PORT)/hookah?sslmode=disable
TEST_DATABASE_URL ?= postgres://hookah:hookah@localhost:$(POSTGRES_PORT)/hookah_test?sslmode=disable
LINT_IMAGE := golangci/golangci-lint:v2.13.2

export DATABASE_URL TEST_DATABASE_URL

.PHONY: dev up down migrate seed api devtoken test lint check app-check

# dev is the whole local server in one command: postgres, migrations, seed, API.
# The app starts in a second terminal with `cd app && npx expo start`.
dev:
	@if lsof -ti tcp:$(API_PORT) -sTCP:LISTEN >/dev/null 2>&1; then \
		echo "Порт $(API_PORT) занят процессом $$(lsof -ti tcp:$(API_PORT) -sTCP:LISTEN | head -1). Останови его: kill $$(lsof -ti tcp:$(API_PORT) -sTCP:LISTEN | head -1)"; exit 1; fi
	POSTGRES_PORT=$(POSTGRES_PORT) docker compose up -d --wait postgres
	$(MAKE) seed
	@echo "API на :$(API_PORT). Код входа мастера 123456, владелец owner@hookah.dev / hookah-dev-pass"
	$(MAKE) api

up: ## start local postgres
	POSTGRES_PORT=$(POSTGRES_PORT) docker compose up -d postgres

down:
	docker compose down

migrate:
	cd server && go run ./cmd/migrate up

seed: migrate
	cd server && go run ./cmd/seed

api:
	cd server && HTTP_ADDR=:$(API_PORT) go run ./cmd/api

devtoken:
	cd server && go run ./cmd/devtoken $(ARGS)

test:
	cd server && go vet ./... && go test ./... -race -count=1

lint:
	docker run --rm -v "$(CURDIR)/server":/src -w /src -v golangci-cache:/root/.cache $(LINT_IMAGE) golangci-lint run ./...

app-check:
	cd app && npx tsc --noEmit && npx eslint . && npx jest --ci

check: lint test app-check
