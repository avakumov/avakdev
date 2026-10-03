package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"avakumov/server/internal/app"
)

// ListNotes возвращает все конспекты с временем чтения, вычисленным
// по скорости чтения текущего пользователя (см. app.ReadingMinutes).
func (h *Handlers) ListNotes(w http.ResponseWriter, r *http.Request) {
	username := ""
	if sessData, ok := sessionOf(r); ok {
		username = sessData.Username
	}
	speed := h.App.ReadingSpeed(username)

	all := h.App.Knowledge.List()
	out := make([]app.Note, len(all))
	for i, n := range all {
		n.ReadingMinutes = app.ReadingMinutes(n.Content, speed)
		out[i] = n
	}
	writeJSON(w, http.StatusOK, out)
}

// CreateNote создаёт новый конспект. Счётчик повторений стартует с нуля.
func (h *Handlers) CreateNote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Topic   string `json:"topic"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	req.Topic = strings.TrimSpace(req.Topic)
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		req.Title = req.Topic
	}
	if req.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Укажите тему или заголовок конспекта"})
		return
	}

	n, err := h.App.Knowledge.Create(req.Topic, req.Title, req.Content)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось сохранить конспект"})
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// UpdateNote обновляет конспект по ID (title, content).
// Счётчик повторений пользователь меняет только через RepeatNote.
func (h *Handlers) UpdateNote(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID"})
		return
	}

	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}

	n, err := h.App.Knowledge.Update(id, strings.TrimSpace(req.Title), req.Content)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// RepeatNote увеличивает счётчик повторений конспекта на единицу.
func (h *Handlers) RepeatNote(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID"})
		return
	}

	n, err := h.App.Knowledge.MarkRepeat(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// DeleteNote удаляет конспект по ID.
func (h *Handlers) DeleteNote(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID"})
		return
	}
	if err := h.App.Knowledge.DeleteByID(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// GenerateNote вызывает DeepSeek для создания краткого конспекта по теме.
func (h *Handlers) GenerateNote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Topic string `json:"topic"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	req.Topic = strings.TrimSpace(req.Topic)
	if req.Topic == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Укажите тему"})
		return
	}

	apiKey := app.DeepSeekAPIKey()
	if apiKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "Ключ DeepSeek не настроен (DEEPSEEK_API_KEY в .env)",
		})
		return
	}

	content, err := generateNoteContent(req.Topic, apiKey)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
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
