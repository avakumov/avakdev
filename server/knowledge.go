package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Note — конспект знаний по конкретной теме. Генерируется через ИИ,
// удачные варианты сохраняются для повторения.
type Note struct {
	// ID — уникальный идентификатор конспекта.
	ID int `json:"id"`
	// Topic — тема, по которой сгенерирован конспект.
	Topic string `json:"topic"`
	// Title — короткий заголовок (уникален, часто равен теме).
	Title string `json:"title"`
	// Content — текст конспекта (Markdown).
	Content string `json:"content"`
	// Repetitions — счётчик выполненных повторений конспекта.
	// Увеличивается на единицу, когда пользователь нажал кнопку «Я повторил».
	// При создании всегда равен 0.
	Repetitions int `json:"repetitions"`
	// Created — время создания (RFC3339, UTC).
	Created string `json:"created"`
	// Updated — время последнего изменения (RFC3339, UTC).
	Updated string `json:"updated"`
}

// noteStore — хранилище конспектов знаний.
// Если PostgreSQL настроена, заметки хранятся в таблице knowledge_notes
// (и кэшируются в памяти). Иначе используется in-memory мапа без персистентности.
type noteStore struct {
	mu     sync.Mutex
	data   map[int]Note // кэш в памяти / хранилище без БД
	nextID int
	hasDB  bool
}

// notes — глобальное хранилище конспектов.
var notes *noteStore

// newNoteStore создаёт новое хранилище конспектов.
// Флаг hasDB определяется наличием подключения к базе (пакетная переменная db).
func newNoteStore() *noteStore {
	return &noteStore{
		data:   make(map[int]Note),
		nextID: 1,
		hasDB:  db != nil,
	}
}

// createNotesTableSQL создаёт таблицу knowledge_notes (идемпотентно).
const createNotesTableSQL = `
CREATE TABLE IF NOT EXISTS knowledge_notes (
	id          SERIAL PRIMARY KEY,
	topic       TEXT NOT NULL,
	title       TEXT NOT NULL,
	content     TEXT NOT NULL DEFAULT '',
	repetitions INTEGER NOT NULL DEFAULT 1,
	created     TIMESTAMP NOT NULL DEFAULT now(),
	updated     TIMESTAMP NOT NULL DEFAULT now()
);
`

// initKnowledge инициализирует глобальное хранилище конспектов.
// При наличии БД создаёт таблицу и подгружает уже сохранённые заметки в память.
func initKnowledge() error {
	notes = newNoteStore()
	if !notes.hasDB {
		return nil
	}

	if _, err := db.Exec(context.Background(), createNotesTableSQL); err != nil {
		return err
	}

	rows, err := db.Query(context.Background(),
		`SELECT id,
		        topic,
		        title,
		        content,
		        repetitions,
		        to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM knowledge_notes`)
	if err != nil {
		return err
	}
	defer rows.Close()

	maxID := 0
	for rows.Next() {
		var n Note
		if err := rows.Scan(&n.ID, &n.Topic, &n.Title, &n.Content,
			&n.Repetitions, &n.Created, &n.Updated); err != nil {
			return err
		}
		notes.data[n.ID] = n
		if n.ID > maxID {
			maxID = n.ID
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// Следующий autoincrement не ниже уже занятых ID.
	notes.nextID = maxID + 1
	return nil
}

// list возвращает все конспекты, отсортированные по дате обновления (новые сверху).
func (ns *noteStore) list() []Note {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	out := make([]Note, 0, len(ns.data))
	for _, n := range ns.data {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Updated > out[j].Updated })
	return out
}

// get возвращает конспект по ID.
func (ns *noteStore) get(id int) (Note, bool) {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	n, ok := ns.data[id]
	return n, ok
}

// create добавляет новый конспект. Счётчик повторений всегда стартует с нуля
// и увеличивается только через markRepeat (кнопка «Я повторил»).
// При наличии БД пишет в таблицу, иначе — только в память.
func (ns *noteStore) create(topic, title, content string) (Note, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	n := Note{
		Topic:       topic,
		Title:       title,
		Content:     content,
		Repetitions: 0,
		Created:     now,
		Updated:     now,
	}

	ns.mu.Lock()
	defer ns.mu.Unlock()

	if ns.hasDB {
		err := db.QueryRow(context.Background(),
			`INSERT INTO knowledge_notes (topic, title, content, repetitions)
			 VALUES ($1, $2, $3, 0)
			 RETURNING id, to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
			topic, title, content).
			Scan(&n.ID, &n.Created)
		if err != nil {
			return Note{}, err
		}
		if n.ID >= ns.nextID {
			ns.nextID = n.ID + 1
		}
	} else {
		n.ID = ns.nextID
		ns.nextID++
	}

	ns.data[n.ID] = n
	return n, nil
}

// update обновляет существующий конспект (title, content).
// Счётчик повторений здесь не меняется — он управляется только через markRepeat.
func (ns *noteStore) update(id int, title, content string) (Note, error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	n, ok := ns.data[id]
	if !ok {
		return Note{}, fmt.Errorf("конспект с ID %d не найден", id)
	}

	if title != "" {
		n.Title = title
	}
	if content != "" {
		n.Content = content
	}
	n.Updated = time.Now().UTC().Format(time.RFC3339)

	if ns.hasDB {
		if _, err := db.Exec(context.Background(),
			`UPDATE knowledge_notes
			 SET title = $2, content = $3, updated = now()
			 WHERE id = $1`,
			id, n.Title, n.Content); err != nil {
			return Note{}, err
		}
	}

	ns.data[id] = n
	return n, nil
}

// markRepeat увеличивает счётчик повторений конспекта на единицу.
// Вызывается при нажатии пользователем кнопки «Я повторил».
func (ns *noteStore) markRepeat(id int) (Note, error) {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	n, ok := ns.data[id]
	if !ok {
		return Note{}, fmt.Errorf("конспект с ID %d не найден", id)
	}

	n.Repetitions++
	n.Updated = time.Now().UTC().Format(time.RFC3339)

	if ns.hasDB {
		if _, err := db.Exec(context.Background(),
			`UPDATE knowledge_notes
			 SET repetitions = repetitions + 1, updated = now()
			 WHERE id = $1`, id); err != nil {
			return Note{}, err
		}
	}

	ns.data[id] = n
	return n, nil
}

// deleteByID удаляет конспект по ID.
func (ns *noteStore) deleteByID(id int) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()

	if _, ok := ns.data[id]; !ok {
		return fmt.Errorf("конспект с ID %d не найден", id)
	}

	if ns.hasDB {
		if _, err := db.Exec(context.Background(),
			`DELETE FROM knowledge_notes WHERE id = $1`, id); err != nil {
			return err
		}
	}

	delete(ns.data, id)
	return nil
}

// handleListNotes возвращает список всех конспектов.
func handleListNotes(c *gin.Context) {
	c.JSON(http.StatusOK, notes.list())
}

// handleCreateNote создаёт новый конспект. Счётчик повторений стартует с нуля.
func handleCreateNote(c *gin.Context) {
	var req struct {
		Topic   string `json:"topic"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	req.Topic = strings.TrimSpace(req.Topic)
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		req.Title = req.Topic
	}
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Укажите тему или заголовок конспекта"})
		return
	}

	n, err := notes.create(req.Topic, req.Title, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить конспект"})
		return
	}
	c.JSON(http.StatusOK, n)
}

// handleUpdateNote обновляет конспект по ID (title, content).
// Счётчик повторений пользователь меняет только через handleRepeatNote.
func handleUpdateNote(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID"})
		return
	}

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}

	n, err := notes.update(id, strings.TrimSpace(req.Title), req.Content)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, n)
}

// handleRepeatNote увеличивает счётчик повторений конспекта на единицу.
func handleRepeatNote(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID"})
		return
	}

	n, err := notes.markRepeat(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, n)
}

// handleDeleteNote удаляет конспект по ID.
func handleDeleteNote(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID"})
		return
	}
	if err := notes.deleteByID(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleGenerateNote вызывает DeepSeek для создания краткого конспекта по теме.
func handleGenerateNote(c *gin.Context) {
	var req struct {
		Topic string `json:"topic"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	req.Topic = strings.TrimSpace(req.Topic)
	if req.Topic == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Укажите тему"})
		return
	}

	apiKey := deepseekAPIKey()
	if apiKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Ключ DeepSeek не настроен (DEEPSEEK_API_KEY в .env)",
		})
		return
	}

	content, err := generateNoteContent(req.Topic, apiKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"topic":   req.Topic,
		"title":   req.Topic,
		"content": content,
	})
}

// generateNoteContent вызывает DeepSeek chat API для создания краткого конспекта
// по заданной теме. Возвращает текст конспекта в Markdown.
func generateNoteContent(topic, apiKey string) (string, error) {
	systemPrompt := "Ты — помощник для учёбы и повторения материала. " +
		"Составь КРАТКИЙ конспект по заданной теме на русском языке, объёмом " +
		"примерно 300–600 слов, в формате Markdown. " +
		"Структура: заголовок темы, несколько пунктов с короткими пояснениями, " +
		"ключевые термины, выделенные **жирным**, и в конце — 3–5 контрольных вопросов " +
		"для самопроверки. Конспект должен быть лаконичным и хорошо запоминаться. " +
		"Верни только Markdown-текст без пояснений вне него."

	payload := map[string]any{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": topic},
		},
		"stream":      false,
		"temperature": 0.7,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.deepseek.com/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ошибка вызова DeepSeek: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DeepSeek вернул статус %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("DeepSeek не вернул ответ")
	}
	return strings.TrimSpace(sanitizeMarkdownAnswer(parsed.Choices[0].Message.Content)), nil
}

// sanitizeMarkdownAnswer подчищает ответ DeepSeek: убирает возможные
// markdown-блоки (``` ... ```) и возвращает только Markdown-контент.
func sanitizeMarkdownAnswer(s string) string {
	s = strings.TrimSpace(s)
	lines := strings.Split(s, "\n")
	if len(lines) >= 2 {
		first := strings.TrimSpace(lines[0])
		last := strings.TrimSpace(lines[len(lines)-1])
		if strings.HasPrefix(first, "```") {
			s = strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
		}
		if strings.HasPrefix(last, "```") && strings.HasSuffix(s, "```") {
			s = strings.TrimSpace(strings.TrimSuffix(s, "```"))
		}
	}
	return s
}
