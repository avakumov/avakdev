package handlers

import (
	"context"
	"net/http"
	"strings"
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

// maxAvatarBytes — максимальный размер фото аватара в base64 (~512 КБ).
// С запасом покрывает 512×512 JPEG (клиент сжимает фото до 512 px по длинной
// стороне), включая «тяжёлые» кадры.
const maxAvatarBytes = 512 << 10

// UpdateAvatar сохраняет аватар текущего пользователя (PUT /api/me/avatar).
// Тело JSON: { preset?, photo_data?, photo_mime? }.
//   - preset — выбранный готовый вариант (тогда фото очищается);
//   - photo_data + photo_mime — своё фото (тогда preset очищается);
//   - если ни preset, ни photo нет — аватар сбрасывается (инициалы).
func (h *Handlers) UpdateAvatar(w http.ResponseWriter, r *http.Request) {
	sessData, ok := sessionOf(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "Требуется вход"})
		return
	}

	var req struct {
		Preset    string `json:"preset"`
		PhotoData string `json:"photo_data"`
		PhotoMime string `json:"photo_mime"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Некорректный запрос"})
		return
	}

	req.Preset = strings.TrimSpace(req.Preset)
	req.PhotoData = strings.TrimSpace(req.PhotoData)
	req.PhotoMime = strings.TrimSpace(req.PhotoMime)

	// Защита от старых клиентов, славаших photo_data с data URI префиксом:
	// храним только «голый» base64.
	if strings.HasPrefix(req.PhotoData, "data:") {
		if i := strings.Index(req.PhotoData, ","); i >= 0 {
			req.PhotoData = req.PhotoData[i+1:]
		}
	}
	req.PhotoData = strings.TrimSpace(req.PhotoData)

	if req.Preset != "" && !validAvatarPresets[req.Preset] {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Неизвестный вариант аватара"})
		return
	}
	if len(req.Preset) > 32 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Значение слишком длинное"})
		return
	}
	if len(req.PhotoData) > maxAvatarBytes {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Фото слишком большое"})
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

	if err := h.App.Users.SetAvatar(context.Background(), sessData.Username, preset, photo, mime); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось сохранить аватар"})
		return
	}

	u, found := h.userByUsername(sessData.Username)
	if !found {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Пользователь не найден"})
		return
	}
	writeJSON(w, http.StatusOK, userPayload(u))
}
