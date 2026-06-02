-include .env.local
export

FORMAT ?= text
CONFIG ?=
EXPECTED_EXIT ?= 1
GORELEASER_REMOTE_URL ?= https://github.com/get-felipe/pg-contract.git
GORELEASER_GIT_CONFIG = GIT_CONFIG_COUNT=2 GIT_CONFIG_KEY_0=remote.origin.url GIT_CONFIG_VALUE_0=$(GORELEASER_REMOTE_URL) GIT_CONFIG_KEY_1=remote.origin.fetch GIT_CONFIG_VALUE_1=+refs/heads/*:refs/remotes/origin/*

.PHONY: build check example example-ambiguous-column example-basic example-enum-value example-function-signature example-init example-manifest-v02 example-missing-table example-search-path example-typed-params example-view-changed fmt release-check release-snapshot test test-integration tidy

build:
	@go build -o bin/pg-contract ./cmd/pg-contract

check: fmt test build

example: build
	@test -n "$(PG_CONTRACT_BEFORE_URL)" || (echo "PG_CONTRACT_BEFORE_URL is required"; exit 2)
	@test -n "$(PG_CONTRACT_AFTER_URL)" || (echo "PG_CONTRACT_AFTER_URL is required"; exit 2)
	@test -n "$(EXAMPLE)" || (echo "EXAMPLE is required"; exit 2)
	@code=0; \
	if [ -n "$(CONFIG)" ]; then \
		./bin/pg-contract check \
			--before-url "$(PG_CONTRACT_BEFORE_URL)" \
			--after-url "$(PG_CONTRACT_AFTER_URL)" \
			--schema-before "examples/$(EXAMPLE)/schema-before.sql" \
			--schema-after "examples/$(EXAMPLE)/schema-after.sql" \
			--queries "examples/$(EXAMPLE)/queries" \
			--format "$(FORMAT)" \
			--config "$(CONFIG)" || code=$$?; \
	else \
		./bin/pg-contract check \
			--before-url "$(PG_CONTRACT_BEFORE_URL)" \
			--after-url "$(PG_CONTRACT_AFTER_URL)" \
			--schema-before "examples/$(EXAMPLE)/schema-before.sql" \
			--schema-after "examples/$(EXAMPLE)/schema-after.sql" \
			--queries "examples/$(EXAMPLE)/queries" \
			--format "$(FORMAT)" || code=$$?; \
	fi; \
	if [ "$$code" -eq "$(EXPECTED_EXIT)" ]; then \
		if [ "$(FORMAT)" = "text" ]; then \
			echo "Example $(EXAMPLE) produced the expected exit code $(EXPECTED_EXIT)."; \
		fi; \
		exit 0; \
	fi; \
	echo "Example $(EXAMPLE) produced exit code $$code; expected $(EXPECTED_EXIT)."; \
	if [ "$$code" -eq 0 ]; then exit 1; fi; \
	exit "$$code"

example-basic: EXAMPLE := basic
example-basic: example

example-missing-table: EXAMPLE := missing-table
example-missing-table: example

example-ambiguous-column: EXAMPLE := ambiguous-column
example-ambiguous-column: example

example-view-changed: EXAMPLE := view-changed
example-view-changed: example

example-function-signature: EXAMPLE := function-signature
example-function-signature: example

example-enum-value: EXAMPLE := enum-value
example-enum-value: example

example-search-path: EXAMPLE := search-path
example-search-path: example

example-typed-params: EXAMPLE := typed-params
example-typed-params: CONFIG := examples/typed-params/pg-contract.yaml
example-typed-params: EXPECTED_EXIT := 0
example-typed-params: example

example-init: build
	@./bin/pg-contract init --queries examples/basic/queries --out -

example-manifest-v02: build
	@test -n "$(PG_CONTRACT_BEFORE_URL)" || (echo "PG_CONTRACT_BEFORE_URL is required"; exit 2)
	@test -n "$(PG_CONTRACT_AFTER_URL)" || (echo "PG_CONTRACT_AFTER_URL is required"; exit 2)
	@code=0; \
	./bin/pg-contract check \
		--before-url "$(PG_CONTRACT_BEFORE_URL)" \
		--after-url "$(PG_CONTRACT_AFTER_URL)" \
		--config examples/manifest-v02/pg-contract.yaml \
		--format "$(FORMAT)" || code=$$?; \
	if [ "$$code" -eq "$(EXPECTED_EXIT)" ]; then \
		if [ "$(FORMAT)" = "text" ]; then \
			echo "Manifest v0.2 example produced the expected exit code $(EXPECTED_EXIT)."; \
		fi; \
		exit 0; \
	fi; \
	echo "Manifest v0.2 example produced exit code $$code; expected $(EXPECTED_EXIT)."; \
	if [ "$$code" -eq 0 ]; then exit 1; fi; \
	exit "$$code"

fmt:
	go fmt ./...

release-check:
	$(GORELEASER_GIT_CONFIG) go run github.com/goreleaser/goreleaser/v2@latest check

release-snapshot:
	$(GORELEASER_GIT_CONFIG) go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean

test:
	go test ./...

test-integration:
	@test -n "$(PG_CONTRACT_TEST_BEFORE_URL)" || (echo "PG_CONTRACT_TEST_BEFORE_URL is required"; exit 2)
	@test -n "$(PG_CONTRACT_TEST_AFTER_URL)" || (echo "PG_CONTRACT_TEST_AFTER_URL is required"; exit 2)
	go test ./internal/check -run TestRunWithPostgres -count=1 -v

tidy:
	go mod tidy
