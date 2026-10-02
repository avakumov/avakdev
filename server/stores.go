package main

import "avakumov/server/internal/store"

// Хранилища БД разделов. Создаются после подключения к БД (см. main → initStores).
var (
	readingStore       *store.Reading
	draftsStore        *store.Drafts
	bookmarksStore     *store.Bookmarks
	reportsStore       *store.Reports
	usersStore         *store.Users
	notificationsStore *store.Notifications
	feedStore          *store.Feed
)

// initStores готовит хранилища разделов. Вызывается после initDB.
func initStores() {
	if db == nil {
		// БД не настроена — хранилища остаются nil, а хендлеры недоступны
		// (authRequired отдаёт 503 до их вызова).
		return
	}
	readingStore = store.NewReading(db)
	draftsStore = store.NewDrafts(db)
	bookmarksStore = store.NewBookmarks(db)
	reportsStore = store.NewReports(db)
	usersStore = store.NewUsers(db)
	notificationsStore = store.NewNotifications(db)
	feedStore = store.NewFeed(db)
}
