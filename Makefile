# Makefile для разработки и деплоя

.PHONY: help dev dev-backend dev-frontend install build build-binary deploy deploy-deps run

# По умолчанию `make` без аргументов показывает справку
.DEFAULT_GOAL := help

help:
	@echo "Доступные команды:"
	@echo ""
	@echo "  make dev            — запустить backend и frontend одновременно"
	@echo "                        (Ctrl+C останавливает оба процесса)"
	@echo "  make dev-backend    — только Go-сервер (http://localhost:8080)"
	@echo "  make dev-frontend   — только Vite dev-сервер (http://localhost:5173)"
	@echo "  make install        — установить зависимости (go mod tidy + npm install)"
	@echo "  make build          — собрать фронтенд в frontend/dist"
	@echo "  make build-binary   — собрать один бинарник со встроенным фронтендом"
	@echo "                        (embed + build tag) в server/server"
	@echo "  make run            — «прод»-запуск: Go отдаёт статику и API на :8080"
	@echo ""
	@echo "Деплой (на текущей машине или VPS):"
	@echo "  make deploy         — собрать и задеплоить через ./deploy.sh"
	@echo "  make deploy-deps    — то же + установить Caddy и служебного юзера"
	@echo ""
	@echo "Например: make dev"

# Оба процесса в одном терминале; Ctrl+C останавливает всё разом
dev:
	@trap 'kill 0' INT TERM; \
	(cd server && go run .) & \
	(cd frontend && npm run dev) & \
	wait

dev-backend:
	cd server && go run .

dev-frontend:
	cd frontend && npm run dev

install:
	cd server && go mod tidy
	cd frontend && npm install

build:
	cd frontend && npm run build

run:
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

# Полный деплой на текущую машину (systemd + Caddy)
deploy:
	./deploy.sh

deploy-deps:
	./deploy.sh --install-deps
