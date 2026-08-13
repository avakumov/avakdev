# Простой Makefile для разработки

.PHONY: help dev dev-backend dev-frontend install build run

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
	@echo "  make run            — «прод»-запуск: Go отдаёт статику и API на :8080"
	@echo "  make help           — показать эту справку"
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

