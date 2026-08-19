package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
)

// Конфигурация Yandex SpeechKit для синтеза речи (TTS).
//
// Поддерживаются два способа аутентификации:
//   - YANDEX_IAM_TOKEN — IAM-токен, передаётся в заголовке Authorization: Bearer <token>
//   - YANDEX_API_KEY   — статический ключ, передаётся в заголовке Authorization: Api-Key <key>
//
// Также требуется YANDEX_FOLDER_ID — каталог Yandex Cloud, где разрешён синтез.
// Формат аудио настраивается через YANDEX_TTS_FORMAT (по умолчанию oggopus;
// многие браузеры лучше открывают mp3, но не все поддерживают oggopus).

// ttsFolderID возвращает ID каталога Yandex Cloud.
func ttsFolderID() string {
	return os.Getenv("YANDEX_FOLDER_ID")
}

// ttsIAMToken возвращает IAM-токен (если задан).
func ttsIAMToken() string {
	return os.Getenv("YANDEX_IAM_TOKEN")
}

// ttsAPIKey возвращает статический API-ключ (если задан).
func ttsAPIKey() string {
	return os.Getenv("YANDEX_API_KEY")
}

// ttsFormat возвращает желаемый формат аудио (oggopus по умолчанию).
func ttsFormat() string {
	if f := os.Getenv("YANDEX_TTS_FORMAT"); f != "" {
		return f
	}
	return "oggopus"
}

// ttsMIME возвращает MIME-тип для заданного формата аудио.
func ttsMIME(format string) string {
	switch strings.ToLower(format) {
	case "mp3":
		return "audio/mpeg"
	case "lpcm":
		return "audio/l16"
	case "oggopus", "ogg":
		return "audio/ogg"
	default:
		return "application/octet-stream"
	}
}

// yandexTTS обращается к Yandex SpeechKit v1 REST API и возвращает аудио
// и его MIME-тип для переданного текста.
//
// URL: POST https://tts.api.cloud.yandex.net/speech/v1/tts:synthesize
// Тело: application/x-www-form-urlencoded (text, lang, voice, format, speed, emotion)
// Аутентификация: Authorization: Bearer <IAM token> или Authorization: Api-Key <API key>.
func yandexTTS(text string) ([]byte, string, error) {
	folderID := ttsFolderID()
	iamToken := ttsIAMToken()
	apiKey := ttsAPIKey()

	if folderID == "" {
		return nil, "", fmt.Errorf("не настроен YANDEX_FOLDER_ID (каталог Yandex Cloud)")
	}
	if iamToken == "" && apiKey == "" {
		return nil, "", fmt.Errorf("не настроены YANDEX_IAM_TOKEN или YANDEX_API_KEY для Yandex SpeechKit")
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return nil, "", fmt.Errorf("текст для озвучки пуст")
	}

	format := ttsFormat()

	// Параметры отправляем как form-urlencoded (так требует v1 REST API).
	form := url.Values{}
	form.Set("text", text)
	form.Set("lang", "ru-RU")
	form.Set("format", format)
	form.Set("voice", "oksana")
	if gv := os.Getenv("YANDEX_TTS_VOICE"); gv != "" {
		form.Set("voice", gv)
	}
	if gs := os.Getenv("YANDEX_TTS_SPEED"); gs != "" {
		form.Set("speed", gs)
	}

	body := strings.NewReader(form.Encode())

	req, err := http.NewRequest(http.MethodPost,
		"https://tts.api.cloud.yandex.net/speech/v1/tts:synthesize", body)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("x-folder-id", folderID)
	if iamToken != "" {
		req.Header.Set("Authorization", "Bearer "+iamToken)
	} else {
		req.Header.Set("Authorization", "Api-Key "+apiKey)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("ошибка вызова Yandex SpeechKit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, "", fmt.Errorf("Yandex SpeechKit вернул статус %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	if len(audio) == 0 {
		return nil, "", fmt.Errorf("Yandex SpeechKit вернул пустое аудио")
	}

	return audio, ttsMIME(format), nil
}

// handleSynthesizeNote генерирует аудио для конспекта по его полному тексту
// и сохраняет его в БД. Если аудио уже есть — возвращает его без повторного
// обращения к Yandex SpeechKit (экономия токенов/квоты).
func handleSynthesizeNote(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID"})
		return
	}

	n, ok := notes.get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "Конспект не найден"})
		return
	}
	if strings.TrimSpace(n.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Конспект пуст — нечего озвучивать"})
		return
	}

	// Текст для озвучки: убираем markdown-разметку (заголовки, жирный,
	// списки, ссылки, код), чтобы TTS читал чистый текст.
	text := markdownToPlainText(n.Content)
	if strings.TrimSpace(text) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Конспект пуст — нечего озвучивать"})
		return
	}

	// Если уже есть сохранённое аудио — отдаём его, не тратя квоту.
	if notes.hasAudio(id) {
		data, mime := notes.getAudio(id)
		c.Data(http.StatusOK, mime, data)
		return
	}

	// Yandex SpeechKit ограничивает длину одного запроса (примерно 5000 символов).
	// Разбиваем длинные конспекты на чанки, синтезируем каждый отдельно и
	// склеиваем в одно аудио (OGG — конкатенация кадров допустима).
	audio, mime, err := synthesizeChunks(text)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	if err := notes.saveAudio(id, audio, mime); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить аудио"})
		return
	}

	c.Data(http.StatusOK, mime, audio)
}

// handleGetNoteAudio возвращает ранее сгенерированное аудио конспекта.
// Если аудио ещё нет, возвращает 404 — фронтенд может вызвать
// POST /api/knowledge/:id/tts для генерации.
func handleGetNoteAudio(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный ID"})
		return
	}

	if !notes.hasAudio(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Аудио ещё не сгенерировано"})
		return
	}

	data, mime := notes.getAudio(id)
	c.Data(http.StatusOK, mime, data)
}

// markdownToPlainText убирает из текста Markdown-разметку и возвращает
// «чистый» текст, пригодный для озвучки через Yandex SpeechKit.
//
// Удаляются: маркеры заголовков (#), жирное/курсивное выделение (** * _ ~),
// списки (- * 1.), ссылки [текст](url), код (` и блоки ``` ``````),
// разделители (---), таблицы и прочие служебные символы разметки.
func markdownToPlainText(s string) string {
	lines := strings.Split(s, "\n")
	var out []string

	for _, line := range lines {
		t := line

		// Убираем HTML-теги и комментарии.
		t = removeHTMLTags(t)

		// Убираем блоки кода (тройные кавычки) и inline-код (одиночные тики).
		t = strings.ReplaceAll(t, "```", " ")
		t = strings.ReplaceAll(t, "``", " ")
		t = strings.ReplaceAll(t, "`", "")

		// Заголовки: "# ", "## ", "### ", "#### ".
		t = regexp.MustCompile(`^#{1,6}\s+`).ReplaceAllString(t, "")

		// Маркеры списков: "- ", "* ", "+ ", "1. ", "2. " и т.п. в начале строки.
		t = regexp.MustCompile(`^\s*[-*+]\s+`).ReplaceAllString(t, "")
		t = regexp.MustCompile(`^\s*\d+\.\s+`).ReplaceAllString(t, "")

		// Ссылки: [текст](url) → текст.
		t = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`).ReplaceAllString(t, "$1")

		// Жирный/курсив/зачёркнутый: **жирный**, *курсив*, __жирный__, _курсив_, ~~зачёркнутый~~.
		t = strings.ReplaceAll(t, "**", "")
		t = strings.ReplaceAll(t, "__", "")
		t = strings.ReplaceAll(t, "~~", "")
		t = strings.ReplaceAll(t, "*", "")
		t = strings.ReplaceAll(t, "_", "")

		// Разделители горизонтальной черты (---, ***, ___).
		if regexp.MustCompile(`^\s*([-*_]\s*){3,}\s*$`).MatchString(t) {
			continue // пропускаем строку-разделитель целиком
		}

		// Таблицы Markdown: пропускаем строки-разделители (|---|---|), а
		// содержимое ячеек превращаем в текст с разделителями "; ".
		if strings.Contains(t, "|") {
			// Строка-разделитель таблицы (|---+---|): пропускаем целиком.
			if regexp.MustCompile(`^\s*\|?[\s\-\|:]+$`).MatchString(t) &&
				strings.Count(t, "-") >= 3 {
				continue
			}
			t = regexp.MustCompile(`^\s*\|?\s*`).ReplaceAllString(t, "")
			t = regexp.MustCompile(`\s*\|\s*$`).ReplaceAllString(t, "")
			t = strings.ReplaceAll(t, "|", "; ")
		}

		// Quote-цитаты: "> text" → "text".
		t = regexp.MustCompile(`^\s*>\s?`).ReplaceAllString(t, "")

		// Чистим множественные пробелы и пустые строки.
		t = regexp.MustCompile(`[ \t]+`).ReplaceAllString(t, " ")
		t = strings.TrimSpace(t)

		// Пустые строки оставляем для пауз между абзацами, но не более двух подряд.
		if t == "" {
			if len(out) > 0 && out[len(out)-1] != "" {
				out = append(out, "")
			}
			continue
		}

		out = append(out, t)
	}

	text := strings.Join(out, "\n")
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")
	return strings.TrimSpace(text)
}

// removeHTMLTags удаляет HTML/XML-теги и HTML-комментарии из строки.
func removeHTMLTags(s string) string {
	// HTML-комментарии <!-- ... -->
	s = regexp.MustCompile(`<!--.*?-->`).ReplaceAllString(s, "")
	// Открывающие/закрывающие теги <tag ...>
	s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, " ")
	return s
}

// maxTTSTextLen — максимальная длина текста, которую SpeechKit принимает
// за один запрос (примерно 5000 байт). Используем консервативное значение.
const maxTTSTextLen = 4800

// synthesizeChunks синтезирует текст через Yandex SpeechKit, разбивая его
// на фрагменты не длиннее maxTTSTextLen символов (по границам абзацев
// или предложений). Все чанки склеиваются в одно аудио.
func synthesizeChunks(text string) ([]byte, string, error) {
	chunks := splitTextForTTS(text, maxTTSTextLen)
	if len(chunks) == 0 {
		return nil, "", fmt.Errorf("текст для озвучки пуст")
	}

	var combined []byte
	mime := ""
	for i, chunk := range chunks {
		audio, m, err := yandexTTS(chunk)
		if err != nil {
			return nil, "", fmt.Errorf("чанк %d/%d: %w", i+1, len(chunks), err)
		}
		if mime == "" {
			mime = m
		}
		combined = append(combined, audio...)
	}
	return combined, mime, nil
}

// splitTextForTTS разбивает текст на чанки не длиннее maxLen символов,
// стараясь резать по границам абзацев (двойной перенос строки) или
// предложений (точка, восклицательный/вопросительный знак).
func splitTextForTTS(text string, maxLen int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	// Если помещается целиком — один чанк.
	if len([]rune(text)) <= maxLen {
		return []string{text}
	}

	// Делим на абзацы.
	paragraphs := regexp.MustCompile(`\n\s*\n`).Split(text, -1)

	var chunks []string
	var current strings.Builder

	for _, para := range paragraphs {
		runes := []rune(strings.TrimSpace(para))
		if len(runes) == 0 {
			continue
		}

		// Если текущий чанк непуст и не влезет с новым абзацем — закрываем.
		if current.Len() > 0 && len([]rune(current.String()))+1+len(runes) > maxLen {
			chunks = append(chunks, strings.TrimSpace(current.String()))
			current.Reset()
		}

		// Если один абзац больше maxLen — режем его по предложениям.
		if len(runes) > maxLen {
			// Сначала сбрасываем текущий чанк (он недостаточно заполнен? нет, уже закрыт выше).
			sentences := splitBySentences(string(runes))
			for _, s := range sentences {
				s = strings.TrimSpace(s)
				if s == "" {
					continue
				}
				sr := []rune(s)
				if current.Len() > 0 && len([]rune(current.String()))+1+len(sr) > maxLen {
					chunks = append(chunks, strings.TrimSpace(current.String()))
					current.Reset()
				}
				if len(sr) > maxLen {
					// Экстремально длинное предложение — режем по maxLen,
					// стараясь не разрывать слова (резать по последнему пробелу).
					rest := sr
					for len(rest) > maxLen {
						cut := maxLen
						// Ищем последний пробел перед границей.
						lastSpace := -1
						for j := maxLen - 1; j >= 0; j-- {
							if rest[j] == ' ' || rest[j] == '\t' ||
								rest[j] == '\n' || rest[j] == '\r' {
								lastSpace = j
								break
							}
						}
						// Если нашли пробел (не слишком близко к началу) — режем по нему.
						if lastSpace > maxLen/3 {
							cut = lastSpace
						}
						chunks = append(chunks, strings.TrimSpace(string(rest[:cut])))
						rest = rest[cut:]
						rest = []rune(strings.TrimLeft(string(rest), " \t\r\n"))
					}
					if len(rest) > 0 {
						if current.Len() > 0 {
							current.WriteString(" ")
						}
						current.WriteString(string(rest))
					}
				} else {
					if current.Len() > 0 {
						current.WriteString(" ")
					}
					current.WriteString(s)
				}
			}
			if current.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(current.String()))
				current.Reset()
			}
			continue
		}

		// Обычный абзац помещается — добавляем к текущему чанку.
		if current.Len() > 0 {
			current.WriteString(" ")
		}
		current.WriteString(string(runes))
	}

	if current.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(current.String()))
	}

	return chunks
}

// splitBySentences разбивает текст на предложения по знакам препинания
// (. ! ? …) с сохранением знака в конце предложения.
//
// Текст разбивается по границам: конец предложения — символ . ! ? … ,
// за которым следует пробел (или конец строки) и затем не-цифра
// (чтобы «1.5» или «3.14» не считались концом предложения).
func splitBySentences(text string) []string {
	runes := []rune(text)
	var sentences []string
	var current []rune

	flush := func() {
		s := strings.TrimSpace(string(current))
		current = current[:0]
		if s != "" {
			sentences = append(sentences, s)
		}
	}

	i := 0
	for i < len(runes) {
		r := runes[i]
		current = append(current, r)

		if r == '.' || r == '!' || r == '?' || r == '…' {
			// Смотрим, что идёт после знака препинания.
			// Если конец текста — финализируем.
			if i+1 >= len(runes) {
				flush()
				break
			}
			// Пропускаем идущие подряд знаки препинания (!!, ?!, ...).
			j := i + 1
			for j < len(runes) && (runes[j] == '.' || runes[j] == '!' || runes[j] == '?' || runes[j] == '…') {
				current = append(current, runes[j])
				j++
			}
			i = j - 1

			if j >= len(runes) {
				flush()
				break
			}
			// Если следующий символ — пробел/перенос, а за ним не цифра —
			// это конец предложения.
			if runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\n' || runes[j] == '\r' {
				k := j
				for k < len(runes) && (runes[k] == ' ' || runes[k] == '\t' || runes[k] == '\n' || runes[k] == '\r') {
					k++
				}
				if k >= len(runes) || !unicode.IsDigit(runes[k]) {
					i = k - 1 // пропускаем все пробелы — начнём следующее предложение с k
					flush()
				}
			}
		}
		i++
	}

	if len(current) > 0 {
		s := strings.TrimSpace(string(current))
		if s != "" {
			sentences = append(sentences, s)
		}
	}

	// Если не нашли ни одной границы — возвращаем исходный текст целиком,
	// чтобы не потерять контент.
	if len(sentences) == 0 && strings.TrimSpace(text) != "" {
		return []string{strings.TrimSpace(text)}
	}

	return sentences
}
