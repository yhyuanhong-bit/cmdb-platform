.PHONY: generate
generate:
	cd cmdb-core && make generate-api
	@echo "Go types generated from openapi.yaml"
