package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"regexp"

	"github.com/gin-gonic/gin"
)

// imgWithID — regex, находящий тег <img> с атрибутом id="resume-photo".
// Используется для подстановки актуального фото в сгенерированный HTML резюме.
var imgWithID = regexp.MustCompile(`<img[^>]*id="resume-photo"[^>]*>`)

// photoImgClass — regex, находящий любой тег <img> с классом "photo".
// Служит запасным вариантом: если DeepSeek не сохранил id="resume-photo",
// а сгенерировал собственный тег фото, мы всё равно подменим его.
var photoImgClass = regexp.MustCompile(`<img[^>]*class="[^"]*\bphoto\b[^"]*"[^>]*>`)

// Profile — пользовательский профиль: описание для генерации резюме
// и сгенерированное резюме.
type Profile struct {
	// Description — короткое описание работника/соискателя,
	// на основе которого генерируется резюме.
	Description string `json:"description"`
	// Resume — сгенерированное DeepSeek резюме (в HTML/текстовом виде).
	// Содержит плейсхолдер <img id="resume-photo">, в который на сервере
	// подставляется актуальное фото.
	Resume string `json:"resume"`
	// PhotoMime — MIME-тип прикреплённого фото (например image/jpeg).
	PhotoMime string `json:"photo_mime"`
	// PhotoData — прикреплённое фото в base64 (без data URI префикса).
	PhotoData string `json:"photo_data"`
	// Updated — время последнего изменения профиля (RFC3339, UTC).
	Updated string `json:"updated"`
}

// profileStore — хранилище профиля.
// Если база данных PostgreSQL настроена, профиль хранится в таблице profile
// (единственная строка id=1). Иначе используется in-memory структура.
type profileStore struct {
	mu    sync.Mutex
	data  Profile
	hasDB bool
}

// profiles — глобальное хранилище профиля.
var profiles *profileStore

// newProfileStore создаёт новое хранилище профиля.
func newProfileStore() *profileStore {
	return &profileStore{data: Profile{}, hasDB: db != nil}
}

// initProfiles инициализирует глобальное хранилище профиля.
// При наличии БД подгружает уже сохранённые данные.
// (Таблица и строка id=1 создаются миграциями goose, см. migrations/.)
func initProfiles() error {
	profiles = newProfileStore()
	if !profiles.hasDB {
		return nil
	}

	var description, resume, photo, photoMime, updated string
	err := db.QueryRow(context.Background(),
		`SELECT description,
		        resume,
		        photo,
		        photo_mime,
		        to_char(updated AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM profile WHERE id = 1`).
		Scan(&description, &resume, &photo, &photoMime, &updated)
	if err != nil && err.Error() != "no rows in result set" {
		return err
	}
	profiles.data = Profile{
		Description: description,
		Resume:      resume,
		PhotoMime:   photoMime,
		PhotoData:   photo,
		Updated:     updated,
	}
	return nil
}

// get возвращает текущий профиль.
func (ps *profileStore) get() Profile {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.data
}

// saveDescription сохраняет только описание (резюме не трогаем).
func (ps *profileStore) saveDescription(description string) (Profile, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	p := Profile{
		Description: description,
		Resume:      ps.data.Resume,
		PhotoMime:   ps.data.PhotoMime,
		PhotoData:   ps.data.PhotoData,
		Updated:     now,
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.hasDB {
		if _, err := db.Exec(context.Background(),
			`UPDATE profile SET description = $1, updated = now() WHERE id = 1`,
			description); err != nil {
			return Profile{}, err
		}
	}
	ps.data = p
	return p, nil
}

// saveResume сохраняет сгенерированное резюме (описание не трогаем).
func (ps *profileStore) saveResume(resume string) (Profile, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	p := Profile{
		Description: ps.data.Description,
		Resume:      resume,
		PhotoMime:   ps.data.PhotoMime,
		PhotoData:   ps.data.PhotoData,
		Updated:     now,
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.hasDB {
		if _, err := db.Exec(context.Background(),
			`UPDATE profile SET resume = $1, updated = now() WHERE id = 1`,
			resume); err != nil {
			return Profile{}, err
		}
	}
	ps.data = p
	return p, nil
}

// savePhoto сохраняет фото (base64 + MIME) в профиль.
func (ps *profileStore) savePhoto(data, mime string) (Profile, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	p := Profile{
		Description: ps.data.Description,
		Resume:      ps.data.Resume,
		PhotoMime:   mime,
		PhotoData:   data,
		Updated:     now,
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.hasDB {
		if _, err := db.Exec(context.Background(),
			`UPDATE profile SET photo = $1, photo_mime = $2, updated = now() WHERE id = 1`,
			data, mime); err != nil {
			return Profile{}, err
		}
	}
	ps.data = p
	return p, nil
}

// clearPhoto удаляет фото из профиля.
func (ps *profileStore) clearPhoto() (Profile, error) {
	return ps.savePhoto("", "")
}

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

// deepseekAPIKey возвращает ключ DeepSeek из окружения/.env.
func deepseekAPIKey() string {
	return getenvOrEnvFile("DEEPSEEK_API_KEY", "")
}

// generateResume вызывает DeepSeek chat API для создания резюме
// на основе описания пользователя.
func generateResume(description string) (string, error) {
	apiKey := deepseekAPIKey()
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

// handleGetProfile возвращает текущий профиль.
func handleGetProfile(c *gin.Context) {
	c.JSON(http.StatusOK, profiles.get())
}

// handleSaveProfile сохраняет описание профиля.
func handleSaveProfile(c *gin.Context) {
	var req struct {
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	p, err := profiles.saveDescription(req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить профиль"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// handleGenerateResume генерирует резюме на основе описания профиля.
func handleGenerateResume(c *gin.Context) {
	p := profiles.get()
	resume, err := generateResume(p.Description)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updated, err := profiles.saveResume(resume)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить резюме"})
		return
	}
	c.JSON(http.StatusOK, updated)
}

// handleSaveResume сохраняет вручную отредактированный текст резюме.
func handleSaveResume(c *gin.Context) {
	var req struct {
		Resume string `json:"resume"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}
	p, err := profiles.saveResume(req.Resume)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить резюме"})
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

// handleUploadPhoto загружает фото профиля из multipart-формы (поле "photo").
func handleUploadPhoto(c *gin.Context) {
	file, header, err := c.Request.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Не удалось прочитать файл"})
		return
	}
	defer file.Close()

	if header.Size > maxPhotoBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Фото слишком большое (макс. 5 МБ)"})
		return
	}
	// Читаем файл в буфер для определения MIME.
	buf := make([]byte, header.Size)
	if _, err := io.ReadFull(file, buf); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Не удалось прочитать файл"})
		return
	}
	mime := http.DetectContentType(buf)
	if !allowedPhotoTypes[mime] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Формат фото не поддерживается (JPEG/PNG/WebP/GIF)"})
		return
	}
	data := base64.StdEncoding.EncodeToString(buf)
	p, err := profiles.savePhoto(data, mime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить фото"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// handleDeletePhoto удаляет фото профиля.
func handleDeletePhoto(c *gin.Context) {
	p, err := profiles.clearPhoto()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось удалить фото"})
		return
	}
	c.JSON(http.StatusOK, p)
}

// handleResumePage отдаёт отдельную HTML-страницу с резюме.
// Страница содержит только резюме (HTML+CSS), её можно открыть в браузере
// и распечатать/сохранить в PDF. В HTML подставляется актуальное фото.
func handleResumePage(c *gin.Context) {
	p := profiles.get()
	if strings.TrimSpace(p.Resume) == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Резюме ещё не сгенерировано"})
		return
	}
	html := applyPhotoToResume(p.Resume, p.PhotoData, p.PhotoMime)
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}
