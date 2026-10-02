package main

import (
	"context"
	"os"

	"avakumov/server/internal/database"
)

// initDB подключается к PostgreSQL по строке подключения из DATABASE_URL.
// БД не настроена (пустая переменная) — приложение работает без неё.
func initDB() error {
	return database.Init(context.Background(), os.Getenv("DATABASE_URL"))
}
