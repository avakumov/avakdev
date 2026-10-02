package app

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Статусы целей.
const (
	GoalActive    = "active"
	GoalPaused    = "paused"
	GoalAchieved  = "achieved"
	GoalCancelled = "cancelled"
)

// ErrGoalNotFound — цели нет или она принадлежит другому пользователю.
// Отдельная ошибка, чтобы обработчики отличали её от ошибок валидации.
var ErrGoalNotFound = errors.New("цель не найдена")

var validGoalStatuses = map[string]bool{
	GoalActive:    true,
	GoalPaused:    true,
	GoalAchieved:  true,
	GoalCancelled: true,
}

// Goal — цель раздела «Цели»: результат с дедлайном и статусом.
// Progress в БД не хранится и вычисляется на лету из привязанных задач:
// доля выполненных задач среди неотменённых.
type Goal struct {
	ID          int    `json:"id"`
	Username    string `json:"-"`
	Title       string `json:"title"`
	Description string `json:"description"`
	TargetDate  string `json:"target_date"` // YYYY-MM-DD или пусто
	Status      string `json:"status"`
	Progress    int    `json:"progress"` // 0..100, вычисляется при ответе
	Created     string `json:"created"`
	Updated     string `json:"updated"`
}

// GoalStore — хранилище целей.
type GoalStore struct {
	mu     sync.Mutex
	data   map[int]Goal
	nextID int
	pool   *pgxpool.Pool
	tasks  *TaskStore
}

// NewGoalStore создаёт хранилище целей (pool == nil — работаем без БД).
func NewGoalStore(pool *pgxpool.Pool) *GoalStore {
	return &GoalStore{
		data:   make(map[int]Goal),
		nextID: 1,
		pool:   pool,
	}
}

// Load подгружает сохранённые цели из БД (pool == nil — ничего не делает).
func (s *GoalStore) Load() error {
	if s.pool == nil {
		return nil
	}
	rows, err := s.pool.Query(context.Background(),
		`SELECT id,
		        username,
		        title,
		        description,
		        COALESCE(to_char(target_date,'YYYY-MM-DD'),''),
		        status,
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
			&g.TargetDate, &g.Status, &g.Created, &g.Updated); err != nil {
			return err
		}
		s.data[g.ID] = g
		if g.ID >= s.nextID {
			s.nextID = g.ID + 1
		}
	}
	return rows.Err()
}

// List возвращает цели пользователя, новые сверху.
func (s *GoalStore) List(username string) []Goal {
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

// GetOwned возвращает цель, если она принадлежит пользователю.
func (s *GoalStore) GetOwned(username string, id int) (Goal, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.data[id]
	if !ok || g.Username != username {
		return Goal{}, false
	}
	return g, true
}

// validateGoalInput проверяет название и статус цели, возвращает подрезанное
// название (используется и при создании, и при обновлении).
func validateGoalInput(title, status string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", errors.New("укажите название цели")
	}
	if !validGoalStatuses[status] {
		return "", errors.New("некорректный статус цели")
	}
	return title, nil
}

// Create добавляет новую цель.
func (s *GoalStore) Create(username, title, description, targetDate, status string) (Goal, error) {
	title, err := validateGoalInput(title, status)
	if err != nil {
		return Goal{}, err
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
		Created:     now,
		Updated:     now,
	}

	if s.pool != nil {
		err := s.pool.QueryRow(context.Background(),
			`INSERT INTO goals (username, title, description, target_date, status)
			 VALUES ($1, $2, $3, $4, $5)
			 RETURNING id,
			           to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			           to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
			username, title, description, nullableDate(targetDate), status).
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

// Update обновляет цель.
func (s *GoalStore) Update(username string, id int, title, description, targetDate, status string) (Goal, error) {
	title, err := validateGoalInput(title, status)
	if err != nil {
		return Goal{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	g, ok := s.data[id]
	if !ok || g.Username != username {
		return Goal{}, ErrGoalNotFound
	}

	g.Title = title
	g.Description = description
	g.TargetDate = targetDate
	g.Status = status
	g.Updated = time.Now().UTC().Format(time.RFC3339)

	if s.pool != nil {
		if _, err := s.pool.Exec(context.Background(),
			`UPDATE goals
			 SET title = $2, description = $3, target_date = $4,
			     status = $5, updated = now()
			 WHERE id = $1`,
			id, g.Title, g.Description, nullableDate(targetDate), g.Status); err != nil {
			return Goal{}, err
		}
	}

	s.data[id] = g
	return g, nil
}

// ComputeProgress заполняет Progress цели на лету по привязанным задачам:
// процент выполненных задач среди неотменённых. Значение не хранится в БД.
// 100% достижимо только при статусе «достигнута» (achieved): даже если все
// задачи выполнены, пока цель официально не завершена, показывается 99%.
func (s *GoalStore) ComputeProgress(g *Goal) {
	if g.Status == GoalAchieved {
		g.Progress = 100
		return
	}
	if s.tasks == nil {
		g.Progress = 0
		return
	}
	done, active := s.tasks.completionStats(g.Username, g.ID)
	if active <= 0 {
		g.Progress = 0
		return
	}
	p := int(math.Round(float64(done) * 100 / float64(active)))
	if p >= 100 {
		p = 99
	}
	g.Progress = p
}

// Delete удаляет цель пользователя. При deleteTasks=true привязанные задачи
// удаляются вместе с целью; иначе задачи остаются, но ссылка на цель
// сбрасывается (в БД — внешним ключом ON DELETE SET NULL, в памяти — вручную).
func (s *GoalStore) Delete(username string, id int, deleteTasks bool) error {
	s.mu.Lock()
	g, ok := s.data[id]
	if !ok || g.Username != username {
		s.mu.Unlock()
		return ErrGoalNotFound
	}
	// Не держим блокировку целей во время обращения к задачам: задачи сначала
	// удаляются (пока ссылка ещё стоит), затем цель.
	s.mu.Unlock()

	if s.tasks != nil {
		if deleteTasks {
			if _, err := s.tasks.RemoveByGoal(username, id); err != nil {
				return err
			}
		} else {
			s.tasks.ClearGoalLinks(username, id)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[id]; !ok || s.data[id].Username != username {
		return ErrGoalNotFound
	}
	if s.pool != nil {
		if _, err := s.pool.Exec(context.Background(),
			`DELETE FROM goals WHERE id = $1`, id); err != nil {
			return err
		}
	}
	delete(s.data, id)
	return nil
}

// nullableDate — дедлайн для SQL: пустая строка значит «без даты» (NULL).
func nullableDate(targetDate string) any {
	if targetDate == "" {
		return nil
	}
	return targetDate
}
