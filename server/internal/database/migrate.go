package database

import (
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// Migrate применяет версионированные миграции (goose) из fsys — в каталоге dir.
// Встроенные миграции передаёт вызывающий код (//go:embed в main), чтобы файлы
// оставались рядом с пакетом main.
//
// БД не настроена (пул не создан) — миграции пропускаются.
func Migrate(fsys fs.FS, dir string) error {
	if pool == nil {
		return nil
	}
	goose.SetBaseFS(fsys)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	// Тот же пул pgx через database/sql-адаптер: goose работает с *sql.DB.
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	if err := goose.Up(sqlDB, dir); err != nil {
		return fmt.Errorf("миграции: %w", err)
	}
	return nil
}
