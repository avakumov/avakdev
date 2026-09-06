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

// Статусы целей.
const (
	goalActive    = "active"
	goalPaused    = "paused"
	goalAchieved  = "achieved"
	goalCancelled = "cancelled"
)

var validGoalStatuses = map[string]bool{
	goalActive:    true,
	goalPaused:    true,
	goalAchieved:  true,
	goalCancelled: true,
}

// Goal — цель раздела «Цели»: результат с дедлайном, статусом и прогрессом.
type Goal struct {
	ID          int    `json:"id"`
	Username    string `json:"-"`
	Title       string `json:"title"`
	Description string `json:"description"`
	TargetDate  string `json:"target_date"` // YYYY-MM-DD или пусто
	Status      string `json:"status"`
	Progress    int    `json:"progress"` // 0..100
	Created     string `json:"created"`
	Updated     string `json:"updated"`
}

// goalStore — хранилище целей.
type goalStore struct {
	mu     sync.Mutex
	data   map[int]Goal
	nextID int
	hasDB  bool
}

var goals *goalStore

// initGoals инициализирует хранилище целей и подгружает их из БД.
func initGoals() error {
	goals = &goalStore{
		data:   make(map[int]Goal),
		nextID: 1,
		hasDB:  db != nil,
	}
	if !goals.hasDB {
		return nil
	}

	rows, err := db.Query(context.Background(),
		`SELECT id,
		        username,
		        title,
		        description,
		        COALESCE(to_char(target_date,'YYYY-MM-DD'),''),
		        status,
		        progress,
		        to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM goals`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var g Goal
		if err := rows.Scan(&g.ID, &g.Username, &g.Title, &g.Description,
			&g.TargetDate, &g.Status, &g.Progress, &g.Created, &g.Updated); err != nil {
			return err
		}
		goals.data[g.ID] = g
		if g.ID >= goals.nextID {
			goals.nextID = g.ID + 1
		}
	}
	return rows.Err()
}

// list возвращает цели пользователя, новые сверху.
func (s *goalStore) list(username string) []Goal {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Goal, 0)
	for _, g := range s.data {
		if g.Username == username {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

// getOwned возвращает цель, если она принадлежит пользователю.
func (s *goalStore) getOwned(username string, id int) (Goal, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.data[id]
	if !ok || g.Username != username {
		return Goal{}, false
	}
	return g, true
}

// create добавляет новую цель.
func (s *goalStore) create(username, title, description, targetDate, status string, progress int) (Goal, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Goal{}, errors.New("укажите название цели")
	}
	if !validGoalStatuses[status] {
		return Goal{}, errors.New("некорректный статус цели")
	}
	if progress < 0 || progress > 100 {
		return Goal{}, errors.New("прогресс должен быть от 0 до 100")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	g := Goal{
		Username:    username,
		Title:       title,
		Description: description,
		TargetDate:  targetDate,
		Status:      status,
		Progress:    progress,
		Created:     now,
		Updated:     now,
	}

	if s.hasDB {
		var dl interface{}
		if targetDate != "" {
			dl = targetDate
		}
		err := db.QueryRow(context.Background(),
			`INSERT INTO goals (username, title, description, target_date, status, progress)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 RETURNING id,
			           to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			           to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
			username, title, description, dl, status, progress).
			Scan(&g.ID, &g.Created, &g.Updated)
		if err != nil {
			return Goal{}, err
		}
		if g.ID >= s.nextID {
			s.nextID = g.ID + 1
		}
	} else {
		g.ID = s.nextID
		s.nextID++
	}

	s.data[g.ID] = g
	return g, nil
}

// update обновляет цель.
func (s *goalStore) update(username string, id int, title, description, targetDate, status string, progress int) (Goal, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Goal{}, errors.New("укажите название цели")
	}
	if !validGoalStatuses[status] {
		return Goal{}, errors.New("некорректный статус цели")
	}
	if progress < 0 || progress > 100 {
		return Goal{}, errors.New("прогресс должен быть от 0 до 100")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.data[id]
	if !ok || g.Username != username {
		return Goal{}, errors.New("цель не найдена")
	}

	g.Title = title
	g.Description = description
	g.TargetDate = targetDate
	g.Status = status
	g.Progress = progress
	g.Updated = time.Now().UTC().Format(time.RFC3339)

	if s.hasDB {
		var dl interface{}
		if targetDate != "" {
			dl = targetDate
		}
		if _, err := db.Exec(context.Background(),
			`UPDATE goals
			 SET title = $2, description = $3, target_date = $4,
			     status = $5, progress = $6, updated = now()
			 WHERE id = $1`,
			id, g.Title, g.Description, dl, g.Status, g.Progress); err != nil {
			return Goal{}, err
		}
	}

	s.data[id] = g
	return g, nil
}

// delete удаляет цель пользователя.
func (s *goalStore) delete(username string, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.data[id]
	if !ok || g.Username != username {
		return errors.New("цель не найдена")
	}
	if s.hasDB {
		if _, err := db.Exec(context.Background(),
			`DELETE FROM goals WHERE id = $1`, id); err != nil {
			return err
		}
	}
	delete(s.data, id)
	return nil
}

// handleListGoals отдаёт цели пользователя.
func handleListGoals(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)
	c.JSON(http.StatusOK, gin.H{"goals": goals.list(sessData.username)})
}

// handleCreateGoal создаёт новую цель.
func handleCreateGoal(c *gin.Context) {
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		TargetDate  string `json:"target_date"`
		Status      string `json:"status"`
		Progress    int    `json:"progress"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	if req.Status == "" {
		req.Status = goalActive
	}
	sessData, _ := c.MustGet("session").(session)
	g, err := goals.create(sessData.username, req.Title, req.Description, req.TargetDate, req.Status, req.Progress)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, g)
}

// handleUpdateGoal обновляет цель.
func handleUpdateGoal(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID цели"})
		return
	}
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		TargetDate  string `json:"target_date"`
		Status      string `json:"status"`
		Progress    int    `json:"progress"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	g, err := goals.update(sessData.username, id, req.Title, req.Description, req.TargetDate, req.Status, req.Progress)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "цель не найдена" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, g)
}

// handleDeleteGoal удаляет цель.
func handleDeleteGoal(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID цели"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	if err := goals.delete(sessData.username, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
