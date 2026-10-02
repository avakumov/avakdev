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
.PHONY: help dev dev-backend dev-frontend install build build-frontend build-binary run \
        run-with-db run-without-db deploy deploy-deps \
        pg-status pg-start pg-stop pg-setup pg-init db-pull

# По умолчанию `make` без аргументов показывает справку
.DEFAULT_GOAL := help

help:
	@echo "Доступные команды:"
	@echo ""
	@echo "  Разработка:"
	@echo "    make dev                  — агент + backend (с БД) + frontend одновременно"
	@echo "    make dev-backend          — агент + только Go-сервер (с БД) на :8080"
	@echo "    make dev-agent            — только агент по задачам (отдельный процесс)"
	@echo "    make dev-frontend         — только Vite dev-сервер на :5173"
	@echo ""
	@echo "  PostgreSQL (системный, без Docker):"
	@echo "    make pg-init              — инициализировать кластер и запустить сервис (sudo)"
	@echo "    make pg-start             — запустить сервис PostgreSQL (sudo)"
	@echo "    make pg-stop              — остановить сервис PostgreSQL (sudo)"
	@echo "    make pg-setup             — создать роль/БД/таблицу users и админа (sudo)"
	@echo "    make pg-status            — статус PostgreSQL и DATABASE_URL"
	@echo "    make db-pull              — скопировать production-базу в локальную (по SSH)"
	@echo ""
	@echo "  Прочее:"
	@echo "    make install              — установить зависимости (go mod tidy + npm install)"
	@echo "    make build                — полная сборка: фронтенд + бинарник (server/server)"
	@echo "    make build-binary         — то же, что make build"
	@echo "    make build-frontend       — только фронтенд в frontend/dist"
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
PG_PORT  ?= 5432
PG_DB    ?= avakumov
# Пароль dev-роли: только из окружения или .env (в git не хранится).
PG_PASS  ?= $(shell grep -E '^PG_PASS=' .env 2>/dev/null | head -n1 | cut -d= -f2-)

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

# Копирует production-базу в локальную: дамп снимается НА сервере по SSH
# (реквизиты из .env, как в deploy.sh), креды прода не покидают сервер.
# Локальная база перезаписывается — скрипт спросит подтверждение.
# Полезные флаги: YES=1 (без вопроса), SCHEMA_ONLY=1 (только структура).
db-pull:
	./scripts/db-pull.sh

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

# Агент по задачам запускается ОТДЕЛЬНЫМ процессом (AVAKUMOV_AGENT=1), чтобы
# air, перезапускающий сервер при изменении файлов, не убивал агента посреди
# задачи (агент сам правит файлы: код, миграции).
AGENT_GO := AVAKUMOV_AGENT=1 go run .

dev: dev-check-db
	$(DEV_GO_WARN)
	@trap 'kill 0' INT TERM; \
	(cd server && $(AGENT_GO)) & \
	(cd server && $(DEV_GO)) & \
	(cd frontend && npm run dev) & \
	wait

dev-backend: dev-check-db
	$(DEV_GO_WARN)
	@trap 'kill 0' INT TERM; \
	(cd server && $(AGENT_GO)) & \
	(cd server && $(DEV_GO)) & \
	wait

# Только агент (без сервера и фронтенда).
dev-agent:
	cd server && $(AGENT_GO)

dev-frontend:
	cd frontend && npm run dev

install:
	cd server && go mod tidy
	cd frontend && npm install

# Фронтенд (vite) → frontend/dist.
build-frontend:
	cd frontend && npm run build

# ---------------------------------------------------------------------------
# Сборка и локальный запуск
# ---------------------------------------------------------------------------
run: dev-check-db
	cd server && go run .

# Бинарник со встроенным фронтендом — server/server. Сначала собирает фронтенд
# (эта же цель — то, что нужно для деплоя).
# nomsgpack — убирает из gin binding поддержку msgpack (тянет ugorji/go/codec,
# ~6 МБ бинарника); само приложение msgpack не использует.
build-binary: build-frontend
	rm -rf server/frontend-dist/assets server/frontend-dist/index.html
	@if [ -f frontend/dist/index.html ]; then \
		cp -r frontend/dist/index.html server/frontend-dist/; \
		cp -r frontend/dist/assets server/frontend-dist/; \
	else \
		echo "Фронтенд не собран. Сначала: make build-frontend"; exit 1; \
	fi
	cd server && CGO_ENABLED=0 go build -trimpath -tags 'embed nomsgpack' -ldflags '-s -w' -o server .

# Полная сборка (для деплоя) — фронтенд + бинарник со встроенной статикой.
build: build-binary

# ---------------------------------------------------------------------------
# Деплой
# ---------------------------------------------------------------------------
deploy:
	./deploy.sh

deploy-deps:
	./deploy.sh --install-deps
