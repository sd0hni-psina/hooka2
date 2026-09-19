SHELL := /bin/bash
POSTGRES_PORT ?= 5432
DATABASE_URL ?= postgres://hookah:hookah@localhost:$(POSTGRES_PORT)/hookah?sslmode=disable
TEST_DATABASE_URL ?= postgres://hookah:hookah@localhost:$(POSTGRES_PORT)/hookah_test?sslmode=disable
LINT_IMAGE := golangci/golangci-lint:v2.13.2

export DATABASE_URL TEST_DATABASE_URL

.PHONY: up down migrate seed api devtoken test lint check app-check

up: ## start local postgres
	POSTGRES_PORT=$(POSTGRES_PORT) docker compose up -d postgres

down:
	docker compose down

migrate:
	cd server && go run ./cmd/migrate up

seed: migrate
	cd server && go run ./cmd/seed

api:
	cd server && go run ./cmd/api

devtoken:
	cd server && go run ./cmd/devtoken $(ARGS)

test:
	cd server && go vet ./... && go test ./... -race -count=1

lint:
	docker run --rm -v "$(CURDIR)/server":/src -w /src -v golangci-cache:/root/.cache $(LINT_IMAGE) golangci-lint run ./...

app-check:
	cd app && npx tsc --noEmit && npx eslint . && npx jest --ci

check: lint test app-check
