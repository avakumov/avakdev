package app

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"avakumov/server/internal/store"
)

// Типы уведомлений: однократное или периодическое.
const (
	NotifOnce     = "once"
	NotifPeriodic = "periodic"
)

// Каналы доставки: встроенные уведомления или Telegram (пока заглушка).
const (
	NotifChannelApp      = "app"
	NotifChannelTelegram = "telegram"
)

var validNotifTypes = map[string]bool{
	NotifOnce:     true,
	NotifPeriodic: true,
}

var validNotifChannels = map[string]bool{
	NotifChannelApp:      true,
	NotifChannelTelegram: true,
}

// validNotifUnits — единицы измерения периода периодических уведомлений.
var validNotifUnits = map[string]bool{
	"minute": true,
	"hour":   true,
	"day":    true,
	"month":  true,
	"year":   true,
}

// NotificationCache — in-memory кэш уведомлений пользователей поверх store.
// Создаёт его один раз при старте и подгружает все уведомления из БД.
type NotificationCache struct {
	mu     sync.Mutex
	data   map[int]store.UserNotification
	nextID int
	db     *store.Notifications
}

// NewNotificationCache создаёт кэш и подгружает уведомления из БД (db == nil —
// работаем без БД).
func NewNotificationCache(db *store.Notifications) (*NotificationCache, error) {
	c := &NotificationCache{
		data:   make(map[int]store.UserNotification),
		nextID: 1,
		db:     db,
	}
	if db == nil {
		return c, nil
	}
	all, err := db.ListAll(context.Background())
	if err != nil {
		return nil, err
	}
	for _, n := range all {
		c.data[n.ID] = n
		if n.ID >= c.nextID {
			c.nextID = n.ID + 1
		}
	}
	return c, nil
}

// List возвращает уведомления пользователя, ближайшие по срабатыванию сверху.
func (c *NotificationCache) List(username string) []store.UserNotification {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]store.UserNotification, 0)
	for _, n := range c.data {
		if n.Username == username {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DueAt < out[j].DueAt })
	return out
}

// Create проверяет и добавляет уведомление.
func (c *NotificationCache) Create(username, text, ntype, dueAt, periodUnit string, periodValue int, channel string) (store.UserNotification, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return store.UserNotification{}, errors.New("укажите текст уведомления")
	}
	if !validNotifTypes[ntype] {
		return store.UserNotification{}, errors.New("некорректный тип уведомления")
	}
	if !validNotifChannels[channel] {
		return store.UserNotification{}, errors.New("некорректный канал уведомления")
	}
	if _, err := time.Parse(time.RFC3339, dueAt); err != nil {
		return store.UserNotification{}, errors.New("некорректная дата срабатывания")
	}
	if ntype == NotifPeriodic {
		if !validNotifUnits[periodUnit] {
			return store.UserNotification{}, errors.New("некорректная единица периода")
		}
		if periodValue < 1 {
			return store.UserNotification{}, errors.New("укажите период повторения")
		}
	} else {
		periodUnit = ""
		periodValue = 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	n := store.UserNotification{
		Username:    username,
		Text:        text,
		Type:        ntype,
		DueAt:       dueAt,
		PeriodUnit:  periodUnit,
		PeriodValue: periodValue,
		Channel:     channel,
		Created:     now,
		Updated:     now,
	}

	if c.db != nil {
		created, err := c.db.Create(context.Background(), n)
		if err != nil {
			return store.UserNotification{}, err
		}
		n = created
		if n.ID >= c.nextID {
			c.nextID = n.ID + 1
		}
	} else {
		n.ID = c.nextID
		c.nextID++
	}

	c.data[n.ID] = n
	return n, nil
}

// Delete удаляет уведомление пользователя.
func (c *NotificationCache) Delete(username string, id int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	n, ok := c.data[id]
	if !ok || n.Username != username {
		return errors.New("уведомление не найдено")
	}
	if c.db != nil {
		if err := c.db.DeleteByID(context.Background(), id); err != nil {
			return err
		}
	}
	delete(c.data, id)
	return nil
}

// RemoveFromMemory удаляет уведомление из кэша (БД уже обновлена отдельно).
func (c *NotificationCache) RemoveFromMemory(id int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, id)
}

// PatchDue обновляет в кэше ближайшее срабатывание (БД уже обновлена отдельно).
func (c *NotificationCache) PatchDue(id int, dueAt string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n, ok := c.data[id]; ok {
		n.DueAt = dueAt
		n.Updated = time.Now().UTC().Format(time.RFC3339)
		c.data[id] = n
	}
}

// AdvanceDue сдвигает ближайшее срабатывание на следующий период.
// Для месяц/год — календарный сдвиг с ограничением дня (31 янв + 1 мес -> 28/29 фев).
func AdvanceDue(due time.Time, unit string, value int) time.Time {
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
