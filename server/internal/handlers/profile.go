package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"avakumov/server/internal/app"
	"avakumov/server/internal/httpkit"
)

// imgWithID — regex, находящий тег <img> с атрибутом id="resume-photo".
// Используется для подстановки актуального фото в сгенерированный HTML резюме.
var imgWithID = regexp.MustCompile(`<img[^>]*id="resume-photo"[^>]*>`)

// photoImgClass — regex, находящий любой тег <img> с классом "photo".
// Служит запасным вариантом: если DeepSeek не сохранил id="resume-photo",
// а сгенерировал собственный тег фото, мы всё равно подменим его.
var photoImgClass = regexp.MustCompile(`<img[^>]*class="[^"]*\bphoto\b[^"]*"[^>]*>`)

// applyPhotoToResume подставляет актуальное фото (data URI) в HTML резюме.
// Сначала ищет наш плейсхолдер <img id="resume-photo">; если его нет
// (DeepSeek мог заменить тег), ищет любой <img class="photo">.
// Если фото не задано — убирает фото-тег из документа.
func applyPhotoToResume(html, photoData, photoMime string) string {
	if html == "" {
		return html
	}
	if strings.TrimSpace(photoData) == "" {
		if imgWithID.MatchString(html) {
			return imgWithID.ReplaceAllString(html, "")
		}
		return photoImgClass.ReplaceAllString(html, "")
	}
	dataURI := "data:" + photoMime + ";base64," + photoData
	replacement := `<img id="resume-photo" class="photo" alt="Фото" src="` + dataURI + `">`
	if imgWithID.MatchString(html) {
		return imgWithID.ReplaceAllString(html, replacement)
	}
	return photoImgClass.ReplaceAllString(html, replacement)
}

// generateResume вызывает DeepSeek chat API для создания резюме
// на основе описания пользователя.
func generateResume(description string) (string, error) {
	apiKey := app.DeepSeekAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("ключ DeepSeek не настроен (DEEPSEEK_API_KEY в .env)")
	}
	if strings.TrimSpace(description) == "" {
		return "", fmt.Errorf("сначала заполните описание профиля")
	}

	systemPrompt := "Ты — профессиональный карьерный консультант. " +
		"Составь резюме на русском языке по описанию соискателя, используя только факты из описания, не выдумывай. " +
		"Сгенерируй ПОЛНЫЙ HTML-документ страницы резюме, который откроется в браузере.\n\n" +
		"Вот наш CSS-шаблон оформления — используй его без изменений (подставь контент в разметку):\n" +
		resumeTemplateHead() + "\n" +
		"А вот пример структуры тела документа — заполни разделы реальными данными из описания, " +
		"при необходимости добавляя/удаляя строки списков и карточки опыта, но сохраняя классы и структуру шаблона:\n" +
		resumeTemplateBodyAsExample() + "\n" +
		resumeTemplateFoot() + "\n\n" +
		"Верни только один полный HTML-документ, начинающийся с <!DOCTYPE html>, без текстовых пояснений вне HTML.\n"

	payload := map[string]any{
		"model": "deepseek-chat",
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": description},
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
	return sanitizeHTMLAnswer(parsed.Choices[0].Message.Content), nil
}

// sanitizeHTMLAnswer подчищает ответ DeepSeek: убирает возможные markdown-блоки
// (```html ... ``` / ``` ... ```) и возвращает только содержимое HTML-документа.
func sanitizeHTMLAnswer(s string) string {
	s = strings.TrimSpace(s)
	// Если модель обернула ответ в fenced code block — срезаем обрамление.
	lines := strings.Split(s, "\n")
	if len(lines) >= 2 {
		first := strings.TrimSpace(lines[0])
		last := strings.TrimSpace(lines[len(lines)-1])
		if strings.HasPrefix(first, "```") {
			s = strings.TrimSpace(strings.Join(lines[1:len(lines)-1], "\n"))
		} else if strings.HasPrefix(first, "```") {
			s = strings.TrimSpace(strings.Join(lines[1:], "\n"))
		}
		if strings.HasPrefix(last, "```") && strings.HasSuffix(s, "```") {
			s = strings.TrimSpace(strings.TrimSuffix(s, "```"))
		}
	}
	// Оставляем только содержимое до закрывающего </html>, если оно есть.
	if idx := strings.Index(s, "</html>"); idx >= 0 {
		s = s[:idx+len("</html>")]
	}
	return strings.TrimSpace(s)
}

// GetProfile возвращает текущий профиль.
func (h *Handlers) GetProfile(c *httpkit.Context) {
	c.JSON(http.StatusOK, h.App.Profile.Get())
}

// SaveProfile сохраняет описание профиля.
func (h *Handlers) SaveProfile(c *httpkit.Context) {
	var req struct {
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	p, err := h.App.Profile.SaveDescription(req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось сохранить профиль"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// GenerateResume генерирует резюме на основе описания профиля.
func (h *Handlers) GenerateResume(c *httpkit.Context) {
	p := h.App.Profile.Get()
	resume, err := generateResume(p.Description)
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": err.Error()})
		return
	}
	updated, err := h.App.Profile.SaveResume(resume)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось сохранить резюме"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

// SaveResume сохраняет вручную отредактированный текст резюме.
func (h *Handlers) SaveResume(c *httpkit.Context) {
	var req struct {
		Resume string `json:"resume"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Некорректный запрос"})
		return
	}
	p, err := h.App.Profile.SaveResume(req.Resume)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось сохранить резюме"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// maxPhotoBytes — максимальный размер загружаемого фото (5 МБ).
const maxPhotoBytes = 5 << 20

// allowedPhotoTypes — допустимые MIME-типы фото.
var allowedPhotoTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

// UploadPhoto загружает фото профиля из multipart-формы (поле "photo").
func (h *Handlers) UploadPhoto(c *httpkit.Context) {
	file, header, err := c.Request.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Не удалось прочитать файл"})
		return
	}
	defer file.Close()

	if header.Size > maxPhotoBytes {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Фото слишком большое (макс. 5 МБ)"})
		return
	}
	// Читаем файл в буфер для определения MIME.
	buf := make([]byte, header.Size)
	if _, err := io.ReadFull(file, buf); err != nil {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Не удалось прочитать файл"})
		return
	}
	mime := http.DetectContentType(buf)
	if !allowedPhotoTypes[mime] {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Формат фото не поддерживается (JPEG/PNG/WebP/GIF)"})
		return
	}
	data := base64.StdEncoding.EncodeToString(buf)
	p, err := h.App.Profile.SavePhoto(data, mime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось сохранить фото"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// DeletePhoto удаляет фото профиля.
func (h *Handlers) DeletePhoto(c *httpkit.Context) {
	p, err := h.App.Profile.ClearPhoto()
	if err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось удалить фото"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// ResumePage отдаёт отдельную HTML-страницу с резюме.
// Страница содержит только резюме (HTML+CSS), её можно открыть в браузере
// и распечатать/сохранить в PDF. В HTML подставляется актуальное фото.
func (h *Handlers) ResumePage(c *httpkit.Context) {
	p := h.App.Profile.Get()
	if strings.TrimSpace(p.Resume) == "" {
		c.JSON(http.StatusNotFound, httpkit.H{"error": "Резюме ещё не сгенерировано"})
		return
	}
	html := applyPhotoToResume(p.Resume, p.PhotoData, p.PhotoMime)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// resumeTemplateCSS содержит CSS-шаблон резюме в одну человекочитаемую строку.
// Он передаётся DeepSeek в системном сообщении как образец оформления,
// чтобы сгенерированное резюме точно соответствовало нашему дизайну.
//
// Это формат А4, ориентированный на печать: страница сама подстраивается,
// а при печати в PDF браузер корректно разобьёт документ на страницы.
func resumeTemplateCSS() string {
	return `<style>
  :root {
    --accent: #2563eb;
    --ink: #1f2937;
    --muted: #6b7280;
    --line: #e5e7eb;
  }
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body {
    font-family: "Segoe UI", system-ui, Roboto, Arial, sans-serif;
    color: var(--ink);
    background: #f3f4f6;
    line-height: 1.5;
  }
  .page {
    max-width: 800px;
    margin: 24px auto;
    background: #ffffff;
    padding: 40px 48px;
    box-shadow: 0 1px 3px rgba(0,0,0,.08);
    border-radius: 8px;
  }
  .header {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 24px;
    margin-bottom: 20px;
  }
  .header .info { flex: 1; min-width: 0; }
  .header .photo-wrap { flex-shrink: 0; }
  .photo {
    display: block;
    width: 140px;
    height: 170px;
    object-fit: cover;
    border-radius: 0;
    margin: 0;
    border: 1px solid #d1d5db;
  }
  h1 {
    font-size: 30px;
    font-weight: 700;
    color: var(--ink);
    margin-bottom: 2px;
  }
  .role { font-size: 16px; color: var(--accent); font-weight: 600; margin-bottom: 12px; }
  .contacts { color: var(--muted); font-size: 14px; display: flex; flex-wrap: wrap; gap: 4px 16px; }
  .section { margin-top: 20px; }
  .section h2 {
    font-size: 14px;
    text-transform: uppercase;
    letter-spacing: .08em;
    color: var(--accent);
    border-bottom: 2px solid var(--line);
    padding-bottom: 6px;
    margin-bottom: 12px;
  }
  .section p { margin-bottom: 8px; }
  ul { margin: 0 0 8px 18px; }
  li { margin-bottom: 4px; }
  .job { margin-bottom: 14px; }
  .job .head { display: flex; justify-content: space-between; flex-wrap: wrap; font-weight: 600; }
  .job .head .dates { font-weight: 400; color: var(--muted); font-style: italic; }
  .skill-tags { display: flex; flex-wrap: wrap; gap: 6px; }
  .skill-tags span {
    background: #eff6ff;
    color: var(--accent);
    border: 1px solid #bfdbfe;
    border-radius: 999px;
    padding: 2px 10px;
    font-size: 13px;
  }
  @media (max-width: 640px) {
    .page { padding: 24px 20px; margin: 8px auto; }
    .header { flex-direction: column; align-items: flex-start; gap: 16px; }
    .header .photo-wrap { order: -1; align-self: center; }
  }
  @media print {
    body { background: #fff; }
    .page { max-width: none; margin: 0; padding: 0; box-shadow: none; border-radius: 0; }
    .section { break-inside: avoid; }
  }
</style>`
}

// resumeTemplateHead возвращает открывающую часть HTML-документа резюме:
// DOCTYPE, head с charset/viewport и встроенным CSS-шаблоном.
func resumeTemplateHead() string {
	return "<!DOCTYPE html>\n<html lang=\"ru\">\n<head>\n<meta charset=\"UTF-8\">\n" +
		"<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n" +
		"<title>Резюме</title>\n" +
		resumeTemplateCSS() + "\n</head>\n<body>\n<div class=\"page\">\n"
}

// resumeTemplateFoot возвращает закрывающую часть HTML-документа резюме.
func resumeTemplateFoot() string {
	return "\n</div>\n</body>\n</html>"
}

// resumeTemplateBodyAsExample возвращает пример структуры тела резюме
// (пустые разделы с плейсхолдерами), который передаётся DeepSeek,
// чтобы модель вписала сгенерированный контент в наш шаблон.
func resumeTemplateBodyAsExample() string {
	return `<div class="header">
  <div class="info">
    <h1>ИМЯ ФАМИЛИЯ</h1>
    <div class="role">ДОЛЖНОСТЬ</div>
    <div class="contacts">
      <span>email@example.com</span>
      <span>+7 (000) 000-00-00</span>
      <span>Город</span>
      <span>github.com/username</span>
    </div>
  </div>
  <div class="photo-wrap"><img id="resume-photo" class="photo" alt="Фото"></div>
</div>

<div class="section">
  <h2>О себе</h2>
  <p>Краткое описание соискателя.</p>
</div>

<div class="section">
  <h2>Навыки</h2>
  <div class="skill-tags">
    <span>Навык 1</span>
    <span>Навык 2</span>
    <span>Навык 3</span>
  </div>
</div>

<div class="section">
  <h2>Опыт работы</h2>
  <div class="job">
    <div class="head"><span>Компания — должность</span><span class="dates">2020 — 2024</span></div>
    <ul>
      <li>Достижение или обязанность.</li>
      <li>Достижение или обязанность.</li>
    </ul>
  </div>
</div>

<div class="section">
  <h2>Образование</h2>
  <p><strong>Университет</strong> — специальность, год окончания.</p>
</div>

<div class="section">
  <h2>Достижения</h2>
  <ul>
    <li>Конкретное достижение.</li>
  </ul>
</div>`
}
