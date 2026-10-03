package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"avakumov/server/internal/app"
)

// LinkTelegram создаёт одноразовый код привязки и возвращает ссылку
// вида https://t.me/<bot>?start=<code> (POST /api/me/telegram/link).
func (h *Handlers) LinkTelegram(w http.ResponseWriter, r *http.Request) {
	if app.TelegramToken() == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "Telegram-бот не настроен (нет TELEGRAM_BOT_TOKEN)"})
		return
	}
	sessData, _ := sessionOf(r)

	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось создать код привязки"})
		return
	}
	code := hex.EncodeToString(buf)

	if err := h.App.Users.SetLinkCode(context.Background(), sessData.Username, code); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось сохранить код привязки"})
		return
	}

	botName, err := app.TelegramBotUsername()
	if err != nil || botName == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось получить имя бота"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url": fmt.Sprintf("https://t.me/%s?start=%s", botName, code),
	})
}

// UnlinkTelegram отвязывает Telegram от пользователя
// (POST /api/me/telegram/unlink).
func (h *Handlers) UnlinkTelegram(w http.ResponseWriter, r *http.Request) {
	sessData, _ := sessionOf(r)
	if err := h.App.Users.UnlinkTelegram(context.Background(), sessData.Username); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "Не удалось отключить Telegram"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
