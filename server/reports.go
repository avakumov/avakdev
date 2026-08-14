package main

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Report — дневной отчёт пользователя.
type Report struct {
	// Date — день отчёта в формате YYYY-MM-DD.
	Date string `json:"date"`
	// Content — текст отчёта.
	Content string `json:"content"`
	// Updated — время последнего изменения (RFC3339, UTC).
	Updated string `json:"updated"`
}

// reportStore — хранилище дневных отчётов.
// Если база данных PostgreSQL настроена, отчёты хранятся в таблице reports
// (и кэшируются в памяти). Иначе используется in-memory мапа без персистентности.
type reportStore struct {
	mu    sync.Mutex
	data  map[string]Report // кэш в памяти / хранилище без БД
	hasDB bool
}

// reports — глобальное хранилище отчётов.
var reports *reportStore

// newReportStore создаёт новое хранилище отчётов.
// Флаг hasDB определяется наличием подключения к базе (пакетная переменная db).
func newReportStore() *reportStore {
	return &reportStore{
		data:  make(map[string]Report),
		hasDB: db != nil,
	}
}

// createReportsTableSQL создаёт таблицу reports (идемпотентно).
const createReportsTableSQL = `
CREATE TABLE IF NOT EXISTS reports (
	date    DATE NOT NULL UNIQUE,
	content TEXT NOT NULL DEFAULT '',
	updated TIMESTAMP NOT NULL DEFAULT now()
);
`

// initReports инициализирует глобальное хранилище отчётов.
// При наличии БД создаёт таблицу и подгружает уже сохранённые отчёты в память.
func initReports() error {
	reports = newReportStore()
	if !reports.hasDB {
		return nil
	}

	if _, err := db.Exec(context.Background(), createReportsTableSQL); err != nil {
		return err
	}

	rows, err := db.Query(context.Background(),
		`SELECT to_char(date,'YYYY-MM-DD'),
		        content,
		        to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM reports`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var r Report
		if err := rows.Scan(&r.Date, &r.Content, &r.Updated); err != nil {
			return err
		}
		reports.data[r.Date] = r
	}
	return rows.Err()
}

// list возвращает все отчёты, отсортированные по дате (новые сверху).
func (rs *reportStore) list() []Report {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	out := make([]Report, 0, len(rs.data))
	for _, r := range rs.data {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date > out[j].Date })
	return out
}

// upsert создаёт или обновляет отчёт за указанный день.
// При наличии БД пишет в таблицу, иначе — только в память.
func (rs *reportStore) upsert(date, content string) (Report, error) {
	r := Report{
		Date:    date,
		Content: content,
		Updated: time.Now().UTC().Format(time.RFC3339),
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.hasDB {
		if _, err := db.Exec(context.Background(),
			`INSERT INTO reports (date, content, updated)
			 VALUES ($1, $2, now())
			 ON CONFLICT (date)
			 DO UPDATE SET content = $2, updated = now()`,
			date, content); err != nil {
			return Report{}, err
		}
	}

	rs.data[date] = r
	return r, nil
}

// handleListReports возвращает список отчётов.
func handleListReports(c *gin.Context) {
	c.JSON(http.StatusOK, reports.list())
}

// handleUpsertReport создаёт или обновляет отчёт за конкретный день.
// Параметр :date — день в формате YYYY-MM-DD.
func handleUpsertReport(c *gin.Context) {
	date := c.Param("date")
	if date == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Не указана дата"})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}

	r, err := reports.upsert(date, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить отчёт"})
		return
	}
	c.JSON(http.StatusOK, r)
}
