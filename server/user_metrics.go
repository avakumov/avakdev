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
	"github.com/jackc/pgx/v5/pgconn"
)

// Типы метрик: целое число, дробное число, да/нет (boolean).
const (
	metricTypeInt   = "int"
	metricTypeFloat = "float"
	metricTypeBool  = "bool"
)

// validMetricTypes — допустимые значения поля type.
var validMetricTypes = map[string]bool{
	metricTypeInt:   true,
	metricTypeFloat: true,
	metricTypeBool:  true,
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

// metricValue — значение метрики за день. Одно значение на (метрика, дата):
// повторное сохранение за тот же день перезаписывает его.
type metricValue struct {
	MetricID int    `json:"metric_id"`
	Date     string `json:"date"`
	Value    string `json:"value"`
}

// metricStore — хранилище метрик пользователей: определения и значения.
type metricStore struct {
	mu     sync.Mutex
	defs   map[int]MetricDef         // id -> определение
	vals   map[int]map[string]string // metric_id -> (YYYY-MM-DD -> значение)
	nextID int
	hasDB  bool
}

// userMetrics — глобальное хранилище метрик.
var userMetrics *metricStore

// initMetrics инициализирует глобальное хранилище метрик.
// При наличии БД подгружает определения и значения в память.
// (Таблицы создаются версионированными миграциями goose, см. migrations/.)
func initMetrics() error {
	userMetrics = &metricStore{
		defs:   make(map[int]MetricDef),
		vals:   make(map[int]map[string]string),
		nextID: 1,
		hasDB:  db != nil,
	}
	if !userMetrics.hasDB {
		return nil
	}

	rows, err := db.Query(context.Background(),
		`SELECT id,
		        username,
		        name,
		        type,
		        unit,
		        to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM metric_definitions`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var d MetricDef
		if err := rows.Scan(&d.ID, &d.Username, &d.Name, &d.Type, &d.Unit, &d.Created); err != nil {
			return err
		}
		userMetrics.defs[d.ID] = d
		if d.ID >= userMetrics.nextID {
			userMetrics.nextID = d.ID + 1
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	valueRows, err := db.Query(context.Background(),
		`SELECT metric_id, to_char(date, 'YYYY-MM-DD'), value FROM metric_values`)
	if err != nil {
		return err
	}
	defer valueRows.Close()
	for valueRows.Next() {
		var id int
		var date, value string
		if err := valueRows.Scan(&id, &date, &value); err != nil {
			return err
		}
		if userMetrics.vals[id] == nil {
			userMetrics.vals[id] = make(map[string]string)
		}
		userMetrics.vals[id][date] = value
	}
	return valueRows.Err()
}

// list возвращает определения метрик пользователя (в порядке создания).
func (s *metricStore) list(username string) []MetricDef {
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

// getOwned возвращает определение метрики, если она принадлежит пользователю.
func (s *metricStore) getOwned(username string, id int) (MetricDef, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.defs[id]
	if !ok || d.Username != username {
		return MetricDef{}, false
	}
	return d, true
}

// values возвращает копию значений метрики: дата -> значение.
func (s *metricStore) values(metricID int) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string)
	for date, v := range s.vals[metricID] {
		out[date] = v
	}
	return out
}

// create добавляет новую метрику пользователя (название + тип + единица).
// Единица измерения применима только к числовым метрикам; для «да/нет» — пусто.
func (s *metricStore) create(username, name, metricType, unit string) (MetricDef, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return MetricDef{}, errors.New("укажите название метрики")
	}
	if !validMetricTypes[metricType] {
		return MetricDef{}, errors.New("некорректный тип метрики")
	}
	unit = strings.TrimSpace(unit)
	if metricType == metricTypeBool {
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

	if s.hasDB {
		err := db.QueryRow(context.Background(),
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

// update переименовывает метрику пользователя и меняет её единицу измерения.
// Тип метрики не меняется: значения уже записаны в формате своего типа.
func (s *metricStore) update(username string, id int, name, unit string) (MetricDef, error) {
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
	if d.Type == metricTypeBool {
		unit = ""
	}
	d.Name = name
	d.Unit = unit

	if s.hasDB {
		if _, err := db.Exec(context.Background(),
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

// delete удаляет метрику пользователя вместе со всеми её значениями.
func (s *metricStore) delete(username string, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.defs[id]
	if !ok || d.Username != username {
		return errors.New("метрика не найдена")
	}
	if s.hasDB {
		if _, err := db.Exec(context.Background(),
			`DELETE FROM metric_definitions WHERE id = $1`, id); err != nil {
			return err
		}
	}
	delete(s.defs, id)
	delete(s.vals, id)
	return nil
}

// setValue сохраняет значение метрики за день (создаёт или перезаписывает —
// за день фиксируется один показатель). Значение валидируется по типу метрики.
func (s *metricStore) setValue(username string, id int, date, value string) error {
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
	normalized, err := normalizeMetricValue(d.Type, value)
	if err != nil {
		return err
	}

	if s.hasDB {
		if _, err := db.Exec(context.Background(),
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

// deleteValue удаляет значение метрики за конкретный день.
func (s *metricStore) deleteValue(username string, id int, date string) error {
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

	if s.hasDB {
		if _, err := db.Exec(context.Background(),
			`DELETE FROM metric_values WHERE metric_id = $1 AND date = $2`,
			id, date); err != nil {
			return err
		}
	}
	delete(s.vals[id], date)
	return nil
}

// normalizeMetricValue приводит введённое значение к каноническому виду
// в зависимости от типа метрики.
func normalizeMetricValue(metricType, value string) (string, error) {
	switch metricType {
	case metricTypeInt:
		v, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return "", errors.New("введите целое число")
		}
		return strconv.Itoa(v), nil
	case metricTypeFloat:
		v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return "", errors.New("введите число (можно дробное)")
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case metricTypeBool:
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

// handleListUserMetrics отдаёт определения метрик пользователя и все их значения.
func handleListUserMetrics(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)
	defs := userMetrics.list(sessData.username)

	values := make([]metricValue, 0)
	for _, d := range defs {
		for date, value := range userMetrics.values(d.ID) {
			values = append(values, metricValue{MetricID: d.ID, Date: date, Value: value})
		}
	}
	c.JSON(http.StatusOK, gin.H{"definitions": defs, "values": values})
}

// handleCreateUserMetric создаёт новую метрику пользователя.
func handleCreateUserMetric(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Unit string `json:"unit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	d, err := userMetrics.create(sessData.username, req.Name, req.Type, req.Unit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, d)
}

// handleUpdateUserMetric переименовывает метрику и меняет её единицу измерения.
func handleUpdateUserMetric(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID метрики"})
		return
	}
	var req struct {
		Name string `json:"name"`
		Unit string `json:"unit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	d, err := userMetrics.update(sessData.username, id, req.Name, req.Unit)
	if err != nil {
		status := http.StatusBadRequest
		if err.Error() == "метрика не найдена" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, d)
}

// handleDeleteUserMetric удаляет метрику пользователя (со значениями).
func handleDeleteUserMetric(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID метрики"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	if err := userMetrics.delete(sessData.username, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleSetUserMetricValue сохраняет показатель метрики за день.
func handleSetUserMetricValue(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID метрики"})
		return
	}
	date := c.Param("date")

	var req struct {
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	if err := userMetrics.setValue(sessData.username, id, date, req.Value); err != nil {
		status := http.StatusBadRequest
		if err.Error() == "метрика не найдена" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"metric_id": id, "date": date, "value": req.Value})
}

// handleDeleteUserMetricValue удаляет показатель метрики за день.
func handleDeleteUserMetricValue(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID метрики"})
		return
	}
	date := c.Param("date")
	sessData, _ := c.MustGet("session").(session)
	if err := userMetrics.deleteValue(sessData.username, id, date); err != nil {
		status := http.StatusBadRequest
		if err.Error() == "метрика не найдена" {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
