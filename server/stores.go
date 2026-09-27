package main

import "avakumov/server/internal/store"

// Хранилища БД разделов. Создаются после подключения к БД (см. main → initStores).
// Пока сюда переехал только раздел «Чтение»; остальные постепенно переезжают
// с прямых db.Query в main.
var readingStore *store.Reading

// initStores готовит хранилища разделов. Вызывается после initDB.
func initStores() {
	if db == nil {
		// БД не настроена — хранилища остаются nil, а хендлеры недоступны
		// (authRequired отдаёт 503 до их вызова).
		return
	}
	readingStore = store.NewReading(db)
}
