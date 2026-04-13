.PHONY: generate test test-go test-frontend test-python build lint

generate:
	cd cmdb-core && make generate-api
	@echo "Go types generated from openapi.yaml"

test: test-go test-frontend test-python

test-go:
	cd cmdb-core && go test ./... -race -count=1

test-frontend:
	cd cmdb-demo && npx vitest run

test-python:
	cd ingestion-engine && pytest tests/ -v --tb=short

build:
	cd cmdb-core && go build ./...
	cd cmdb-demo && npm run build

lint:
	cd cmdb-core && go vet ./...
	cd cmdb-demo && npx tsc --noEmit
