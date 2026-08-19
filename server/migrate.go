package main

import (
	"embed"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
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
	if db == nil {
		// База не настроена (DATABASE_URL пуст) — мигрировать нечего,
		// авторизация и хранилища работают в памяти.
		return nil
	}

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	// Используем тот же пул соединений pgx через database/sql адаптер.
	sqlDB := stdlib.OpenDBFromPool(db)
	defer sqlDB.Close()

	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return err
	}
	return nil
}
