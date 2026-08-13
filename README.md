# Go (Gin) + React

Простое приложение: бэкенд на **Go (Gin)**, фронтенд на **React (Vite)**.
React-приложение работает **через Go-сервер**: он раздаёт собранный фронтенд и обслуживает API.

## Структура

```
├── server/    # Go-бэкенд (Gin): API + раздача статики React
├── frontend/  # React-приложение (Vite)
└── README.md
```

## Быстрый старт

### 1. Запустить бэкенд

```bash
cd server
go mod tidy
go run .
# Сервер: http://localhost:8080
```

### 2. Разработка фронтенда (hot-reload)

```bash
cd frontend
npm install
npm run dev
# Vite: http://localhost:5173
# Запросы /api проксируются на Go-сервер (см. vite.config.js)
```

### 3. Сборка и запуск «по-настоящему» через Go

```bash
cd frontend
npm run build        # соберёт фронтенд в frontend/dist

cd ../server
go run .             # Go-сервер отдаёт и страницу, и API на :8080
```

Откройте http://localhost:8080 — фронтенд отдаётся Go-сервером,
а данные загружаются с `/api/health` и `/api/message`.

## API

| Метод | Путь          | Описание            |
|-------|---------------|---------------------|
| GET   | `/api/health` | Статус сервера      |
| GET   | `/api/message`| Приветственное сообщение |

## Переменные окружения

- `PORT` — порт сервера (по умолчанию `8080`)
- `GIN_MODE=release` — release-режим Gin
