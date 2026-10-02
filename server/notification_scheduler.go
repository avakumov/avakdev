package main

import (
	"context"
	"log"
	"time"

	"avakumov/server/internal/app"
)

// notificationTick — как часто проверяем наступившие уведомления.
const notificationTick = 30 * time.Second

// advanceDue сдвигает ближайшее срабатывание на следующий период.
// Для месяц/год — календарный сдвиг с ограничением дня (31 янв + 1 мес -> 28/29 фев).
func advanceDue(due time.Time, unit string, value int) time.Time {
	switch unit {
	case "minute":
		return due.Add(time.Duration(value) * time.Minute)
	case "hour":
		return due.Add(time.Duration(value) * time.Hour)
	case "day":
		return due.AddDate(0, 0, value)
	case "month":
		return addCalendarClamped(due, value, 0)
	case "year":
		return addCalendarClamped(due, value*12, 0)
	default:
		return due.AddDate(0, 0, value)
	}
}

// addCalendarClamped добавляет месяцы, не выходя за последний день месяца.
func addCalendarClamped(t time.Time, months int, _ int) time.Time {
	y, m, d := t.Date()
	hh, mm, ss := t.Clock()
	// Последний день целевого месяца: 1-е число следующего месяца минус 1 день.
	first := time.Date(y, time.Month(int(m)+months), 1, hh, mm, ss, t.Nanosecond(), t.Location())
	lastDay := first.AddDate(0, 1, -1).Day()
	if d > lastDay {
		d = lastDay
	}
	return time.Date(y, time.Month(int(m)+months), d, hh, mm, ss, t.Nanosecond(), t.Location())
}

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
	items, err := notificationsStore.DueTelegram(context.Background())
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

		if n.Type == notifOnce {
			if err := notificationsStore.DeleteByID(context.Background(), n.ID); err != nil {
				log.Printf("УВЕДОМЛЕНИЯ #%d: удаление: %v", n.ID, err)
			}
			notifications.removeFromMemory(n.ID)
			continue
		}

		// Периодическое: сдвигаем ближайшее срабатывание.
		due, err := time.Parse(time.RFC3339, n.DueAt)
		if err != nil {
			log.Printf("УВЕДОМЛЕНИЯ #%d: разбор due_at: %v", n.ID, err)
			continue
		}
		next := advanceDue(due.UTC(), n.Unit, n.Value).Format(time.RFC3339)
		if err := notificationsStore.SetDue(context.Background(), n.ID, next); err != nil {
			log.Printf("УВЕДОМЛЕНИЯ #%d: сдвиг периода: %v", n.ID, err)
		}
		notifications.patchDue(n.ID, next)
	}
}
