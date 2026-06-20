.DEFAULT_GOAL := help

GO ?= go
CONFIG ?= config.yaml
BINARY ?= bin/jxh-go
COMPOSE ?= docker compose
NAPCAT_UID ?= $(shell id -u)
NAPCAT_GID ?= $(shell id -g)
GORMGEN_DSN ?= $(if $(JXH_GORMGEN_DSN),$(JXH_GORMGEN_DSN),jxh:jxh_password@tcp(127.0.0.1:3306)/jxh_bot?charset=utf8mb4&parseTime=True&loc=Local)

.PHONY: help run build test fmt tidy clean compose-up compose-down compose-logs mysql napcat gormgen-install gormgen

help: ## Show available targets
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*##/ {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

run: ## Run the bot locally
	$(GO) run ./cmd/bot -config $(CONFIG)

build: ## Build the bot binary
	@mkdir -p $(dir $(BINARY))
	$(GO) build -o $(BINARY) ./cmd/bot

test: ## Run Go tests
	$(GO) test ./...

fmt: ## Format Go source files
	$(GO) fmt ./...

tidy: ## Tidy Go module dependencies
	$(GO) mod tidy

clean: ## Remove build artifacts
	rm -rf $(dir $(BINARY))

compose-up: ## Start the full compose stack
	NAPCAT_UID=$(NAPCAT_UID) NAPCAT_GID=$(NAPCAT_GID) $(COMPOSE) up -d --build

compose-down: ## Stop local external dependencies
	$(COMPOSE) down

compose-logs: ## Follow Docker Compose logs
	$(COMPOSE) logs -f

mysql: ## Start MySQL only
	$(COMPOSE) up -d mysql

napcat: ## Start NapCat only
	NAPCAT_UID=$(NAPCAT_UID) NAPCAT_GID=$(NAPCAT_GID) $(COMPOSE) up -d napcat

gormgen-install: ## Install the pinned GORM gentool wrapper
	./scripts/install-gentool.sh

gormgen: ## Generate GORM query/model code from MySQL schema
	JXH_GORMGEN_DSN="$(GORMGEN_DSN)" $(GO) generate ./internal/storage
