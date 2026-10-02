// Package notifier — фоновый планировщик доставки уведомлений: в Telegram
// (channel='telegram') и во «входящие» колокольчика (channel='app').
package notifier

import (
	"context"
	"log"
	"time"

	"avakumov/server/internal/app"
)

// tick — как часто проверяем наступившие уведомления.
const tick = 30 * time.Second

// Start запускает фоновую доставку наступивших уведомлений. Без БД (a == nil
// или a.DB == nil) ничего не делает.
func Start(a *app.App) {
	if a == nil || a.DB == nil {
		return
	}
	go func() {
		for range time.Tick(tick) {
			deliverDueTelegram(a)
			deliverDueApp(a)
		}
	}()
}

// deliverDueTelegram отправляет наступившие telegram-уведомления.
func deliverDueTelegram(a *app.App) {
	items, err := a.NotifDB.DueTelegram(context.Background())
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
			if err := a.NotifDB.DeleteByID(context.Background(), n.ID); err != nil {
				log.Printf("УВЕДОМЛЕНИЯ #%d: удаление: %v", n.ID, err)
			}
			a.Notifications.RemoveFromMemory(n.ID)
			continue
		}

		// Периодическое: сдвигаем ближайшее срабатывание.
		due, err := time.Parse(time.RFC3339, n.DueAt)
		if err != nil {
			log.Printf("УВЕДОМЛЕНИЯ #%d: разбор due_at: %v", n.ID, err)
			continue
		}
		next := app.AdvanceDue(due.UTC(), n.Unit, n.Value).Format(time.RFC3339)
		if err := a.NotifDB.SetDue(context.Background(), n.ID, next); err != nil {
			log.Printf("УВЕДОМЛЕНИЯ #%d: сдвиг периода: %v", n.ID, err)
		}
		a.Notifications.PatchDue(n.ID, next)
	}
}

// deliverDueApp кладёт во «входящие» наступившие уведомления канала app
// (по одному на уведомление, пока оно не закрыто).
func deliverDueApp(a *app.App) {
	if err := a.NotifDB.DeliverDueApp(context.Background()); err != nil {
		log.Printf("УВЕДОМЛЕНИЯ: входящие (app): %v", err)
	}
}
