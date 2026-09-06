package main

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Типы уведомлений: однократное или периодическое.
const (
	notifOnce     = "once"
	notifPeriodic = "periodic"
)

// Каналы доставки: встроенные уведомления или Telegram (пока заглушка).
const (
	notifChannelApp      = "app"
	notifChannelTelegram = "telegram"
)

var validNotifTypes = map[string]bool{
	notifOnce:     true,
	notifPeriodic: true,
}

var validNotifChannels = map[string]bool{
	notifChannelApp:      true,
	notifChannelTelegram: true,
}

// validNotifUnits — единицы измерения периода периодических уведомлений.
var validNotifUnits = map[string]bool{
	"minute": true,
	"hour":   true,
	"day":    true,
	"month":  true,
	"year":   true,
}

// UserNotification — уведомление пользователя (создаёт себе сам).
type UserNotification struct {
	ID          int    `json:"id"`
	Username    string `json:"-"`
	Text        string `json:"text"`
	Type        string `json:"type"`
	DueAt       string `json:"due_at"` // RFC3339 (UTC)
	PeriodUnit  string `json:"period_unit"`
	PeriodValue int    `json:"period_value"`
	Channel     string `json:"channel"`
	Created     string `json:"created"`
	Updated     string `json:"updated"`
}

// notificationStore — хранилище уведомлений пользователей.
type notificationStore struct {
	mu     sync.Mutex
	data   map[int]UserNotification
	nextID int
	hasDB  bool
}

var notifications *notificationStore

// initNotifications инициализирует хранилище и подгружает уведомления из БД.
func initNotifications() error {
	notifications = &notificationStore{
		data:   make(map[int]UserNotification),
		nextID: 1,
		hasDB:  db != nil,
	}
	if !notifications.hasDB {
		return nil
	}

	rows, err := db.Query(context.Background(),
		`SELECT id,
		        username,
		        text,
		        type,
		        to_char(due_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        period_unit,
		        period_value,
		        channel,
		        to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM user_notifications`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var n UserNotification
		if err := rows.Scan(&n.ID, &n.Username, &n.Text, &n.Type,
			&n.DueAt, &n.PeriodUnit, &n.PeriodValue, &n.Channel,
			&n.Created, &n.Updated); err != nil {
			return err
		}
		notifications.data[n.ID] = n
		if n.ID >= notifications.nextID {
			notifications.nextID = n.ID + 1
		}
	}
	return rows.Err()
}

// list возвращает уведомления пользователя, ближайшие по срабатыванию сверху.
func (s *notificationStore) list(username string) []UserNotification {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]UserNotification, 0)
	for _, n := range s.data {
		if n.Username == username {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DueAt < out[j].DueAt })
	return out
}

// create добавляет уведомление.
func (s *notificationStore) create(username, text, ntype, dueAt, periodUnit string, periodValue int, channel string) (UserNotification, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return UserNotification{}, errors.New("укажите текст уведомления")
	}
	if !validNotifTypes[ntype] {
		return UserNotification{}, errors.New("некорректный тип уведомления")
	}
	if !validNotifChannels[channel] {
		return UserNotification{}, errors.New("некорректный канал уведомления")
	}
	if _, err := time.Parse(time.RFC3339, dueAt); err != nil {
		return UserNotification{}, errors.New("некорректная дата срабатывания")
	}
	if ntype == notifPeriodic {
		if !validNotifUnits[periodUnit] {
			return UserNotification{}, errors.New("некорректная единица периода")
		}
		if periodValue < 1 {
			return UserNotification{}, errors.New("укажите период повторения")
		}
	} else {
		periodUnit = ""
		periodValue = 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	n := UserNotification{
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

	if s.hasDB {
		err := db.QueryRow(context.Background(),
			`INSERT INTO user_notifications
			   (username, text, type, due_at, period_unit, period_value, channel)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 RETURNING id,
			           to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			           to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
			username, text, ntype, dueAt, periodUnit, periodValue, channel).
			Scan(&n.ID, &n.Created, &n.Updated)
		if err != nil {
			return UserNotification{}, err
		}
		if n.ID >= s.nextID {
			s.nextID = n.ID + 1
		}
	} else {
		n.ID = s.nextID
		s.nextID++
	}

	s.data[n.ID] = n
	return n, nil
}

// delete удаляет уведомление пользователя.
func (s *notificationStore) delete(username string, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.data[id]
	if !ok || n.Username != username {
		return errors.New("уведомление не найдено")
	}
	if s.hasDB {
		if _, err := db.Exec(context.Background(),
			`DELETE FROM user_notifications WHERE id = $1`, id); err != nil {
			return err
		}
	}
	delete(s.data, id)
	return nil
}

// removeFromMemory удаляет уведомление из кэша (БД уже обновлена отдельно).
func (s *notificationStore) removeFromMemory(id int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
}

// patchDue обновляет в кэше ближайшее срабатывание (БД уже обновлена отдельно).
func (s *notificationStore) patchDue(id int, dueAt string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.data[id]; ok {
		n.DueAt = dueAt
		n.Updated = time.Now().UTC().Format(time.RFC3339)
		s.data[id] = n
	}
}

// handleListNotifications отдаёт уведомления пользователя.
func handleListNotifications(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)
	c.JSON(http.StatusOK, gin.H{"notifications": notifications.list(sessData.username)})
}

// handleCreateNotification создаёт уведомление.
func handleCreateNotification(c *gin.Context) {
	var req struct {
		Text        string `json:"text"`
		Type        string `json:"type"`
		DueAt       string `json:"due_at"`
		PeriodUnit  string `json:"period_unit"`
		PeriodValue int    `json:"period_value"`
		Channel     string `json:"channel"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	if req.Type == "" {
		req.Type = notifOnce
	}
	if req.Channel == "" {
		req.Channel = notifChannelApp
	}
	if req.PeriodValue == 0 {
		req.PeriodValue = 1
	}
	sessData, _ := c.MustGet("session").(session)
	n, err := notifications.create(sessData.username, req.Text, req.Type, req.DueAt, req.PeriodUnit, req.PeriodValue, req.Channel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, n)
}

// handleDeleteNotification удаляет уведомление.
func handleDeleteNotification(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID уведомления"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	if err := notifications.delete(sessData.username, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
