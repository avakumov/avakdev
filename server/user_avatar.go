package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// validAvatarPresets — допустимые готовые варианты аватара (id).
// Список синхронизирован с frontend/src/lib/avatars.js.
var validAvatarPresets = map[string]bool{
	"fox":    true,
	"panda":  true,
	"tiger":  true,
	"frog":   true,
	"cat":    true,
	"owl":    true,
	"robot":  true,
	"rocket": true,
	"star":   true,
	"sun":    true,
}

// maxAvatarBytes — максимальный размер фото аватара в base64 (~300 КБ).
const maxAvatarBytes = 300 << 10

// handleUpdateAvatar сохраняет аватар текущего пользователя (PUT /api/me/avatar).
// Тело JSON: { preset?, photo_data?, photo_mime? }.
//   - preset — выбранный готовый вариант (тогда фото очищается);
//   - photo_data + photo_mime — своё фото (тогда preset очищается);
//   - если ни preset, ни photo нет — аватар сбрасывается (инициалы).
func handleUpdateAvatar(c *gin.Context) {
	sessVal, ok := c.Get("session")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Требуется вход"})
		return
	}
	sessData, _ := sessVal.(session)

	var req struct {
		Preset    string `json:"preset"`
		PhotoData string `json:"photo_data"`
		PhotoMime string `json:"photo_mime"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный запрос"})
		return
	}

	req.Preset = strings.TrimSpace(req.Preset)
	req.PhotoData = strings.TrimSpace(req.PhotoData)
	req.PhotoMime = strings.TrimSpace(req.PhotoMime)

	if req.Preset != "" && !validAvatarPresets[req.Preset] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Неизвестный вариант аватара"})
		return
	}
	if len(req.Preset) > 32 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Значение слишком длинное"})
		return
	}
	if len(req.PhotoData) > maxAvatarBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Фото слишком большое"})
		return
	}
	if req.PhotoData == "" {
		req.PhotoMime = ""
	} else if req.PhotoMime == "" {
		req.PhotoMime = "image/jpeg"
	}

	preset := req.Preset
	photo := req.PhotoData
	mime := req.PhotoMime
	if preset != "" {
		// Готовый вариант — фото не нужно.
		photo = ""
		mime = ""
	}

	if _, err := db.Exec(context.Background(),
		`UPDATE users
		 SET avatar_preset = $1, avatar_data = $2, avatar_mime = $3
		 WHERE username = $4`,
		preset, photo, mime, sessData.username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить аватар"})
		return
	}

	u, found := loadUser(sessData.username)
	if !found {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Пользователь не найден"})
		return
	}
	c.JSON(http.StatusOK, userPayload(u))
}
