.PHONY: 
	docker-compose-up \
	docker-compose-down \
	docker-compose-purge \
	build \
	run \
	dev \
	test

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

# HELPER
help:
	@echo "Commands:"
	@echo ""
	@echo "Docker Commands:"
	@echo "  make docker-up - Run docker containers"
	@echo "  make docker-down - Stop docker containers"
	@echo "  make docker-purge - Remove docker containers, images, volumnes, networks"
	@echo ""