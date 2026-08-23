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

// Статусы задач по модификации приложения.
const (
	taskStatusNew        = "new"
	taskStatusInProgress = "in_progress"
	taskStatusDone       = "done"
	taskStatusCancelled  = "cancelled"
	taskStatusFailed     = "failed"
)

// validTaskStatuses — допустимые значения статуса.
var validTaskStatuses = map[string]bool{
	taskStatusNew:        true,
	taskStatusInProgress: true,
	taskStatusDone:       true,
	taskStatusCancelled:  true,
	taskStatusFailed:     true,
}

// AppTask — задача по модификации приложения: описание того, что нужно
// исправить, изменить или добавить. Владелец — конкретный пользователь.
type AppTask struct {
	ID          int    `json:"id"`
	Username    string `json:"-"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Result      string `json:"result"`
	Log         string `json:"log"`
	Created     string `json:"created"`
	Updated     string `json:"updated"`
}

// appTaskStore — хранилище задач по модификации приложения.
type appTaskStore struct {
	mu     sync.Mutex
	data   map[int]AppTask
	nextID int
	hasDB  bool
}

// appTasks — глобальное хранилище задач.
var appTasks *appTaskStore

// initAppTasks инициализирует глобальное хранилище задач.
// При наличии БД подгружает сохранённые задачи в память.
// (Таблица создаётся версионированными миграциями goose, см. migrations/.)
func initAppTasks() error {
	appTasks = &appTaskStore{
		data:   make(map[int]AppTask),
		nextID: 1,
		hasDB:  db != nil,
	}
	if !appTasks.hasDB {
		return nil
	}

	rows, err := db.Query(context.Background(),
		`SELECT id,
		        username,
		        title,
		        description,
		        status,
		        result,
		        log,
		        to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM app_tasks`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var t AppTask
		if err := rows.Scan(&t.ID, &t.Username, &t.Title, &t.Description,
			&t.Status, &t.Result, &t.Log, &t.Created, &t.Updated); err != nil {
			return err
		}
		appTasks.data[t.ID] = t
		if t.ID >= appTasks.nextID {
			appTasks.nextID = t.ID + 1
		}
	}
	return rows.Err()
}

// list возвращает задачи пользователя, новые сверху (по дате создания).
func (s *appTaskStore) list(username string) []AppTask {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]AppTask, 0)
	for _, t := range s.data {
		if t.Username == username {
			out = append(out, t)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out
}

// getOwned возвращает задачу, если она принадлежит пользователю.
func (s *appTaskStore) getOwned(username string, id int) (AppTask, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.data[id]
	if !ok || t.Username != username {
		return AppTask{}, false
	}
	return t, true
}

// create добавляет новую задачу (заголовок + описание).
func (s *appTaskStore) create(username, title, description string) (AppTask, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return AppTask{}, errors.New("укажите заголовок задачи")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC().Format(time.RFC3339)
	t := AppTask{
		Username:    username,
		Title:       title,
		Description: description,
		Status:      taskStatusNew,
		Created:     now,
		Updated:     now,
	}

	if s.hasDB {
		err := db.QueryRow(context.Background(),
			`INSERT INTO app_tasks (username, title, description, status)
			 VALUES ($1, $2, $3, 'new')
			 RETURNING id,
			           to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			           to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
			username, title, description).
			Scan(&t.ID, &t.Created, &t.Updated)
		if err != nil {
			return AppTask{}, err
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

// update обновляет задачу (заголовок, описание, статус, результат, журнал).
// result и log — указатели: nil означает «не менять» (так UI-редактор не
// затирает данные, записанные агентом).
func (s *appTaskStore) update(username string, id int, title, description, status string, result, log *string) (AppTask, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return AppTask{}, errors.New("укажите заголовок задачи")
	}
	if !validTaskStatuses[status] {
		return AppTask{}, errors.New("некорректный статус задачи")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.data[id]
	if !ok || t.Username != username {
		return AppTask{}, errors.New("задача не найдена")
	}

	t.Title = title
	t.Description = description
	t.Status = status
	if result != nil {
		t.Result = *result
	}
	if log != nil {
		t.Log = *log
	}
	t.Updated = time.Now().UTC().Format(time.RFC3339)

	if s.hasDB {
		if _, err := db.Exec(context.Background(),
			`UPDATE app_tasks
			 SET title = $2, description = $3, status = $4, result = $5, log = $6, updated = now()
			 WHERE id = $1`,
			id, t.Title, t.Description, t.Status, t.Result, t.Log); err != nil {
			return AppTask{}, err
		}
	}

	s.data[id] = t
	return t, nil
}

// delete удаляет задачу пользователя.
func (s *appTaskStore) delete(username string, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.data[id]
	if !ok || t.Username != username {
		return errors.New("задача не найдена")
	}
	if s.hasDB {
		if _, err := db.Exec(context.Background(),
			`DELETE FROM app_tasks WHERE id = $1`, id); err != nil {
			return err
		}
	}
	delete(s.data, id)
	return nil
}

// handleListAppTasks отдаёт задачи пользователя.
func handleListAppTasks(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)
	c.JSON(http.StatusOK, gin.H{"tasks": appTasks.list(sessData.username)})
}

// handleCreateAppTask создаёт новую задачу.
func handleCreateAppTask(c *gin.Context) {
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	t, err := appTasks.create(sessData.username, req.Title, req.Description)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, t)
}

// handleUpdateAppTask обновляет задачу (заголовок, описание, статус).
func handleUpdateAppTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID задачи"})
		return
	}
	var req struct {
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Status      string  `json:"status"`
		Result      *string `json:"result"`
		Log         *string `json:"log"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	t, err := appTasks.update(sessData.username, id, req.Title, req.Description, req.Status, req.Result, req.Log)
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

// handleDeleteAppTask удаляет задачу.
func handleDeleteAppTask(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID задачи"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	if err := appTasks.delete(sessData.username, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
