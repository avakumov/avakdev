# Makefile для разработки и деплоя
#
# Локальная разработка с системным PostgreSQL:
#   make pg-status        — статус локального PostgreSQL + DATABASE_URL
#   make pg-start         — запустить PostgreSQL (первый раз: init + setup)
#   make dev              — запустить backend (с БД) и frontend одновременно
#   make dev-backend      — только Go-сервер с подключением к БД

# ---------------------------------------------------------------------------
# Конфигурация
# ---------------------------------------------------------------------------
.PHONY: help dev dev-backend dev-frontend install build build-binary run \
        run-with-db run-without-db deploy deploy-deps \
        pg-status pg-start pg-stop pg-setup pg-init

# По умолчанию `make` без аргументов показывает справку
.DEFAULT_GOAL := help

help:
	@echo "Доступные команды:"
	@echo ""
	@echo "  Разработка:"
	@echo "    make dev                  — backend (с БД) + frontend одновременно"
	@echo "    make dev-backend          — только Go-сервер (с БД) на :8080"
	@echo "    make dev-frontend         — только Vite dev-сервер на :5173"
	@echo ""
	@echo "  PostgreSQL (системный, без Docker):"
	@echo "    make pg-init              — инициализировать кластер и запустить сервис (sudo)"
	@echo "    make pg-start             — запустить сервис PostgreSQL (sudo)"
	@echo "    make pg-stop              — остановить сервис PostgreSQL (sudo)"
	@echo "    make pg-setup             — создать роль/БД/таблицу users и админа (sudo)"
	@echo "    make pg-status            — статус PostgreSQL и DATABASE_URL"
	@echo ""
	@echo "  Прочее:"
	@echo "    make install              — установить зависимости (go mod tidy + npm install)"
	@echo "    make build                — собрать фронтенд в frontend/dist"
	@echo "    make build-binary         — собрать бинарник со встроенным фронтендом"
	@echo "    make run                  — «прод»-запуск: Go отдаёт статику и API на :8080"
	@echo ""
	@echo "  Деплой:"
	@echo "    make deploy               — собрать и задеплоить через ./deploy.sh"
	@echo "    make deploy-deps          — то же + установить Caddy и служебного юзера"
	@echo ""
	@echo "Например: make dev"

# ---------------------------------------------------------------------------
# Локальная база данных (системный PostgreSQL). Переменные можно переопределить
# извне, например: make dev PG_PORT=5434 PG_USER=foo
# ---------------------------------------------------------------------------
PG_USER  ?= avakumov
PG_PASS  ?= 2d38869aeef2a8c628edce3903a94a09
PG_PORT  ?= 5432
PG_DB    ?= avakumov

# Строка подключения, используемая dev/run. Если DATABASE_URL уже задана
# в окружении (или в .env), берём её; иначе собираем из параметров выше.
ifndef DATABASE_URL
  DATABASE_URL_FILE := $(shell grep -E '^DATABASE_URL=' .env 2>/dev/null | head -n1 | cut -d= -f2-)
  ifdef DATABASE_URL_FILE
    DATABASE_URL := $(strip $(DATABASE_URL_FILE))
  else
    DATABASE_URL := postgres://$(PG_USER):$(PG_PASS)@127.0.0.1:$(PG_PORT)/$(PG_DB)?sslmode=disable
  endif
endif
export DATABASE_URL

# Проверка, что параметры PG не были переопределены вразрез с файлом .env
dev-check-db:
	@echo "DATABASE_URL: $(DATABASE_URL)"
	@if ! (echo > /dev/tcp/127.0.0.1/$(PG_PORT)) 2>/dev/null; then \
		echo "(!) PostgreSQL на порту $(PG_PORT) не отвечает."; \
		echo "    Запустите его: sudo systemctl enable --now postgresql  (или: make pg-start)"; \
		exit 1; \
	fi

# ---------------------------------------------------------------------------
# PostgreSQL-таргеты (обёртки над scripts/dev-pg.sh)
# ---------------------------------------------------------------------------
pg-status:
	./scripts/dev-pg.sh status

pg-init:
	./scripts/dev-pg.sh init

pg-start:
	./scripts/dev-pg.sh start

pg-stop:
	./scripts/dev-pg.sh stop

pg-setup:
	./scripts/dev-pg.sh setup

# ---------------------------------------------------------------------------
# Разработка
# ---------------------------------------------------------------------------
# Go-сервер в dev: если установлен air — горячая перезагрузка при изменении
# .go/.sql файлов (см. server/.air.toml); иначе обычный `go run .` без
# авто-рестарта. Установка air: go install github.com/air-verse/air@latest
ifneq ($(shell command -v air 2>/dev/null),)
  DEV_GO := air
else
  DEV_GO := go run .
  DEV_GO_WARN := @echo "(!) air не установлен — Go-сервер без авто-перезагрузки. Установка: go install github.com/air-verse/air@latest"
endif

dev: dev-check-db
	$(DEV_GO_WARN)
	@trap 'kill 0' INT TERM; \
	(cd server && $(DEV_GO)) & \
	(cd frontend && npm run dev) & \
	wait

dev-backend: dev-check-db
	$(DEV_GO_WARN)
	cd server && $(DEV_GO)

dev-frontend:
	cd frontend && npm run dev

install:
	cd server && go mod tidy
	cd frontend && npm install

build:
	cd frontend && npm run build

# ---------------------------------------------------------------------------
# Сборка и локальный запуск
# ---------------------------------------------------------------------------
run: dev-check-db
	cd server && go run .

# Собирает один исполняемый файл со встроенным фронтендом.
# (embed-сборка требует, чтобы server/frontend-dist содержал собранную статику)
build-binary:
	rm -rf server/frontend-dist/assets server/frontend-dist/index.html
	@if [ -f frontend/dist/index.html ]; then \
		cp -r frontend/dist/index.html server/frontend-dist/; \
		cp -r frontend/dist/assets server/frontend-dist/; \
	else \
		echo "Фронтенд не собран. Сначала: make build"; exit 1; \
	fi
	cd server && CGO_ENABLED=0 go build -trimpath -tags embed -ldflags '-s -w' -o server .

# ---------------------------------------------------------------------------
# Деплой
# ---------------------------------------------------------------------------
deploy:
	./deploy.sh

deploy-deps:
	./deploy.sh --install-deps
