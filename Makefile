-include .env

POSTGRES_USER ?= otp_user
POSTGRES_PASSWORD ?= otp_password
POSTGRES_DB ?= otp_db
REDIS_URL ?= redis://redis:6379
MIGRATION_DATABASE_URL := postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@postgres:5432/$(POSTGRES_DB)?sslmode=disable

.PHONY: migration-up migration-down migration-version redis-up redis-down

migration-up:
	docker compose run --rm migrate -path=/migrations -database "$(MIGRATION_DATABASE_URL)" up

migration-down:
	docker compose run --rm migrate -path=/migrations -database "$(MIGRATION_DATABASE_URL)" down 1

migration-version:
	docker compose run --rm migrate -path=/migrations -database "$(MIGRATION_DATABASE_URL)" version

redis-up:
	docker compose up -d redis

redis-down:
	docker compose down redis

all-up:
	docker compose up -d postgres redis

all-down:
	docker compose down
