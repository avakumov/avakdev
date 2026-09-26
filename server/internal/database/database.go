// Package database — подключение к PostgreSQL и миграции БД. Единственное
// место, где приложение знает про драйвер (pgx) и про то, как открывается пул
// соединений. Остальной код получает уже готовый *pgxpool.Pool.
package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open создаёт пул соединений с PostgreSQL и проверяет связь (ping).
// Пустой dsn считается ошибкой: решение «БД не настроена» принимает
// вызывающий код (см. initDB в main).
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	if dsn == "" {
		return nil, errors.New("пустой DATABASE_URL")
	}
	// Внутри pgx сам разберётся с контекстом и переподключением.
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("подключение к БД: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("проверка связи с БД: %w", err)
	}
	return pool, nil
}
