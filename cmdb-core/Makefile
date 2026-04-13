.PHONY: infra infra-down migrate migrate-down sqlc wire dev test build seed generate-api

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

# Deployment targets
.PHONY: up up-edge down logs grafana jaeger

up:
	cd deploy && docker compose up -d

up-edge:
	cd deploy && docker compose -f docker-compose.yml -f docker-compose.edge.yml up -d

down:
	cd deploy && docker compose down

logs:
	cd deploy && docker compose logs -f --tail=100

grafana:
	@echo "Grafana: http://localhost:3000 (admin/admin)"

jaeger:
	@echo "Jaeger: http://localhost:16686"

generate-api:
	oapi-codegen --config oapi-codegen.yaml ../api/openapi.yaml
