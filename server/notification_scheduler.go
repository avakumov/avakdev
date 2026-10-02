package main

import (
	"context"
	"log"
	"time"

	"avakumov/server/internal/app"
)

// notificationTick — как часто проверяем наступившие уведомления.
const notificationTick = 30 * time.Second

// startNotificationScheduler запускает доставку наступивших уведомлений:
// в Telegram (channel='telegram') и во «входящие» колокольчика (channel='app').
func startNotificationScheduler() {
	if db == nil {
		return
	}
	go func() {
		for range time.Tick(notificationTick) {
			deliverDueTelegramNotifications()
			deliverDueAppNotifications()
		}
	}()
}

// deliverDueTelegramNotifications отправляет наступившие telegram-уведомления.
func deliverDueTelegramNotifications() {
	items, err := application.NotifDB.DueTelegram(context.Background())
	if err != nil {
		log.Printf("УВЕДОМЛЕНИЯ: выборка: %v", err)
		return
	}

	for _, n := range items {
		msg := "🔔 " + n.Text
		if err := app.TelegramSendMessage(n.ChatID, msg); err != nil {
			log.Printf("УВЕДОМЛЕНИЯ #%d: отправка в Telegram: %v", n.ID, err)
			continue // попробуем в следующий тик
		}
		log.Printf("УВЕДОМЛЕНИЯ #%d: отправлено в Telegram", n.ID)

		if n.Type == app.NotifOnce {
			if err := application.NotifDB.DeleteByID(context.Background(), n.ID); err != nil {
				log.Printf("УВЕДОМЛЕНИЯ #%d: удаление: %v", n.ID, err)
			}
			application.Notifications.RemoveFromMemory(n.ID)
			continue
		}

		// Периодическое: сдвигаем ближайшее срабатывание.
		due, err := time.Parse(time.RFC3339, n.DueAt)
		if err != nil {
			log.Printf("УВЕДОМЛЕНИЯ #%d: разбор due_at: %v", n.ID, err)
			continue
		}
		next := app.AdvanceDue(due.UTC(), n.Unit, n.Value).Format(time.RFC3339)
		if err := application.NotifDB.SetDue(context.Background(), n.ID, next); err != nil {
			log.Printf("УВЕДОМЛЕНИЯ #%d: сдвиг периода: %v", n.ID, err)
		}
		application.Notifications.PatchDue(n.ID, next)
	}
}

// deliverDueAppNotifications кладёт во «входящие» наступившие уведомления
// канала app (по одному на уведомление, пока оно не закрыто).
func deliverDueAppNotifications() {
	if err := application.NotifDB.DeliverDueApp(context.Background()); err != nil {
		log.Printf("УВЕДОМЛЕНИЯ: входящие (app): %v", err)
	}
}
