.PHONY: infra infra-down migrate migrate-down sqlc wire dev test build seed

infra:
	docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example up -d

infra-down:
	docker compose -f deploy/docker-compose.yml --env-file deploy/.env.example down

migrate:
	go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
		-path migrations -database "$${DATABASE_URL}" up

migrate-down:
	go run -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
		-path migrations -database "$${DATABASE_URL}" down

sqlc:
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate

wire:
	go run github.com/google/wire/cmd/wire@latest ./...

dev:
	go run ./cmd/server

test:
	go test ./... -race -count=1

build:
	CGO_ENABLED=0 go build -o bin/cmdb-core ./cmd/server

seed:
	go run ./cmd/seed
