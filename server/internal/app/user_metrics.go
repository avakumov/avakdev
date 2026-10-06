package app

import (
	"context"
	"errors"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Типы метрик: положительное целое число, положительное дробное число,
// да/нет (boolean).
const (
	MetricTypeInt   = "int"
	MetricTypeFloat = "float"
	MetricTypeBool  = "bool"
)

// validMetricTypes — допустимые значения поля type.
var validMetricTypes = map[string]bool{
	MetricTypeInt:   true,
	MetricTypeFloat: true,
	MetricTypeBool:  true,
}

// MetricDef — определение метрики пользователя (например «Вес», «Калории»,
// «Отжимания», «Курил»). Владелец — конкретный пользователь.
type MetricDef struct {
	ID       int    `json:"id"`
	Username string `json:"-"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Unit     string `json:"unit"`
	Created  string `json:"created"`
}

// MetricValue — значение метрики за день. Одно значение на (метрика, дата):
// повторное сохранение за тот же день перезаписывает его.
type MetricValue struct {
	MetricID int    `json:"metric_id"`
	Date     string `json:"date"`
	Value    string `json:"value"`
}

// MetricStore — хранилище метрик пользователей: определения и значения.
type MetricStore struct {
	mu     sync.Mutex
	defs   map[int]MetricDef         // id -> определение
	vals   map[int]map[string]string // metric_id -> (YYYY-MM-DD -> значение)
	nextID int
	pool   *pgxpool.Pool
}

// NewMetricStore создаёт хранилище метрик и подгружает данные из БД
// (pool == nil — работаем без БД).
func NewMetricStore(pool *pgxpool.Pool) (*MetricStore, error) {
	s := &MetricStore{
		defs:   make(map[int]MetricDef),
		vals:   make(map[int]map[string]string),
		nextID: 1,
		pool:   pool,
	}
	if pool == nil {
		return s, nil
	}

	rows, err := pool.Query(context.Background(),
		`SELECT id,
		        username,
		        name,
		        type,
		        unit,
		        to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM metric_definitions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d MetricDef
		if err := rows.Scan(&d.ID, &d.Username, &d.Name, &d.Type, &d.Unit, &d.Created); err != nil {
			return nil, err
		}
		s.defs[d.ID] = d
		if d.ID >= s.nextID {
			s.nextID = d.ID + 1
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	valueRows, err := pool.Query(context.Background(),
		`SELECT metric_id, to_char(date, 'YYYY-MM-DD'), value FROM metric_values`)
	if err != nil {
		return nil, err
	}
	defer valueRows.Close()
	for valueRows.Next() {
		var id int
		var date, value string
		if err := valueRows.Scan(&id, &date, &value); err != nil {
			return nil, err
		}
		if s.vals[id] == nil {
			s.vals[id] = make(map[string]string)
		}
		s.vals[id][date] = value
	}
	return s, valueRows.Err()
}

// List возвращает определения метрик пользователя (в порядке создания).
func (s *MetricStore) List(username string) []MetricDef {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]MetricDef, 0)
	for _, d := range s.defs {
		if d.Username == username {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// GetOwned возвращает определение метрики, если она принадлежит пользователю.
func (s *MetricStore) GetOwned(username string, id int) (MetricDef, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.defs[id]
	if !ok || d.Username != username {
		return MetricDef{}, false
	}
	return d, true
}

// Values возвращает копию значений метрики: дата -> значение.
func (s *MetricStore) Values(metricID int) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string)
	for date, v := range s.vals[metricID] {
		out[date] = v
	}
	return out
}

// Create добавляет новую метрику пользователя (название + тип + единица).
// Единица измерения применима только к числовым метрикам; для «да/нет» — пусто.
func (s *MetricStore) Create(username, name, metricType, unit string) (MetricDef, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return MetricDef{}, errors.New("укажите название метрики")
	}
	if !validMetricTypes[metricType] {
		return MetricDef{}, errors.New("некорректный тип метрики")
	}
	unit = strings.TrimSpace(unit)
	if metricType == MetricTypeBool {
		unit = ""
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	d := MetricDef{
		Username: username,
		Name:     name,
		Type:     metricType,
		Unit:     unit,
		Created:  time.Now().UTC().Format(time.RFC3339),
	}

	if s.pool != nil {
		err := s.pool.QueryRow(context.Background(),
			`INSERT INTO metric_definitions (username, name, type, unit)
			 VALUES ($1, $2, $3, $4)
			 RETURNING id,
			           to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
			username, name, metricType, unit).
			Scan(&d.ID, &d.Created)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return MetricDef{}, errors.New("метрика с таким названием уже есть")
			}
			return MetricDef{}, err
		}
		if d.ID >= s.nextID {
			s.nextID = d.ID + 1
		}
	} else {
		d.ID = s.nextID
		s.nextID++
	}

	s.defs[d.ID] = d
	return d, nil
}

// Update переименовывает метрику пользователя и меняет её единицу измерения.
// Тип метрики не меняется: значения уже записаны в формате своего типа.
func (s *MetricStore) Update(username string, id int, name, unit string) (MetricDef, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return MetricDef{}, errors.New("укажите название метрики")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.defs[id]
	if !ok || d.Username != username {
		return MetricDef{}, errors.New("метрика не найдена")
	}

	unit = strings.TrimSpace(unit)
	if d.Type == MetricTypeBool {
		unit = ""
	}
	d.Name = name
	d.Unit = unit

	if s.pool != nil {
		if _, err := s.pool.Exec(context.Background(),
			`UPDATE metric_definitions SET name = $2, unit = $3 WHERE id = $1`,
			id, name, unit); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return MetricDef{}, errors.New("метрика с таким названием уже есть")
			}
			return MetricDef{}, err
		}
	}

	s.defs[id] = d
	return d, nil
}

// Delete удаляет метрику пользователя вместе со всеми её значениями.
func (s *MetricStore) Delete(username string, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.defs[id]
	if !ok || d.Username != username {
		return errors.New("метрика не найдена")
	}
	if s.pool != nil {
		if _, err := s.pool.Exec(context.Background(),
			`DELETE FROM metric_definitions WHERE id = $1`, id); err != nil {
			return err
		}
	}
	delete(s.defs, id)
	delete(s.vals, id)
	return nil
}

// SetValue сохраняет значение метрики за день (создаёт или перезаписывает —
// за день фиксируется один показатель). Значение валидируется по типу метрики.
func (s *MetricStore) SetValue(username string, id int, date, value string) error {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return errors.New("некорректная дата")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.defs[id]
	if !ok || d.Username != username {
		return errors.New("метрика не найдена")
	}

	// Нормализуем значение по типу метрики.
	normalized, err := NormalizeMetricValue(d.Type, value)
	if err != nil {
		return err
	}

	if s.pool != nil {
		if _, err := s.pool.Exec(context.Background(),
			`INSERT INTO metric_values (metric_id, date, value, updated)
			 VALUES ($1, $2, $3, now())
			 ON CONFLICT (metric_id, date)
			 DO UPDATE SET value = $3, updated = now()`,
			id, date, normalized); err != nil {
			return err
		}
	}

	if s.vals[id] == nil {
		s.vals[id] = make(map[string]string)
	}
	s.vals[id][date] = normalized
	return nil
}

// DeleteValue удаляет значение метрики за конкретный день.
func (s *MetricStore) DeleteValue(username string, id int, date string) error {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return errors.New("некорректная дата")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.defs[id]
	if !ok || d.Username != username {
		return errors.New("метрика не найдена")
	}
	if _, ok := s.vals[id][date]; !ok {
		return errors.New("значение за этот день не найдено")
	}

	if s.pool != nil {
		if _, err := s.pool.Exec(context.Background(),
			`DELETE FROM metric_values WHERE metric_id = $1 AND date = $2`,
			id, date); err != nil {
			return err
		}
	}
	delete(s.vals[id], date)
	return nil
}

// NormalizeMetricValue приводит введённое значение к каноническому виду
// в зависимости от типа метрики. Числовые метрики — неотрицательные:
// отрицательные значения отклоняются.
func NormalizeMetricValue(metricType, value string) (string, error) {
	switch metricType {
	case MetricTypeInt:
		v, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return "", errors.New("введите положительное целое число")
		}
		if v < 0 {
			return "", errors.New("значение не может быть отрицательным")
		}
		return strconv.Itoa(v), nil
	case MetricTypeFloat:
		v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return "", errors.New("введите положительное число (можно дробное)")
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "", errors.New("введите положительное число (можно дробное)")
		}
		if v < 0 {
			return "", errors.New("значение не может быть отрицательным")
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case MetricTypeBool:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "1", "да":
			return "true", nil
		case "false", "0", "нет":
			return "false", nil
		}
		return "", errors.New("введите «да» или «нет»")
	}
	return "", errors.New("некорректный тип метрики")
}
