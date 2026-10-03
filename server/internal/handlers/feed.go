package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"avakumov/server/internal/app"
)

// Раздел «Лента»: элементы ленты пользователя. Первый тип контента —
// «вопрос-ответ» (kind = qa): вопрос, ответ и счётчик показов.
// Наполняют ленту в разделе меню «Лента» (FeedEdit), читают — свайпом
// на мобильных (Feed): сначала вопрос, ответ — после касания.

// Типы контента ленты. Пока единственный — «вопрос-ответ».
const feedKindQA = "qa"

// Сколько элементов можно сгенерировать/сохранить за раз.
const (
	feedDefaultGenerateCount = 5
	maxFeedGenerateCount     = 50
)

// feedDraft — черновик элемента ленты, который предлагает ИИ (ещё не в БД).
type feedDraft struct {
	Topic    string `json:"topic"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Details  string `json:"details"`
}

// Предельные размеры полей элемента (символов).
const (
	maxFeedTopicRunes    = 40
	maxFeedQuestionRunes = 2000
	maxFeedAnswerRunes   = 20000
	maxFeedDetailsRunes  = 20000
)

// feedPayload проверяет и нормализует поля элемента ленты.
// Раздел (topic), вопрос и ответ обязательны; объяснение (details) — нет.
func feedPayload(kind, topic, question, answer, details string) (string, string, string, string, string, error) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		kind = feedKindQA
	}
	if kind != feedKindQA {
		return "", "", "", "", "", errors.New("неизвестный тип контента")
	}
	t := strings.TrimSpace(topic)
	if t == "" {
		return "", "", "", "", "", errors.New("укажите раздел")
	}
	if len([]rune(t)) > maxFeedTopicRunes {
		return "", "", "", "", "", errors.New("раздел слишком длинный")
	}
	q := strings.TrimSpace(question)
	a := strings.TrimSpace(answer)
	d := strings.TrimSpace(details)
	if q == "" {
		return "", "", "", "", "", errors.New("укажите вопрос")
	}
	if len([]rune(q)) > maxFeedQuestionRunes {
		return "", "", "", "", "", errors.New("вопрос слишком длинный")
	}
	if a == "" {
		return "", "", "", "", "", errors.New("укажите ответ")
	}
	if len([]rune(a)) > maxFeedAnswerRunes {
		return "", "", "", "", "", errors.New("ответ слишком длинный")
	}
	if len([]rune(d)) > maxFeedDetailsRunes {
		return "", "", "", "", "", errors.New("объяснение слишком длинное")
	}
	return kind, t, q, a, d, nil
}

// ListFeed возвращает элементы ленты пользователя (свежие сверху).
func (h *Handlers) ListFeed(w http.ResponseWriter, r *http.Request) {
	sessData, _ := sessionOf(r)
	out, err := h.App.Feed.List(context.Background(), sessData.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось загрузить ленту"})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// CreateFeedItem добавляет элемент ленты.
// Тело: {"kind": "qa", "topic": "golang", "question": "...", "answer": "...",
//
//	"details": "..."}
func (h *Handlers) CreateFeedItem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind     string `json:"kind"`
		Topic    string `json:"topic"`
		Question string `json:"question"`
		Answer   string `json:"answer"`
		Details  string `json:"details"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	kind, topic, question, answer, details, err := feedPayload(req.Kind, req.Topic, req.Question, req.Answer, req.Details)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	sessData, _ := sessionOf(r)

	it, err := h.App.Feed.Create(context.Background(), sessData.Username, kind, topic, question, answer, details)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось сохранить элемент ленты"})
		return
	}
	writeJSON(w, http.StatusOK, it)
}

// UpdateFeedItem меняет раздел, вопрос, ответ и объяснение (показы не трогаем).
func (h *Handlers) UpdateFeedItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID элемента"})
		return
	}
	var req struct {
		Kind     string `json:"kind"`
		Topic    string `json:"topic"`
		Question string `json:"question"`
		Answer   string `json:"answer"`
		Details  string `json:"details"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	kind, topic, question, answer, details, err := feedPayload(req.Kind, req.Topic, req.Question, req.Answer, req.Details)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	sessData, _ := sessionOf(r)

	it, err := h.App.Feed.Update(context.Background(), sessData.Username, id, kind, topic, question, answer, details)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Элемент ленты не найден"})
		return
	}
	writeJSON(w, http.StatusOK, it)
}

// DeleteFeedItem удаляет элемент ленты (счётчик показов уходит с ним).
func (h *Handlers) DeleteFeedItem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID элемента"})
		return
	}
	sessData, _ := sessionOf(r)
	ok, err := h.App.Feed.Delete(context.Background(), sessData.Username, id)
	if err != nil || !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Элемент ленты не найден"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// BulkCreateFeedItems сохраняет сразу несколько элементов ленты одним
// запросом (кнопка «Сохранить» после ИИ-генерации).
// Тело: {"items": [{"topic": "golang", "question": "...", "answer": "...",
//
//	"details": "..."}]}
func (h *Handlers) BulkCreateFeedItems(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Items []struct {
			Kind     string `json:"kind"`
			Topic    string `json:"topic"`
			Question string `json:"question"`
			Answer   string `json:"answer"`
			Details  string `json:"details"`
		} `json:"items"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	if len(req.Items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Нет элементов для сохранения"})
		return
	}
	if len(req.Items) > maxFeedGenerateCount {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Слишком много элементов за раз"})
		return
	}

	// Проверяем всё до записи: либо сохраняем всю пачку, либо ничего.
	topics := make([]string, 0, len(req.Items))
	questions := make([]string, 0, len(req.Items))
	answers := make([]string, 0, len(req.Items))
	details := make([]string, 0, len(req.Items))
	for _, it := range req.Items {
		_, topic, question, answer, extra, err := feedPayload(it.Kind, it.Topic, it.Question, it.Answer, it.Details)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		topics = append(topics, topic)
		questions = append(questions, question)
		answers = append(answers, answer)
		details = append(details, extra)
	}

	sessData, _ := sessionOf(r)
	out, err := h.App.Feed.BulkCreate(context.Background(), sessData.Username, feedKindQA, topics, questions, answers, details)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось сохранить элементы ленты"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// GenerateFeedItems генерирует черновики элементов ленты через DeepSeek.
// Ничего не сохраняет — возвращает список, который пользователь чистит и
// сохраняет отдельно (как черновики задач цели).
// Тело: {"topic": "...", "description": "...", "count": 5}
func (h *Handlers) GenerateFeedItems(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Topic       string `json:"topic"`
		Description string `json:"description"`
		Count       int    `json:"count"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}
	req.Topic = strings.TrimSpace(req.Topic)
	if req.Topic == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Сначала укажите тему"})
		return
	}
	if req.Count <= 0 {
		req.Count = feedDefaultGenerateCount
	}
	if req.Count > maxFeedGenerateCount {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": fmt.Sprintf("За раз можно создать не больше %d элементов", maxFeedGenerateCount),
		})
		return
	}

	apiKey := app.DeepSeekAPIKey()
	if apiKey == "" {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"error": "Ключ DeepSeek не настроен (DEEPSEEK_API_KEY в .env)",
		})
		return
	}

	drafts, truncated, err := aiGenerateFeedItems(req.Topic, req.Description, req.Count, apiKey)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": drafts, "truncated": truncated})
}

// aiGenerateFeedItems просит модель придумать элементы ленты по теме.
// Возвращает черновики; на БД они не сохраняются. Признак truncated — ответ
// модели обрезан по лимиту вывода, то есть элементов может не хватать.
func aiGenerateFeedItems(topic, description string, count int, apiKey string) (drafts []feedDraft, truncated bool, err error) {
	topicText := "Тема: " + topic
	if trimmed := strings.TrimSpace(description); trimmed != "" {
		topicText += "\nОписание: " + trimmed
	}

	systemPrompt := "Ты — составитель карточек для ленты «вопрос-ответ» (как карточки для запоминания). " +
		fmt.Sprintf("Создай ровно %d элементов — ни больше ни меньше. ", count) +
		"Верни СТРОГО валидный JSON без текста вне него, вида: " +
		"{\"items\": [{\"topic\": string, \"question\": string, \"answer\": string, \"details\": string}]}. " +
		"Правила: topic — раздел (область) этого элемента, ОДНО короткое слово в нижнем регистре " +
		"(например: golang, linux, ооп, sql, git); " +
		"question — конкретный вопрос по теме (до 140 символов); " +
		// По умолчанию ответ — одно предложение: глубина уходит в details.
		// Если в описании явно просят развёрнутый ответ — разрешаем 2–3.
		"answer — точный, самодостаточный ответ ОДНИМ предложением (до 200 символов); " +
		"только если в описании явно просят развёрнутый или подробный ответ — допустимо 2–3 предложения (до 400 символов); " +
		"details — объяснение и примеры в Markdown: пояснение (2–4 предложения) и, если уместно, " +
		"1–2 примера кода в ограждённых блоках с языком (```go … ```); до 1200 символов, " +
		"без заголовков первого уровня; если пояснять нечего — пустая строка; " +
		"весь текст (question, answer, details) — валидный Markdown, код только внутри ограждённых блоков; " +
		"каждый элемент раскрывает свой аспект темы, повторов и общих фраз не должно быть. " +
		"Язык — тот же, что у темы и описания."

	payload := map[string]any{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": topicText},
		},
		"stream": false,
		// Явный лимит вывода: чтобы большая пачка элементов не обрезалась.
		"max_tokens":      8192,
		"temperature":     0.7,
		"response_format": map[string]string{"type": "json_object"},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, false, err
	}

	hreq, err := http.NewRequest(http.MethodPost, "https://api.deepseek.com/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(hreq)
	if err != nil {
		return nil, false, fmt.Errorf("ошибка вызова DeepSeek: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("DeepSeek вернул статус %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, false, err
	}
	if len(parsed.Choices) == 0 {
		return nil, false, fmt.Errorf("DeepSeek не вернул ответ")
	}

	choice := parsed.Choices[0]
	truncated = choice.FinishReason == "length"
	drafts, err = parseFeedDrafts(choice.Message.Content)
	// Пишем в лог, сколько просили и сколько вернула модель: по этому логу видно,
	// если ответ обрезан или модель проигнорировала количество.
	log.Printf("FEED: элементов запрошено: %d, получено: %d, finish_reason=%s, tokens=%d",
		count, len(drafts), choice.FinishReason, parsed.Usage.CompletionTokens)
	if err != nil {
		if truncated {
			return nil, true, fmt.Errorf("ответ модели обрезан по лимиту длины — попробуйте ещё раз или уменьшите число элементов")
		}
		return nil, false, err
	}
	return drafts, truncated, nil
}

// parseFeedDrafts разбирает JSON-ответ модели в черновики элементов ленты.
// Количество не ограничиваем: сколько вернула модель, столько и отдаём
// (лишнее пользователь удалит сам).
func parseFeedDrafts(content string) ([]feedDraft, error) {
	// Модель может обернуть JSON в ```json ... ``` — вырезаем содержимое.
	s := strings.TrimSpace(content)
	if start := strings.Index(s, "{"); start >= 0 {
		if end := strings.LastIndex(s, "}"); end > start {
			s = s[start : end+1]
		}
	}

	var parsed struct {
		Items []struct {
			Topic    string `json:"topic"`
			Question string `json:"question"`
			Answer   string `json:"answer"`
			Details  string `json:"details"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		return nil, fmt.Errorf("не удалось разобрать ответ модели: %w", err)
	}

	out := make([]feedDraft, 0, len(parsed.Items))
	for _, it := range parsed.Items {
		topic := strings.TrimSpace(it.Topic)
		question := strings.TrimSpace(it.Question)
		answer := strings.TrimSpace(it.Answer)
		details := strings.TrimSpace(it.Details)
		// Раздел обязателен: если модель его не дала, берём общий — «Прочее»,
		// чтобы элемент всё равно можно было сохранить.
		if topic == "" {
			topic = "Прочее"
		}
		if len([]rune(topic)) > maxFeedTopicRunes {
			topic = string([]rune(topic)[:maxFeedTopicRunes])
		}
		if question == "" || answer == "" {
			continue
		}
		if len([]rune(question)) > maxFeedQuestionRunes {
			question = string([]rune(question)[:maxFeedQuestionRunes])
		}
		if len([]rune(answer)) > maxFeedAnswerRunes {
			answer = string([]rune(answer)[:maxFeedAnswerRunes])
		}
		if len([]rune(details)) > maxFeedDetailsRunes {
			details = string([]rune(details)[:maxFeedDetailsRunes])
		}
		out = append(out, feedDraft{
			Topic: topic, Question: question, Answer: answer, Details: details,
		})
	}
	return out, nil
}

// FeedItemReaction фиксирует реакцию на элемент ленты: «знаю» (know)
// или «не знаю» (unknown) — соответствующий счётчик +1.
// Тело: {"value": "know" | "unknown"}
func (h *Handlers) FeedItemReaction(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID элемента"})
		return
	}
	var req struct {
		Value string `json:"value"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}

	var know bool
	switch strings.TrimSpace(req.Value) {
	case "know":
		know = true
	case "unknown":
		know = false
	default:
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректная реакция"})
		return
	}

	sessData, _ := sessionOf(r)
	it, err := h.App.Feed.React(context.Background(), sessData.Username, id, know)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Элемент ленты не найден"})
		return
	}
	writeJSON(w, http.StatusOK, it)
}

// FeedItemView отмечает показ элемента в ленте: views = views + 1.
// Вызывается, когда элемент показан в ленте; текст при этом не меняется.
func (h *Handlers) FeedItemView(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(param(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный ID элемента"})
		return
	}
	sessData, _ := sessionOf(r)
	it, err := h.App.Feed.View(context.Background(), sessData.Username, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "Элемент ленты не найден"})
		return
	}
	writeJSON(w, http.StatusOK, it)
}
