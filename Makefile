COMPOSE_FILE=docker-compose.yaml
MARKUPP_WORKDIR=/src
DB_CONFIG_PKG=./internal/storage
DOCKER_COMPOSE=docker compose

.PHONY: all test-db-config compose-config compose-env docker-up docker-test docker-down run hooks test-hooks

all: compose-env compose-config docker-up docker-test docker-down

test: docker-test

# Testa o módulo de banco de dados do servidor dentro de um contêiner Docker
test-db-config:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) run --rm markupp sh -c "cd $(MARKUPP_WORKDIR) && go test $(DB_CONFIG_PKG)"

# Valida o arquivo docker-compose e a interpolação de variáveis de ambiente
compose-config:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) config

# Configura variáveis de ambiente no ambiente do Compose
compose-env:
	@if [ ! -f .env ]; then \
		printf 'MARKUPP_PORT=8080\nDATA_VOLUME=markupp_data\nGO_MOD_CACHE=go_mod_cache\n' > .env; \
	fi
	@echo ".env criado/atualizado com sucesso."

# Sobe o container Docker do servidor em modo destacado
docker-up:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) up -d --build markupp

# Executa todos os testes Go do servidor dentro do container Docker em execução
docker-test:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) run --rm markupp sh -c "cd $(MARKUPP_WORKDIR) && go test ./..."

# Desce e remove o container Docker usado nos testes
docker-down:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) down

# Roda a aplicação completa com air via Docker
run: compose-env compose-config
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) up --build markupp

# Instala os hooks de pre-commit neste clone
hooks:
	pre-commit install

# Roda os testes dos scripts de hook
test-hooks:
	./scripts/valida-mensagem-de-commit-test.sh
