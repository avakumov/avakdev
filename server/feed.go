package main

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

	"github.com/gin-gonic/gin"
)

// Раздел «Лента»: элементы ленты пользователя. Первый тип контента —
// «вопрос-ответ» (kind = qa): вопрос, ответ и счётчик показов.
// Наполняют ленту в разделе меню «Лента» (FeedEdit), читают — свайпом
// на мобильных (Feed): сначала вопрос, ответ — после касания.

// FeedItem — элемент ленты.
type FeedItem struct {
	ID   int    `json:"id"`
	Kind string `json:"kind"`
	// Topic — раздел (область) элемента: короткое слово вроде «golang».
	Topic    string `json:"topic"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Views    int    `json:"views"`
	// KnowCount/UnknownCount — сколько раз отмечено «знаю» / «не знаю».
	KnowCount    int    `json:"know_count"`
	UnknownCount int    `json:"unknown_count"`
	Created      string `json:"created"`
	Updated      string `json:"updated"`
}

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
}

// Предельные размеры полей элемента (символов).
const (
	maxFeedTopicRunes    = 40
	maxFeedQuestionRunes = 2000
	maxFeedAnswerRunes   = 20000
)

// Единый формат времени элемента (RFC3339, UTC).
const (
	feedCreatedExpr = `to_char(created AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`
	feedUpdatedExpr = `to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')`
	feedSelectCols  = `id, kind, topic, question, answer, views, know_count, unknown_count, ` +
		feedCreatedExpr + `, ` + feedUpdatedExpr
)

// feedPayload проверяет и нормализует поля элемента ленты.
// Раздел (topic), вопрос и ответ обязательны.
func feedPayload(kind, topic, question, answer string) (string, string, string, string, error) {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		kind = feedKindQA
	}
	if kind != feedKindQA {
		return "", "", "", "", errors.New("неизвестный тип контента")
	}
	t := strings.TrimSpace(topic)
	if t == "" {
		return "", "", "", "", errors.New("укажите раздел")
	}
	if len([]rune(t)) > maxFeedTopicRunes {
		return "", "", "", "", errors.New("раздел слишком длинный")
	}
	q := strings.TrimSpace(question)
	a := strings.TrimSpace(answer)
	if q == "" {
		return "", "", "", "", errors.New("укажите вопрос")
	}
	if len([]rune(q)) > maxFeedQuestionRunes {
		return "", "", "", "", errors.New("вопрос слишком длинный")
	}
	if a == "" {
		return "", "", "", "", errors.New("укажите ответ")
	}
	if len([]rune(a)) > maxFeedAnswerRunes {
		return "", "", "", "", errors.New("ответ слишком длинный")
	}
	return kind, t, q, a, nil
}

// feedScan собирает FeedItem из строки результата.
// Порядок полей — как в feedSelectCols.
func feedScan(row interface{ Scan(...any) error }) (FeedItem, error) {
	var it FeedItem
	err := row.Scan(&it.ID, &it.Kind, &it.Topic, &it.Question, &it.Answer, &it.Views,
		&it.KnowCount, &it.UnknownCount, &it.Created, &it.Updated)
	return it, err
}

// handleListFeed возвращает элементы ленты пользователя (свежие сверху).
func handleListFeed(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)
	rows, err := db.Query(context.Background(),
		`SELECT `+feedSelectCols+`
		 FROM feed_items WHERE username = $1
		 ORDER BY id DESC`, sessData.username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось загрузить ленту"})
		return
	}
	defer rows.Close()

	out := make([]FeedItem, 0)
	for rows.Next() {
		if it, err := feedScan(rows); err == nil {
			out = append(out, it)
		}
	}
	c.JSON(http.StatusOK, out)
}

// handleCreateFeedItem добавляет элемент ленты.
// Тело: {"kind": "qa", "topic": "golang", "question": "...", "answer": "..."}
func handleCreateFeedItem(c *gin.Context) {
	var req struct {
		Kind     string `json:"kind"`
		Topic    string `json:"topic"`
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	kind, topic, question, answer, err := feedPayload(req.Kind, req.Topic, req.Question, req.Answer)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sessData, _ := c.MustGet("session").(session)

	it, err := feedScan(db.QueryRow(context.Background(),
		`INSERT INTO feed_items (username, kind, topic, question, answer)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+feedSelectCols,
		sessData.username, kind, topic, question, answer))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить элемент ленты"})
		return
	}
	c.JSON(http.StatusOK, it)
}

// handleUpdateFeedItem меняет раздел, вопрос и ответ элемента (показы не трогаем).
func handleUpdateFeedItem(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID элемента"})
		return
	}
	var req struct {
		Kind     string `json:"kind"`
		Topic    string `json:"topic"`
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	kind, topic, question, answer, err := feedPayload(req.Kind, req.Topic, req.Question, req.Answer)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sessData, _ := c.MustGet("session").(session)

	it, err := feedScan(db.QueryRow(context.Background(),
		`UPDATE feed_items
		 SET kind = $3, topic = $4, question = $5, answer = $6, updated = now()
		 WHERE id = $1 AND username = $2
		 RETURNING `+feedSelectCols,
		id, sessData.username, kind, topic, question, answer))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Элемент ленты не найден"})
		return
	}
	c.JSON(http.StatusOK, it)
}

// handleDeleteFeedItem удаляет элемент ленты (счётчик показов уходит с ним).
func handleDeleteFeedItem(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID элемента"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	tag, err := db.Exec(context.Background(),
		`DELETE FROM feed_items WHERE id = $1 AND username = $2`, id, sessData.username)
	if err != nil || tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Элемент ленты не найден"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// handleBulkCreateFeedItems сохраняет сразу несколько элементов ленты одним
// запросом (кнопка «Сохранить» после ИИ-генерации).
// Тело: {"items": [{"topic": "golang", "question": "...", "answer": "..."}]}
func handleBulkCreateFeedItems(c *gin.Context) {
	var req struct {
		Items []struct {
			Kind     string `json:"kind"`
			Topic    string `json:"topic"`
			Question string `json:"question"`
			Answer   string `json:"answer"`
		} `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Нет элементов для сохранения"})
		return
	}
	if len(req.Items) > maxFeedGenerateCount {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Слишком много элементов за раз"})
		return
	}

	// Проверяем всё до записи: либо сохраняем всю пачку, либо ничего.
	topics := make([]string, 0, len(req.Items))
	questions := make([]string, 0, len(req.Items))
	answers := make([]string, 0, len(req.Items))
	for _, it := range req.Items {
		_, topic, question, answer, err := feedPayload(it.Kind, it.Topic, it.Question, it.Answer)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		topics = append(topics, topic)
		questions = append(questions, question)
		answers = append(answers, answer)
	}

	sessData, _ := c.MustGet("session").(session)
	rows, err := db.Query(context.Background(),
		`INSERT INTO feed_items (username, kind, topic, question, answer)
		 SELECT $1, $2, t, q, a
		 FROM unnest($3::text[], $4::text[], $5::text[]) AS x(t, q, a)
		 RETURNING `+feedSelectCols,
		sessData.username, feedKindQA, topics, questions, answers)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить элементы ленты"})
		return
	}
	defer rows.Close()

	out := make([]FeedItem, 0, len(req.Items))
	for rows.Next() {
		if it, err := feedScan(rows); err == nil {
			out = append(out, it)
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

// handleGenerateFeedItems генерирует черновики элементов ленты через DeepSeek.
// Ничего не сохраняет — возвращает список, который пользователь чистит и
// сохраняет отдельно (как черновики задач цели).
// Тело: {"topic": "...", "description": "...", "count": 5}
func handleGenerateFeedItems(c *gin.Context) {
	var req struct {
		Topic       string `json:"topic"`
		Description string `json:"description"`
		Count       int    `json:"count"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	req.Topic = strings.TrimSpace(req.Topic)
	if req.Topic == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Сначала укажите тему"})
		return
	}
	if req.Count <= 0 {
		req.Count = feedDefaultGenerateCount
	}
	if req.Count > maxFeedGenerateCount {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("За раз можно создать не больше %d элементов", maxFeedGenerateCount),
		})
		return
	}

	apiKey := deepseekAPIKey()
	if apiKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Ключ DeepSeek не настроен (DEEPSEEK_API_KEY в .env)",
		})
		return
	}

	drafts, truncated, err := aiGenerateFeedItems(req.Topic, req.Description, req.Count, apiKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": drafts, "truncated": truncated})
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
		"{\"items\": [{\"topic\": string, \"question\": string, \"answer\": string}]}. " +
		"Правила: topic — раздел (область) этого элемента, ОДНО короткое слово в нижнем регистре " +
		"(например: golang, linux, ооп, sql, git); " +
		"question — конкретный вопрос по теме (до 140 символов); " +
		"answer — точный, самодостаточный ответ на 1–3 предложения (до 400 символов); " +
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
		out = append(out, feedDraft{Topic: topic, Question: question, Answer: answer})
	}
	return out, nil
}

// handleFeedItemReaction фиксирует реакцию на элемент ленты: «знаю» (know)
// или «не знаю» (unknown) — соответствующий счётчик +1.
// Тело: {"value": "know" | "unknown"}
func handleFeedItemReaction(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID элемента"})
		return
	}
	var req struct {
		Value string `json:"value"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}

	// Две готовые ветки вместо подстановки имени колонки: без динамического SQL.
	increment := `UPDATE feed_items SET unknown_count = unknown_count + 1
		WHERE id = $1 AND username = $2
		RETURNING ` + feedSelectCols
	switch strings.TrimSpace(req.Value) {
	case "know":
		increment = `UPDATE feed_items SET know_count = know_count + 1
		WHERE id = $1 AND username = $2
		RETURNING ` + feedSelectCols
	case "unknown":
		// остаётся ветка по умолчанию выше
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректная реакция"})
		return
	}

	sessData, _ := c.MustGet("session").(session)
	it, err := feedScan(db.QueryRow(context.Background(), increment, id, sessData.username))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Элемент ленты не найден"})
		return
	}
	c.JSON(http.StatusOK, it)
}

// handleFeedItemView отмечает показ элемента в ленте: views = views + 1.
// Вызывается, когда элемент показан в ленте; текст при этом не меняется.
func handleFeedItemView(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID элемента"})
		return
	}
	sessData, _ := c.MustGet("session").(session)
	it, err := feedScan(db.QueryRow(context.Background(),
		`UPDATE feed_items SET views = views + 1
		 WHERE id = $1 AND username = $2
		 RETURNING `+feedSelectCols,
		id, sessData.username))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Элемент ленты не найден"})
		return
	}
	c.JSON(http.StatusOK, it)
}
