package database

import (
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// migrationsFS — встроенные SQL-миграции (goose). Путь задан относительно этого
// файла, поэтому каталог migrations/ лежит рядом с пакетом database.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate применяет версионированные миграции (goose) из встроенного каталога
// migrations. Файлы встроены в бинарник (//go:embed), поэтому отдельный шаг
// деплоя не нужен: приложение мигрирует само.
//
// Новую миграцию добавляют файлом вида migrations/NNNNN_name.sql с секциями
// "-- +goose Up" и "-- +goose Down" — она применится при следующем старте.
//
// БД не настроена (пул не создан) — миграции пропускаются.
func Migrate() error {
	if pool == nil {
		return nil
	}
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	// Тот же пул pgx через database/sql-адаптер: goose работает с *sql.DB.
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("миграции: %w", err)
	}
	return nil
}
