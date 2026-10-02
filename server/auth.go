package main

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"avakumov/server/internal/app"
	"avakumov/server/internal/database"
)

// session — активная сессия пользователя (определение живёт в app).
type session = app.Session

var (
	db          *pgxpool.Pool
	application *app.App
)

// initDB подключается к PostgreSQL по строке подключения из DATABASE_URL.
// БД не настроена (пустая переменная) — приложение работает без неё.
func initDB() error {
	if err := database.Init(context.Background(), os.Getenv("DATABASE_URL")); err != nil {
		return err
	}
	// Ссылка на пул для хендлеров. Постепенно её вытесняют хранилища (store).
	db = database.Pool()
	return nil
}

// logAuthConfig печатает состояние подключения к БД при старте.
func logAuthConfig() {
	if db == nil {
		log.Println("AUTH: база данных не настроена (DATABASE_URL пуст). Авторизация отключена.")
		return
	}
	log.Println("AUTH: подключение к PostgreSQL установлено. Авторизация включена.")
}
