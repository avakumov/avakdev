package app

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Статусы задач по модификации приложения.
const (
	AppTaskStatusNew        = "new"
	AppTaskStatusInProgress = "in_progress"
	AppTaskStatusDone       = "done"
	AppTaskStatusCancelled  = "cancelled"
	AppTaskStatusFailed     = "failed"
)

// validAppTaskStatuses — допустимые значения статуса.
var validAppTaskStatuses = map[string]bool{
	AppTaskStatusNew:        true,
	AppTaskStatusInProgress: true,
	AppTaskStatusDone:       true,
	AppTaskStatusCancelled:  true,
	AppTaskStatusFailed:     true,
}

// AppTask — задача по модификации приложения: описание того, что нужно
// исправить, изменить или добавить. Владелец — конкретный пользователь.
type AppTask struct {
	ID              int    `json:"id"`
	Username        string `json:"-"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Status          string `json:"status"`
	Result          string `json:"result"`
	Log             string `json:"log"`
	DeployRequested bool   `json:"deploy_requested"`
	DeployedAt      string `json:"deployed_at"`
	CommitHash      string `json:"commit_hash"`
	RevertRequested bool   `json:"revert_requested"`
	RevertedAt      string `json:"reverted_at"`
	Created         string `json:"created"`
	Updated         string `json:"updated"`
}

// AppTaskStore — хранилище задач по модификации приложения.
type AppTaskStore struct {
	mu     sync.Mutex
	data   map[int]AppTask
	nextID int
	pool   *pgxpool.Pool
}

// NewAppTaskStore создаёт хранилище и подгружает задачи из БД
// (pool == nil — работаем без БД).
func NewAppTaskStore(pool *pgxpool.Pool) (*AppTaskStore, error) {
	s := &AppTaskStore{
		data:   make(map[int]AppTask),
		nextID: 1,
		pool:   pool,
	}
	if pool == nil {
		return s, nil
	}

	rows, err := pool.Query(context.Background(),
		`SELECT id,
		        username,
		        title,
		        description,
		        status,
		        result,
		        log,
		        deploy_requested,
		        deployed_at,
		        commit_hash,
		        revert_requested,
		        reverted_at,
		        to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM app_tasks`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var t AppTask
		if err := rows.Scan(&t.ID, &t.Username, &t.Title, &t.Description,
			&t.Status, &t.Result, &t.Log, &t.DeployRequested, &t.DeployedAt,
			&t.CommitHash, &t.RevertRequested, &t.RevertedAt,
			&t.Created, &t.Updated); err != nil {
			return nil, err
		}
		s.data[t.ID] = t
		if t.ID >= s.nextID {
			s.nextID = t.ID + 1
		}
	}
	return s, rows.Err()
}

// List возвращает задачи пользователя, новые сверху (по дате создания).
func (s *AppTaskStore) List(username string) []AppTask {
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

// GetOwned возвращает задачу, если она принадлежит пользователю.
func (s *AppTaskStore) GetOwned(username string, id int) (AppTask, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.data[id]
	if !ok || t.Username != username {
		return AppTask{}, false
	}
	return t, true
}

// Create добавляет новую задачу (заголовок + описание).
func (s *AppTaskStore) Create(username, title, description string) (AppTask, error) {
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
		Status:      AppTaskStatusNew,
		Created:     now,
		Updated:     now,
	}

	if s.pool != nil {
		err := s.pool.QueryRow(context.Background(),
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

// Update обновляет задачу (заголовок, описание, статус, результат, журнал,
// флаги деплоя/отката, хэш коммита и времена). Указатели: nil означает
// «не менять» (так UI-редактор не затирает данные, записанные агентом).
func (s *AppTaskStore) Update(username string, id int, title, description, status string, result, log *string, deployRequested *bool, deployedAt *string, commitHash *string, revertRequested *bool, revertedAt *string) (AppTask, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return AppTask{}, errors.New("укажите заголовок задачи")
	}
	if !validAppTaskStatuses[status] {
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
	if deployRequested != nil {
		t.DeployRequested = *deployRequested
	}
	if deployedAt != nil {
		t.DeployedAt = *deployedAt
	}
	if commitHash != nil {
		t.CommitHash = *commitHash
	}
	if revertRequested != nil {
		t.RevertRequested = *revertRequested
	}
	if revertedAt != nil {
		t.RevertedAt = *revertedAt
	}
	t.Updated = time.Now().UTC().Format(time.RFC3339)

	if s.pool != nil {
		if _, err := s.pool.Exec(context.Background(),
			`UPDATE app_tasks
			 SET title = $2, description = $3, status = $4, result = $5, log = $6,
			     deploy_requested = $7, deployed_at = $8, commit_hash = $9,
			     revert_requested = $10, reverted_at = $11, updated = now()
			 WHERE id = $1`,
			id, t.Title, t.Description, t.Status, t.Result, t.Log,
			t.DeployRequested, t.DeployedAt, t.CommitHash,
			t.RevertRequested, t.RevertedAt); err != nil {
			return AppTask{}, err
		}
	}

	s.data[id] = t
	return t, nil
}

// Delete удаляет задачу пользователя.
func (s *AppTaskStore) Delete(username string, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.data[id]
	if !ok || t.Username != username {
		return errors.New("задача не найдена")
	}
	if s.pool != nil {
		if _, err := s.pool.Exec(context.Background(),
			`DELETE FROM app_tasks WHERE id = $1`, id); err != nil {
			return err
		}
	}
	delete(s.data, id)
	return nil
}
