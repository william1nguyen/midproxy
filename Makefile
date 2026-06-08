.PHONY:
	docker-up \
	docker-down \
	docker-purge \
	build \
	run \
	dev \
	test \
	solver \
	solver-dev

# DOCKER
DOCKER_COMPOSE_YAML = ./docker-compose.yml

docker-up:
	docker compose -f $(DOCKER_COMPOSE_YAML) -p midproxy up -d

docker-down:
	docker compose -f $(DOCKER_COMPOSE_YAML) -p midproxy down

docker-purge:
	docker compose -f $(DOCKER_COMPOSE_YAML) -p midproxy down --rmi local -v

docker: docker-compose-up

# PROXY
CMD = ./cmd/proxy/main.go
BIN = ./bin/proxy

build:
	go build -o $(BIN) $(CMD)

run: build
	$(BIN)

dev:
	go run $(CMD)

test:
	go test ./... -v

# SOLVER
SOLVER_DIR = ./solver

solver:
	cd $(SOLVER_DIR) && pnpm start

solver-dev:
	cd $(SOLVER_DIR) && pnpm dev

# HELPER
help:
	@echo "Commands:"
	@echo ""
	@echo "Proxy:"
	@echo "  make build      - Build proxy binary"
	@echo "  make run         - Build and run proxy"
	@echo "  make dev         - Run proxy with go run"
	@echo "  make test        - Run tests"
	@echo ""
	@echo "Solver:"
	@echo "  make solver      - Start solver service (headless)"
	@echo "  make solver-dev  - Start solver service (headful)"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-up    - Run docker containers"
	@echo "  make docker-down  - Stop docker containers"
	@echo "  make docker-purge - Remove containers, images, volumes"
	@echo ""