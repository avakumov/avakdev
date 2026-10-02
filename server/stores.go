package main

import "avakumov/server/internal/store"

// Хранилища БД разделов. Создаются после подключения к БД (см. main → initStores).
var (
	dayStore *store.Day
)

// initStores готовит хранилища разделов. Вызывается после initDB.
func initStores() {
	if db == nil {
		// БД не настроена — хранилища остаются nil, а хендлеры недоступны
		// (authRequired отдаёт 503 до их вызова).
		return
	}
	dayStore = store.NewDay(db)
}
