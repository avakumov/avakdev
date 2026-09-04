package main

import (
	"context"
	"log"
	"time"
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
	rows, err := db.Query(context.Background(),
		`SELECT n.id, n.text, n.type, n.period_unit, n.period_value,
		        to_char(n.due_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        u.telegram_chat_id
		 FROM user_notifications n
		 JOIN users u ON u.username = n.username
		 WHERE n.channel = 'telegram'
		   AND n.due_at <= now()
		   AND u.telegram_chat_id <> ''`)
	if err != nil {
		log.Printf("УВЕДОМЛЕНИЯ: выборка: %v", err)
		return
	}
	defer rows.Close()

	type dueNotif struct {
		id     int
		text   string
		ntype  string
		unit   string
		value  int
		dueAt  string
		chatID string
	}
	items := make([]dueNotif, 0)
	for rows.Next() {
		var n dueNotif
		if err := rows.Scan(&n.id, &n.text, &n.ntype, &n.unit, &n.value,
			&n.dueAt, &n.chatID); err != nil {
			log.Printf("УВЕДОМЛЕНИЯ: чтение: %v", err)
			return
		}
		items = append(items, n)
	}
	if err := rows.Err(); err != nil {
		log.Printf("УВЕДОМЛЕНИЯ: чтение: %v", err)
		return
	}

	for _, n := range items {
		msg := "🔔 " + n.text
		if err := tgSendMessage(n.chatID, msg); err != nil {
			log.Printf("УВЕДОМЛЕНИЯ #%d: отправка в Telegram: %v", n.id, err)
			continue // попробуем в следующий тик
		}
		log.Printf("УВЕДОМЛЕНИЯ #%d: отправлено в Telegram", n.id)

		if n.ntype == notifOnce {
			if _, err := db.Exec(context.Background(),
				`DELETE FROM user_notifications WHERE id = $1`, n.id); err != nil {
				log.Printf("УВЕДОМЛЕНИЯ #%d: удаление: %v", n.id, err)
			}
			continue
		}

		// Периодическое: сдвигаем ближайшее срабатывание.
		due, err := time.Parse(time.RFC3339, n.dueAt)
		if err != nil {
			log.Printf("УВЕДОМЛЕНИЯ #%d: разбор due_at: %v", n.id, err)
			continue
		}
		next := advanceDue(due.UTC(), n.unit, n.value).Format(time.RFC3339)
		if _, err := db.Exec(context.Background(),
			`UPDATE user_notifications SET due_at = $2, updated = now() WHERE id = $1`,
			n.id, next); err != nil {
			log.Printf("УВЕДОМЛЕНИЯ #%d: сдвиг периода: %v", n.id, err)
		}
	}
}
