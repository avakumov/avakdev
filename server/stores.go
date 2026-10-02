package main

import "avakumov/server/internal/store"

// Хранилища БД разделов. Создаются после подключения к БД (см. main → initStores).
var (
	readingStore       *store.Reading
	usersStore         *store.Users
	notificationsStore *store.Notifications
	feedStore          *store.Feed
	booksStore         *store.Books
	dayStore           *store.Day
)

// initStores готовит хранилища разделов. Вызывается после initDB.
func initStores() {
	if db == nil {
		// БД не настроена — хранилища остаются nil, а хендлеры недоступны
		// (authRequired отдаёт 503 до их вызова).
		return
	}
	readingStore = store.NewReading(db)
	usersStore = store.NewUsers(db)
	notificationsStore = store.NewNotifications(db)
	feedStore = store.NewFeed(db)
	booksStore = store.NewBooks(db)
	dayStore = store.NewDay(db)
}
