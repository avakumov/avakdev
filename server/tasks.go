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

// Статусы задач раздела «Задачи».
const (
	taskTodo       = "todo"
	taskInProgress = "in_progress"
	taskDone       = "done"
	taskCancelled  = "cancelled"
)

// validTodoStatuses — допустимые значения статуса задач раздела «Задачи».
// Имя не пересекается с validTaskStatuses из app_tasks.go (раздел «Приложение»).
var validTodoStatuses = map[string]bool{
	taskTodo:       true,
	taskInProgress: true,
	taskDone:       true,
	taskCancelled:  true,
}

// TaskCategories — предопределённые категории задач.
var TaskCategories = []string{"Работа", "Личное", "Учёба", "Дом", "Прочее"}

// Task — задача раздела «Задачи»: категория, планируемое и фактическое время
// в часах, дедлайн и статус. Владелец — конкретный пользователь.
// GoalID — ссылка на цель из раздела «Цели» (nil — задача без цели).
type Task struct {
	ID           int     `json:"id"`
	Username     string  `json:"-"`
	Category     string  `json:"category"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	PlannedHours float64 `json:"planned_hours"`
	ActualHours  float64 `json:"actual_hours"`
	Deadline     string  `json:"deadline"` // дата YYYY-MM-DD или пусто
	Status       string  `json:"status"`
	GoalID       *int    `json:"goal_id"`
	Created      string  `json:"created"`
	Updated      string  `json:"updated"`
}

// taskStore — хранилище задач раздела «Задачи».
type taskStore struct {
	mu     sync.Mutex
	data   map[int]Task
	nextID int
	hasDB  bool
}

// tasks — глобальное хранилище задач.
var tasks *taskStore

// initTasks инициализирует глобальное хранилище задач.
// При наличии БД подгружает сохранённые задачи в память.
func initTasks() error {
	tasks = &taskStore{
		data:   make(map[int]Task),
		nextID: 1,
		hasDB:  db != nil,
	}
	if !tasks.hasDB {
		return nil
	}

	rows, err := db.Query(context.Background(),
		`SELECT id,
		        username,
		        category,
		        title,
		        description,
		        planned_hours,
		        actual_hours,
		        COALESCE(to_char(deadline,'YYYY-MM-DD'),''),
		        status,
		        goal_id,
		        to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM tasks`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Username, &t.Category, &t.Title,
			&t.Description, &t.PlannedHours, &t.ActualHours, &t.Deadline,
			&t.Status, &t.GoalID, &t.Created, &t.Updated); err != nil {
			return err
		}
		tasks.data[t.ID] = t
		if t.ID >= tasks.nextID {
			tasks.nextID = t.ID + 1
		}
	}
	return rows.Err()
}

// list возвращает задачи пользователя, новые сверху (по дате создания).
func (s *taskStore) list(username string) []Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Task, 0)
	for _, t := range s.data {
		if t.Username == username {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

// getOwned возвращает задачу, если она принадлежит пользователю.
func (s *taskStore) getOwned(username string, id int) (Task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.data[id]
	if !ok || t.Username != username {
		return Task{}, false
	}
	return t, true
}

// create добавляет новую задачу.
// goalID — ссылка на цель пользователя; nil означает «без цели».
func (s *taskStore) create(username, category, title, description string, plannedHours, actualHours float64, deadline, status string, goalID *int) (Task, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Task{}, errors.New("укажите заголовок задачи")
	}
	if !validTodoStatuses[status] {
		return Task{}, errors.New("некорректный статус задачи")
	}
	if category == "" {
		category = "Прочее"
	}
	if plannedHours < 0 || actualHours < 0 {
		return Task{}, errors.New("время не может быть отрицательным")
	}
	if goalID != nil {
		if _, ok := goals.getOwned(username, *goalID); !ok {
			return Task{}, errors.New("цель не найдена или недоступна")
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	t := Task{
		Username:     username,
		Category:     category,
		Title:        title,
		Description:  description,
		PlannedHours: plannedHours,
		ActualHours:  actualHours,
		Deadline:     deadline,
		Status:       status,
		GoalID:       goalID,
		Created:      now,
		Updated:      now,
	}

	if s.hasDB {
		var dl interface{}
		if deadline != "" {
			dl = deadline
		}
		var gid interface{}
		if goalID != nil {
			gid = *goalID
		}
		err := db.QueryRow(context.Background(),
			`INSERT INTO tasks (username, category, title, description, planned_hours, actual_hours, deadline, status, goal_id)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			 RETURNING id,
			           to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			           to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
			username, category, title, description, plannedHours, actualHours, dl, status, gid).
			Scan(&t.ID, &t.Created, &t.Updated)
		if err != nil {
			return Task{}, err
		}
		if t.ID >= s.nextID {
			s.nextID = t.ID + 1
		}
	} else {
		t.ID = s.nextID
		s.nextID++
	}

	s.data[t.ID] = t
	return t, nil
}

// update обновляет задачу.
// goalID — новая ссылка на цель пользователя; nil означает «без цели».
func (s *taskStore) update(username string, id int, category, title, description string, plannedHours, actualHours float64, deadline, status string, goalID *int) (Task, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return Task{}, errors.New("укажите заголовок задачи")
	}
	if !validTodoStatuses[status] {
		return Task{}, errors.New("некорректный статус задачи")
	}
	if category == "" {
		category = "Прочее"
	}
	if plannedHours < 0 || actualHours < 0 {
		return Task{}, errors.New("время не может быть отрицательным")
	}
	if goalID != nil {
		if _, ok := goals.getOwned(username, *goalID); !ok {
			return Task{}, errors.New("цель не найдена или недоступна")
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.data[id]
	if !ok || t.Username != username {
		return Task{}, errors.New("задача не найдена")
	}

	t.Category = category
	t.Title = title
	t.Description = description
	t.PlannedHours = plannedHours
	t.ActualHours = actualHours
	t.Deadline = deadline
	t.Status = status
	t.GoalID = goalID
	t.Updated = time.Now().UTC().Format(time.RFC3339)

	if s.hasDB {
		var dl interface{}
		if deadline != "" {
			dl = deadline
		}
		var gid interface{}
		if goalID != nil {
			gid = *goalID
		}
		if _, err := db.Exec(context.Background(),
			`UPDATE tasks
			 SET category = $2, title = $3, description = $4,
			     planned_hours = $5, actual_hours = $6, deadline = $7,
			     status = $8, goal_id = $9, updated = now()
			 WHERE id = $1`,
			id, t.Category, t.Title, t.Description, t.PlannedHours,
			t.ActualHours, dl, t.Status, gid); err != nil {
			return Task{}, err
		}
	}

	s.data[id] = t
	return t, nil
}

// delete удаляет задачу пользователя.
func (s *taskStore) delete(username string, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.data[id]
	if !ok || t.Username != username {
		return errors.New("задача не найдена")
	}
	if s.hasDB {
		if _, err := db.Exec(context.Background(),
			`DELETE FROM tasks WHERE id = $1`, id); err != nil {
			return err
		}
	}
	delete(s.data, id)
	return nil
}

// handleListTasks отдаёт задачи пользователя, категории и цели
// (для выбора/отображения привязки задачи к цели).
func handleListTasks(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)

	// Лёгкое представление целей пользователя: только id, название, статус.
	allGoals := goals.list(sessData.username)
	brief := make([]struct {
		ID     int    `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}, 0, len(allGoals))
	for _, g := range allGoals {
		brief = append(brief, struct {
			ID     int    `json:"id"`
			Title  string `json:"title"`
			Status string `json:"status"`
		}{g.ID, g.Title, g.Status})
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks":      tasks.list(sessData.username),
		"categories": TaskCategories,
		"goals":      brief,
	})
}

// handleCreateTask создаёт новую задачу.
func handleCreateTask(c *gin.Context) {
	var req struct {
		Category     string  `json:"category"`
		Title        string  `json:"title"`
		Description  string  `json:"description"`
		PlannedHours float64 `json:"planned_hours"`
		ActualHours  float64 `json:"actual_hours"`
		Deadline     string  `json:"deadline"`
		Status       string  `json:"status"`
		GoalID       *int    `json:"goal_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	if req.Status == "" {
		req.Status = taskTodo
	}
	sessData, _ := c.MustGet("session").(session)
	t, err := tasks.create(sessData.username, req.Category, req.Title, req.Description, req.PlannedHours, req.ActualHours, req.Deadline, req.Status, req.GoalID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, t)
}

// handleUpdateTask обновляет задачу.
func handleUpdateTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID задачи"})
		return
	}
	var req struct {
		Category     string  `json:"category"`
		Title        string  `json:"title"`
		Description  string  `json:"description"`
		PlannedHours float64 `json:"planned_hours"`
		ActualHours  float64 `json:"actual_hours"`
		Deadline     string  `json:"deadline"`
		Status       string  `json:"status"`
		GoalID       *int    `json:"goal_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	t, err := tasks.update(sessData.username, id, req.Category, req.Title, req.Description, req.PlannedHours, req.ActualHours, req.Deadline, req.Status, req.GoalID)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "задача не найдена" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, t)
}

// handleDeleteTask удаляет задачу.
func handleDeleteTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID задачи"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	if err := tasks.delete(sessData.username, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
