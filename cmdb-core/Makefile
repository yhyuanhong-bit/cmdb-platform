.PHONY: infra infra-down migrate migrate-down sqlc wire dev test test-integration build seed generate-api check-api-routes lint errcheck tenantlint

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

# Lint targets — wired into CI so a regression that swallows an error
# blocks the merge.
#
# The custom analyzers build into bin/ on demand and run via go vet's
# -vettool protocol:
#  - errcheck (Phase 3.1): flags discarded pool.Exec/Query/Scan and
#    json.Unmarshal errors. See tools/errcheck/README.md for the
#    allow-list convention (//errcheck:allow per-line directive).
#  - tenantlint: flags direct pool access outside the database.TenantScoped
#    helper. Called out separately from `lint` because it still has a
#    known backlog of pre-existing sites in cmd/server and a few api/
#    handlers; wire it in once that backlog has been burned down.

bin/errcheck: tools/errcheck/*.go tools/errcheck/cmd/errcheck/*.go
	go build -o bin/errcheck ./tools/errcheck/cmd/errcheck

errcheck: bin/errcheck
	go vet -vettool=$(PWD)/bin/errcheck ./...

bin/tenantlint: tools/tenantlint/*.go tools/tenantlint/cmd/tenantlint/*.go
	go build -o bin/tenantlint ./tools/tenantlint/cmd/tenantlint

tenantlint: bin/tenantlint
	go vet -vettool=$(PWD)/bin/tenantlint ./...

# `lint` is the single CI-facing target. It currently runs errcheck +
# the stdlib vet. tenantlint is opt-in via `make tenantlint` until its
# pre-existing backlog is resolved.
lint: errcheck
	go vet ./...
