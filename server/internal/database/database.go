// Package database — подключение к PostgreSQL и миграции БД. Единственное
// место, где приложение знает про драйвер (pgx), про то, как открывается пул
// соединений, и про его жизненный цикл. Остальной код получает готовый
// *pgxpool.Pool через Pool().
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// pool — единственный пул соединений приложения. Создаётся в Init.
var pool *pgxpool.Pool

// Init открывает пул соединений по строке dsn. Пустой dsn означает «БД не
// настроена»: пул не создаётся, Pool() вернёт nil, ошибки нет.
func Init(ctx context.Context, dsn string) error {
	if dsn == "" {
		pool = nil
		return nil
	}
	p, err := open(ctx, dsn)
	if err != nil {
		return err
	}
	pool = p
	return nil
}

// Pool возвращает пул соединений (nil — БД не настроена).
func Pool() *pgxpool.Pool { return pool }

// Close закрывает пул, если он был открыт.
func Close() {
	if pool != nil {
		pool.Close()
		pool = nil
	}
}

// open создаёт пул и проверяет связь (ping).
func open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	// Внутри pgx сам разберётся с контекстом и переподключением.
	p, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("подключение к БД: %w", err)
	}
	if err := p.Ping(ctx); err != nil {
		p.Close()
		return nil, fmt.Errorf("проверка связи с БД: %w", err)
	}
	return p, nil
}
