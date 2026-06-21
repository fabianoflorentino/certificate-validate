# =============================================================================
# Certificate Validate - Makefile
# =============================================================================
# Targets:
#   Development:   build, run, test, test/race, test/cover, lint, fmt, tidy, clean
#   Docker:        docker/build, docker/run, docker/stop, docker/rm
#   Docker Compose: compose/up, compose/down, compose/logs, compose/build,
#                   compose/restart, compose/ps, compose/exec
# =============================================================================

BINARY_NAME    := certificate-validate
MAIN_PACKAGE   := ./cmd/certificate-validate
DOCKER_IMAGE   := fabianoflorentino/certificate-validate
DOCKER_TAG     := latest
DOCKER_COMPOSE := docker compose

GO             := go
CGO_ENABLED    := 0

# ---- Colors for help output ------------------------------------------------

BLUE  := \033[36m
BOLD  := \033[1m
RESET := \033[0m

# ---- Default target --------------------------------------------------------

.DEFAULT_GOAL := help

# ---- Development -----------------------------------------------------------

.PHONY: build
build: ## Build the binary into the project root
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build -ldflags="-s -w" -o $(BINARY_NAME) $(MAIN_PACKAGE)

.PHONY: run
run: build ## Build and run the binary locally (pass args after --, e.g. make run -- --watch)
	./$(BINARY_NAME) $(ARGS)

.PHONY: run/hot
run/hot: ## Run with live reload via go run (useful during development)
	$(GO) run $(MAIN_PACKAGE) $(ARGS)

.PHONY: test
test: ## Run all tests with race detector and count=1 (no cache)
	$(GO) test -race -count=1 ./...

.PHONY: test/short
test/short: ## Run tests without race detector (faster)
	$(GO) test -count=1 ./...

.PHONY: test/cover
test/cover: ## Run tests with coverage and open HTML report
	$(GO) test -race -count=1 -coverprofile=coverage.out ./... && \
		$(GO) tool cover -html=coverage.out -o coverage.html && \
		echo "Coverage report: file://$(PWD)/coverage.html"

.PHONY: test/cover/check
test/cover/check: test/cover ## Run tests with coverage and verify ≥ 80%
	@go tool cover -func=coverage.out | grep total | \
		awk '{print $$3}' | awk -F'%' '{if ($$1 < 80) {printf "FAIL: coverage %.1f%% < 80%%\n", $$1; exit 1} else {printf "PASS: coverage %.1f%%\n", $$1}}'

.PHONY: test/cover/func
test/cover/func: test/cover ## Run tests with coverage and show per-function breakdown
	$(GO) tool cover -func=coverage.out

.PHONY: test/verbose
test/verbose: ## Run tests with verbose output
	$(GO) test -race -count=1 -v ./...

.PHONY: lint
lint: ## Run static analysis with golangci-lint and go vet
	golangci-lint run ./... && \
		$(GO) vet ./...

.PHONY: fmt
fmt: ## Format all Go source code with gofmt
	$(GO) fmt ./...

.PHONY: tidy
tidy: ## Tidy and verify module dependencies
	$(GO) mod tidy && \
		$(GO) mod verify

.PHONY: clean
clean: ## Remove build artifacts and coverage output
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

# ---- Docker ----------------------------------------------------------------

.PHONY: docker/build
docker/build: ## Build the Docker image
	docker build --no-cache --rm \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-f ./Dockerfile .

.PHONY: docker/build/nocache
docker/build/nocache: ## Build the Docker image (alias, identical)
	docker build --no-cache --rm \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-f ./Dockerfile .

.PHONY: docker/run
docker/run: docker/build ## Build and run a ephemeral container (CLI mode)
	docker run --rm -it \
		-v $(PWD)/config:/app/config:ro \
		$(DOCKER_IMAGE):$(DOCKER_TAG) $(ARGS)

.PHONY: docker/run/serve
docker/run/serve: docker/build ## Build and run in API server mode
	docker run --rm -it \
		-p 5000:5000 \
		-v $(PWD)/config:/app/config:ro \
		$(DOCKER_IMAGE):$(DOCKER_TAG) serve

.PHONY: docker/stop
docker/stop: ## Stop the running container
	-docker stop $(BINARY_NAME)

.PHONY: docker/rm
docker/rm: docker/stop ## Remove the container
	-docker rm $(BINARY_NAME)

.PHONY: docker/rmi
docker/rmi: ## Remove the Docker image
	-docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG)

.PHONY: docker/clean
docker/clean: docker/rm docker/rmi ## Remove container and image (full cleanup)

# ---- Docker Compose --------------------------------------------------------

.PHONY: compose/up
compose/up: ## Start services in detached mode
	$(DOCKER_COMPOSE) up -d

.PHONY: compose/up/logs
compose/up/logs: ## Start services with attached logs
	$(DOCKER_COMPOSE) up

.PHONY: compose/down
compose/down: ## Stop and remove containers, networks
	$(DOCKER_COMPOSE) down

.PHONY: compose/down/volumes
compose/down/volumes: ## Stop and remove everything including volumes
	$(DOCKER_COMPOSE) down -v

.PHONY: compose/logs
compose/logs: ## Tail logs from all services
	$(DOCKER_COMPOSE) logs -f

.PHONY: compose/build
compose/build: ## Build or rebuild all services
	$(DOCKER_COMPOSE) build --no-cache

.PHONY: compose/restart
compose/restart: compose/down compose/up ## Restart all services

.PHONY: compose/ps
compose/ps: ## List running services
	$(DOCKER_COMPOSE) ps

.PHONY: compose/exec
compose/exec: ## Execute a command in the running container (e.g. make compose/exec ARGS="sh")
	$(DOCKER_COMPOSE) exec certificate-validate $(ARGS)

.PHONY: compose/health
compose/health: ## Check the health of the running service
	$(DOCKER_COMPOSE) exec certificate-validate wget --quiet --tries=1 --spider http://localhost:5000/api/v1/cert/info/all

# ---- Help ------------------------------------------------------------------

.PHONY: help
help: ## Show this help message
	@awk 'BEGIN {FS = ":.*?## "}; /^[A-Za-z0-9\/_-]+:.*?## / {printf "  \033[36m%-22s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@printf "\n"
	@printf "Examples:\n"
	@printf "  make build              # Build binary\n"
	@printf "  make run                # Build and run\n"
	@printf "  make run ARGS=\"check --watch\" # Build and run with arguments\n"
	@printf "  make test               # Run all tests\n"
	@printf "  make test/cover         # Run tests with coverage HTML report\n"
	@printf "  make lint               # Static analysis\n"
	@printf "  make docker/build       # Build Docker image\n"
	@printf "  make compose/up         # Start Docker Compose services\n"
	@printf "  make compose/logs       # Follow logs\n"
