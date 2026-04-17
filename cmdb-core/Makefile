.PHONY: infra infra-down migrate migrate-down sqlc wire dev test test-integration build seed generate-api check-api-routes

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

# Integration tests require a running Postgres (TEST_DATABASE_URL or the
# default docker-compose DB). Tests skip automatically if the DB is
# unreachable, so this target is safe to run locally without setup.
test-integration:
	go test -tags integration -race -count=1 ./...

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
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.6.0 \
		--config oapi-codegen.yaml ../api/openapi.yaml

# Verify api/openapi.yaml and the routes registered in cmd/server/main.go
# + internal/api/generated.go are in sync. Catches drift at PR time.
check-api-routes:
	OPENAPI_SPEC=../api/openapi.yaml \
	SERVER_MAIN=./cmd/server/main.go \
	GENERATED_GO=./internal/api/generated.go \
	go run ./cmd/check-api-routes
