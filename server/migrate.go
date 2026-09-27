package main

import (
	"embed"

	"avakumov/server/internal/database"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// runMigrations применяет версионированные миграции БД (goose) при старте
// сервера. Файлы лежат в server/migrations и встроены в бинарник (//go:embed),
// поэтому отдельный шаг деплоя не нужен: приложение мигрирует само.
//
// Новую миграцию добавляют файлом вида migrations/NNNNN_name.sql с секциями
// "-- +goose Up" и "-- +goose Down" — она применится при следующем старте.
func runMigrations() error {
	// База не настроена (DATABASE_URL пуст) — мигрировать нечего:
	// database.Migrate сама пропускает такой случай.
	return database.Migrate(migrationsFS, "migrations")
}
