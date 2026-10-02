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

// Статусы задач раздела «Задачи».
const (
	TaskTodo       = "todo"
	TaskInProgress = "in_progress"
	TaskDone       = "done"
	TaskCancelled  = "cancelled"
)

// validTodoStatuses — допустимые значения статуса задач раздела «Задачи».
// Имя не пересекается с validTaskStatuses из app_tasks.go (раздел «Приложение»).
var validTodoStatuses = map[string]bool{
	TaskTodo:       true,
	TaskInProgress: true,
	TaskDone:       true,
	TaskCancelled:  true,
}

// TaskCategories — предопределённые категории задач.
var TaskCategories = []string{"Работа", "Личное", "Учёба", "Дом", "Прочее"}

// TaskDaySpent — фактически потраченное время задачи за конкретный день
// (позиция плана этого дня).
type TaskDaySpent struct {
	Date    string `json:"date"`    // ГГГГ-ММ-ДД
	Minutes int    `json:"minutes"` // фактически проставленные минуты
}

// Task — задача раздела «Задачи»: категория, планируемое и фактическое время
// в часах, дедлайн и статус. Владелец — конкретный пользователь.
// GoalID — ссылка на цель из раздела «Цели» (nil — задача без цели);
// Position — порядок выполнения внутри цели (1..N, 0 — вне цели).
type Task struct {
	ID           int     `json:"id"`
	Username     string  `json:"-"`
	Category     string  `json:"category"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	PlannedHours float64 `json:"planned_hours"`
	// ActualHours — потраченное время, ч. Только для чтения: сумма фактического
	// времени по дням (day_items.actual_minutes) плюс «база» — остаток прежнего
	// значения tasks.actual_hours у задач, которых нет ни в одном плане.
	// Редактируется в разделе «День» (факт по позиции дня).
	ActualHours float64 `json:"actual_hours"`
	// SpentByDay — разбивка потраченного времени по дням (только для чтения).
	SpentByDay []TaskDaySpent `json:"spent_by_day"`
	Deadline   string         `json:"deadline"` // дата YYYY-MM-DD или пусто
	Status     string         `json:"status"`
	// CompletedAt — когда задача отмечена выполненной (пусто — не выполнена).
	CompletedAt string `json:"completed_at"`
	// CompletedDate — за какой день задача выполнена (ГГГГ-ММ-ДД). По ней она
	// попадает в план/отчёты дня; может быть раньше даты отметки (задним числом).
	CompletedDate string `json:"completed_date"`
	GoalID        *int   `json:"goal_id"`
	Position      int    `json:"position"`
	Created       string `json:"created"`
	Updated       string `json:"updated"`
}

// TaskStore — хранилище задач раздела «Задачи».
type TaskStore struct {
	mu     sync.Mutex
	data   map[int]Task
	nextID int
	pool   *pgxpool.Pool
	goals  *GoalStore
}

// NewTaskStore создаёт хранилище задач (pool == nil — работаем без БД).
func NewTaskStore(pool *pgxpool.Pool) *TaskStore {
	return &TaskStore{
		data:   make(map[int]Task),
		nextID: 1,
		pool:   pool,
	}
}

// Load подгружает сохранённые задачи из БД (pool == nil — ничего не делает).
func (s *TaskStore) Load() error {
	if s.pool == nil {
		return nil
	}
	rows, err := s.pool.Query(context.Background(),
		`SELECT id,
		        username,
		        category,
		        title,
		        description,
		        planned_hours,
		        actual_hours,
		        COALESCE(to_char(deadline,'YYYY-MM-DD'),''),
		        status,
		        COALESCE(to_char(completed_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
		        COALESCE(to_char(completed_date,'YYYY-MM-DD'),''),
		        goal_id,
		        position,
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
			&t.Status, &t.CompletedAt, &t.CompletedDate, &t.GoalID, &t.Position, &t.Created, &t.Updated); err != nil {
			return err
		}
		s.data[t.ID] = t
		if t.ID >= s.nextID {
			s.nextID = t.ID + 1
		}
	}
	return rows.Err()
}

// taskSpentByDay возвращает разбивку фактически потраченного времени по дням
// для задач пользователя: id задачи → список дней с минутами. Без БД — пустая
// карта. Источник — day_items.actual_minutes (ввод в разделе «День»).
func (s *TaskStore) taskSpentByDay(username string) map[int][]TaskDaySpent {
	byDay := make(map[int][]TaskDaySpent)
	if s.pool == nil {
		return byDay
	}
	rows, err := s.pool.Query(context.Background(),
		`SELECT i.ref_id, to_char(p.day, 'YYYY-MM-DD'), i.actual_minutes
		   FROM day_items i
		   JOIN day_plans p ON p.id = i.plan_id
		  WHERE i.kind = 'task' AND p.username = $1 AND i.actual_minutes > 0
		  ORDER BY i.ref_id, p.day`, username)
	if err != nil {
		return byDay
	}
	defer rows.Close()
	for rows.Next() {
		var id, minutes int
		var day string
		if err := rows.Scan(&id, &day, &minutes); err != nil {
			continue
		}
		byDay[id] = append(byDay[id], TaskDaySpent{Date: day, Minutes: minutes})
	}
	return byDay
}

// WithSpent заполняет у копий задач потраченное время: ActualHours — сумма
// фактического времени по дням плюс «база» (остаток tasks.actual_hours у задач
// вне планов), SpentByDay — разбивка по дням. Вызывать нужно один раз на срез:
// повторный вызов примет уже посчитанную сумму за «базу».
func (s *TaskStore) WithSpent(username string, ts []Task) []Task {
	byDay := s.taskSpentByDay(username)
	for i := range ts {
		t := &ts[i]
		days := byDay[t.ID]
		if days == nil {
			days = []TaskDaySpent{}
		}
		minutes := int(t.ActualHours*60 + 0.5) // «база» без разбивки по дням
		for _, d := range days {
			minutes += d.Minutes
		}
		t.SpentByDay = days
		t.ActualHours = float64(minutes) / 60
	}
	return ts
}

// List возвращает задачи пользователя, новые сверху (по дате создания).
func (s *TaskStore) List(username string) []Task {
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

// maxPositionLocked возвращает максимальный position среди задач цели.
// Вызывается только при удержании s.mu.
func (s *TaskStore) maxPositionLocked(goalID int) int {
	best := 0
	for _, x := range s.data {
		if x.GoalID != nil && *x.GoalID == goalID && x.Position > best {
			best = x.Position
		}
	}
	return best
}

// SetGoalOrder задаёт последовательность задач цели: ids — полный список
// id задач пользователя, привязанных к цели, в нужном порядке (позиции 1..N).
func (s *TaskStore) SetGoalOrder(username string, goalID int, ids []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := make(map[int]bool)
	for id, t := range s.data {
		if t.Username == username && t.GoalID != nil && *t.GoalID == goalID {
			current[id] = true
		}
	}
	if len(ids) != len(current) {
		return errors.New("переданы не все задачи цели")
	}

	seen := make(map[int]bool, len(ids))
	for i, id := range ids {
		if !current[id] || seen[id] {
			return errors.New("некорректный список задач цели")
		}
		seen[id] = true

		pos := i + 1
		t := s.data[id]
		if t.Position != pos {
			t.Position = pos
			s.data[id] = t
			if s.pool != nil {
				if _, err := s.pool.Exec(context.Background(),
					`UPDATE tasks SET position = $2 WHERE id = $1 AND username = $3 AND goal_id = $4`,
					id, pos, username, goalID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// GetOwned возвращает задачу, если она принадлежит пользователю.
func (s *TaskStore) GetOwned(username string, id int) (Task, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.data[id]
	if !ok || t.Username != username {
		return Task{}, false
	}
	return t, true
}

// dateOnlyLayout — формат даты без времени (completed_date).
const dateOnlyLayout = "2006-01-02"

// explicitDate достаёт необязательную дату из вариативного аргумента:
// пустая строка означает «не указана».
func explicitDate(dates []string) string {
	if len(dates) == 0 {
		return ""
	}
	return strings.TrimSpace(dates[0])
}

// Create добавляет новую задачу.
// goalID — ссылка на цель пользователя; nil означает «без цели».
// Необязательный completedDate (ГГГГ-ММ-ДД) — за какой день задача выполнена:
// нужен, когда задачу создают сразу закрытой задним числом.
// Потраченное время (ActualHours) не задаётся здесь: оно складывается из
// фактического времени по дням (см. раздел «День»).
func (s *TaskStore) Create(username, category, title, description string, plannedHours float64, deadline, status string, goalID *int, completedDate ...string) (Task, error) {
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
	if plannedHours < 0 {
		return Task{}, errors.New("время не может быть отрицательным")
	}
	if goalID != nil {
		if s.goals == nil {
			return Task{}, errors.New("цель не найдена или недоступна")
		}
		if _, ok := s.goals.GetOwned(username, *goalID); !ok {
			return Task{}, errors.New("цель не найдена или недоступна")
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	nowTime := time.Now().UTC()
	now := nowTime.Format(time.RFC3339)
	// Новая задача в цели дописывается в конец её последовательности.
	position := 0
	if goalID != nil {
		position = s.maxPositionLocked(*goalID) + 1
	}
	// Задачу могут создать сразу выполненной — тогда сразу ставим отметку
	// закрытия. Дата выполнения определяет, в какой день задача попадёт в
	// план/отчёты: указанная пользователем либо сегодняшняя.
	completedAt, doneDate := "", ""
	if status == TaskDone {
		completedAt = now
		doneDate = nowTime.Format(dateOnlyLayout)
		if d := explicitDate(completedDate); d != "" {
			doneDate = d
		}
	}
	t := Task{
		Username:      username,
		Category:      category,
		Title:         title,
		Description:   description,
		PlannedHours:  plannedHours,
		Deadline:      deadline,
		Status:        status,
		CompletedAt:   completedAt,
		CompletedDate: doneDate,
		GoalID:        goalID,
		Position:      position,
		Created:       now,
		Updated:       now,
	}

	if s.pool != nil {
		var dl interface{}
		if deadline != "" {
			dl = deadline
		}
		var gid interface{}
		if goalID != nil {
			gid = *goalID
		}
		err := s.pool.QueryRow(context.Background(),
			`INSERT INTO tasks (username, category, title, description, planned_hours, deadline, status, completed_at, completed_date, goal_id, position)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8::text::timestamptz, $9::text::date, $10, $11)
			 RETURNING id,
			           to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
			           to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
			username, category, title, description, plannedHours, dl, status,
			nullableDate(completedAt), nullableDate(doneDate), gid, position).
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

// Update обновляет задачу.
// goalID — новая ссылка на цель пользователя; nil означает «без цели».
// Необязательный completedDate (ГГГГ-ММ-ДД) — за какой день задача выполнена
// (позволяет отметить забытую задачу задним числом).
// Потраченное время здесь не меняется: оно складывается из фактов по дням.
func (s *TaskStore) Update(username string, id int, category, title, description string, plannedHours float64, deadline, status string, goalID *int, completedDate ...string) (Task, error) {
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
	if plannedHours < 0 {
		return Task{}, errors.New("время не может быть отрицательным")
	}
	if goalID != nil {
		if s.goals == nil {
			return Task{}, errors.New("цель не найдена или недоступна")
		}
		if _, ok := s.goals.GetOwned(username, *goalID); !ok {
			return Task{}, errors.New("цель не найдена или недоступна")
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.data[id]
	if !ok || t.Username != username {
		return Task{}, errors.New("задача не найдена")
	}

	// При переносе задачи в другую цель (или из «без цели») дописываем её
	// в конец последовательности новой цели. Внутри той же цели порядок
	// не трогаем — им управляет отдельный эндпоинт смены порядка.
	oldGoal := t.GoalID
	if goalID != nil && (oldGoal == nil || *oldGoal != *goalID) {
		t.Position = s.maxPositionLocked(*goalID) + 1
	} else if goalID == nil {
		t.Position = 0
	}

	t.Category = category
	t.Title = title
	t.Description = description
	t.PlannedHours = plannedHours
	t.Deadline = deadline
	// Отметка закрытия: время ставим при переходе в «выполнена», снимаем при
	// возврате из неё. Дата (completed_date) определяет день в плане/отчётах:
	// явно указанная пользователем либо сегодняшняя — по ней отчёт дня
	// показывает закрытые задачи.
	now := time.Now().UTC()
	switch {
	case status != TaskDone:
		t.CompletedAt = ""
		t.CompletedDate = ""
	default:
		if t.CompletedAt == "" {
			t.CompletedAt = now.Format(time.RFC3339)
		}
		if d := explicitDate(completedDate); d != "" {
			t.CompletedDate = d
		} else if t.CompletedDate == "" {
			t.CompletedDate = now.Format(dateOnlyLayout)
		}
	}
	t.Status = status
	t.GoalID = goalID
	t.Updated = now.Format(time.RFC3339)

	if s.pool != nil {
		var dl interface{}
		if deadline != "" {
			dl = deadline
		}
		var gid interface{}
		if goalID != nil {
			gid = *goalID
		}
		if _, err := s.pool.Exec(context.Background(),
			`UPDATE tasks
			 SET category = $2, title = $3, description = $4,
			     planned_hours = $5, deadline = $6,
			     status = $7, completed_at = $8::text::timestamptz,
			     completed_date = $9::text::date, goal_id = $10, position = $11, updated = now()
			 WHERE id = $1`,
			id, t.Category, t.Title, t.Description, t.PlannedHours,
			dl, t.Status, nullableDate(t.CompletedAt),
			nullableDate(t.CompletedDate), gid, t.Position); err != nil {
			return Task{}, err
		}
	}

	s.data[id] = t

	// Если задача покинула цель (отвязана или перенесена в другую),
	// уплотняем позиции оставшихся задач прежней цели (1..N) — без «дырок».
	if oldGoal != nil && (goalID == nil || *oldGoal != *goalID) {
		if err := s.compactGoalPositionsLocked(username, *oldGoal); err != nil {
			return Task{}, err
		}
	}
	return t, nil
}

// Delete удаляет задачу пользователя. Если задача была привязана к цели,
// оставшиеся задачи цели перенумеровываются подряд (1..N) — без «дырок».
func (s *TaskStore) Delete(username string, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.data[id]
	if !ok || t.Username != username {
		return errors.New("задача не найдена")
	}
	if s.pool != nil {
		if _, err := s.pool.Exec(context.Background(),
			`DELETE FROM tasks WHERE id = $1`, id); err != nil {
			return err
		}
	}
	delete(s.data, id)

	if t.GoalID != nil {
		return s.compactGoalPositionsLocked(username, *t.GoalID)
	}
	return nil
}

// compactGoalPositionsLocked перенумеровывает задачи цели подряд (1..N),
// сохраняя их относительный порядок. Вызывается при удержании s.mu.
func (s *TaskStore) compactGoalPositionsLocked(username string, goalID int) error {
	ids := make([]int, 0)
	for id, x := range s.data {
		if x.Username == username && x.GoalID != nil && *x.GoalID == goalID {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := s.data[ids[i]], s.data[ids[j]]
		if a.Position != b.Position {
			return a.Position < b.Position
		}
		return a.ID < b.ID
	})
	for i, id := range ids {
		pos := i + 1
		x := s.data[id]
		if x.Position == pos {
			continue
		}
		x.Position = pos
		s.data[id] = x
		if s.pool != nil {
			if _, err := s.pool.Exec(context.Background(),
				`UPDATE tasks SET position = $2 WHERE id = $1`,
				id, pos); err != nil {
				return err
			}
		}
	}
	return nil
}

// completionStats возвращает, сколько задач, привязанных к цели, выполнено
// (done) и сколько «активны» — участвуют в расчёте прогресса. Отменённые
// (cancelled) задачи не учитываются вовсе.
func (s *TaskStore) completionStats(username string, goalID int) (done, active int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.data {
		if t.Username != username || t.GoalID == nil || *t.GoalID != goalID {
			continue
		}
		if t.Status == TaskCancelled {
			continue
		}
		active++
		if t.Status == TaskDone {
			done++
		}
	}
	return done, active
}

// ClearGoalLinks убирает у задач пользователя ссылку на удаляемую цель.
// В БД это делает внешний ключ (ON DELETE SET NULL), здесь — в памяти сервера.
func (s *TaskStore) ClearGoalLinks(username string, goalID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, t := range s.data {
		if t.Username == username && t.GoalID != nil && *t.GoalID == goalID {
			t.GoalID = nil
			s.data[id] = t
		}
	}
}

// RemoveByGoal удаляет все задачи пользователя, привязанные к цели
// (и из БД, и из памяти). Используется при удалении цели с опцией
// «удалить привязанные задачи». Возвращает количество удалённых задач.
func (s *TaskStore) RemoveByGoal(username string, goalID int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]int, 0)
	for id, t := range s.data {
		if t.Username == username && t.GoalID != nil && *t.GoalID == goalID {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return 0, nil
	}

	if s.pool != nil {
		if _, err := s.pool.Exec(context.Background(),
			`DELETE FROM tasks WHERE username = $1 AND goal_id = $2`,
			username, goalID); err != nil {
			return 0, err
		}
	}
	for _, id := range ids {
		delete(s.data, id)
	}
	return len(ids), nil
}
