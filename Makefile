COMPOSE_FILE=docker-compose.yaml
DOCKER_COMPOSE=docker compose
META_COVERAGE=90
PERFIL_COVERAGE=markupp/cover.out

.PHONY: all test smoke-kind kind-up kind-painel kind-down compose-config compose-env docker-up docker-down run hooks test-scripts coverage coverage-check

all: compose-env compose-config test

# Roda todos os testes Go do servidor no host, que precisa de Go e de Docker
# para o testcontainers subir o PostgreSQL de teste
test:
	cd markupp && go test ./...

# Smoke descartável da stack dev num kind com os requisitos do cluster de produção.
# Precisa de Docker, kind, helm, kubectl e Pulumi, e apaga o cluster no fim
smoke-kind:
	./deploy/kind/ambiente.sh smoke

# Sobe o mesmo ambiente com o painel Headlamp e deixa de pé para acompanhar
kind-up:
	./deploy/kind/ambiente.sh up

# Abre o Headlamp do ambiente de pé em http://localhost:4466
kind-painel:
	./deploy/kind/ambiente.sh painel

# Apaga o ambiente de pé
kind-down:
	./deploy/kind/ambiente.sh down

# Valida o arquivo docker-compose e a interpolação de variáveis de ambiente
compose-config:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) config

# Configura variáveis de ambiente no ambiente do Compose
compose-env:
	@if [ ! -f .env ]; then \
		printf 'MARKUPP_PORT=8080\nPOSTGRES_VOLUME=postgres_data\nGO_MOD_CACHE=go_mod_cache\n' > .env; \
	fi
	@echo ".env criado/atualizado com sucesso."

# Sobe o container Docker do servidor em modo destacado
docker-up:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) up -d --build markupp

# Desce e remove o container Docker usado nos testes
docker-down:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) down

# Roda a aplicação completa com air via Docker
run: compose-env compose-config
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) up --build markupp

# Instala os hooks de pre-commit neste clone
hooks:
	pre-commit install

# Roda os testes dos scripts de apoio
test-scripts:
	./scripts/valida-mensagem-de-commit-test.sh
	./scripts/verifica-coverage-test.sh

# Mede o coverage do servidor e checa a meta minima
coverage:
	cd markupp && go test ./... -coverprofile=../$(PERFIL_COVERAGE) -covermode=atomic
	$(MAKE) coverage-check

# Checa um coverprofile ja existente contra a meta minima
coverage-check:
	./scripts/verifica-coverage.sh $(PERFIL_COVERAGE) $(META_COVERAGE)
