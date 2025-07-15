# --------------------------------------------------------------------
# Helper variables
# --------------------------------------------------------------------
GO            ?= go
GOMOD         ?= go.mod
ENV_TEST      ?= .env.test
LINTER        ?= golangci-lint
LINTER_VERSION ?= v1.59.1
LINTER_FILE    := golangci-lint-$(shell echo $(LINTER_VERSION) | sed 's/^v//')-$(OS)-$(ARCH).tar.gz
TOOLS_DIR     ?= .tools
OS            := $(shell uname -s | tr A-Z a-z)
ARCH          := $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')

# --------------------------------------------------------------------
# .PHONY targets
# --------------------------------------------------------------------
.PHONY: run test tidy lint

# --------------------------------------------------------------------
# run : start the web server
# --------------------------------------------------------------------
run:
	@command -v air >/dev/null 2>&1 && { \
		echo "▶ starting server with live reload (air) ..."; \
		air -c .air.toml; \
	} || { \
		echo "▶ starting server (go run) ..."; \
		$(GO) run ./cmd/server; \
	}

# --------------------------------------------------------------------
# test : execute whole test-suite
# --------------------------------------------------------------------
test:
	@echo "▶ running unit tests ..."
	@set -a; \
	if [ -f $(ENV_TEST) ]; then \
		echo "  – loading environment from $(ENV_TEST)"; \
		. $(ENV_TEST); \
	fi; \
	set -e; \
	$(GO) test -v ./...

# --------------------------------------------------------------------
# tidy : keep go.mod / go.sum clean
# --------------------------------------------------------------------
tidy:
	$(GO) mod tidy

# --------------------------------------------------------------------
# lint : static analysis for production code
# --------------------------------------------------------------------
lint: $(TOOLS_DIR)/$(LINTER)
	@echo "▶ running linters ..."
	@$(TOOLS_DIR)/$(LINTER) run ./...

$(TOOLS_DIR)/$(LINTER):
	@echo "▶ installing $(LINTER) $(LINTER_VERSION) ..."
	@mkdir -p $(TOOLS_DIR)
	@curl -sSL "https://github.com/golangci/golangci-lint/releases/download/$(LINTER_VERSION)/$(LINTER_FILE)" \
	  | tar -xz -C $(TOOLS_DIR) --strip-components=1 "golangci-lint-$(shell echo $(LINTER_VERSION) | sed 's/^v//')-$(OS)-$(ARCH)/$(LINTER)"
	@chmod +x $(TOOLS_DIR)/$(LINTER)


docker-run:
	docker compose -f docker-compose.local.yml up --build

docker-stop:
	docker-compose -f docker-compose.local.yml down


test-load-small:
	@echo "Running small load test..."
	go run cmd/loadtest/main.go \
		-url=http://localhost:8080 \
		-requests=1000 \
		-concurrent=50 \
		-rampup=5s \
		-duration=1m

test-load:
	@echo "Running load test with 10k requests..."
	go run cmd/loadtest/main.go \
		-url=http://localhost:8080 \
		-requests=10000 \
		-concurrent=100 \
		-rampup=30s \
		-duration=10m

test-load-5m:
	@echo "Running load test with 10k requests for 5 mins..."
	go run cmd/loadtest/main.go \
		-url=http://localhost:8080 \
		-requests=10000 \
		-concurrent=100 \
		-rampup=30s \
		-duration=5m

test-monitor:
	go run cmd/monitor/main.go