package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"

	"avakumov/server/internal/app"
	"avakumov/server/internal/httpkit"
)

// LinkTelegram создаёт одноразовый код привязки и возвращает ссылку
// вида https://t.me/<bot>?start=<code> (POST /api/me/telegram/link).
func (h *Handlers) LinkTelegram(c *httpkit.Context) {
	if app.TelegramToken() == "" {
		c.JSON(http.StatusBadRequest, httpkit.H{"error": "Telegram-бот не настроен (нет TELEGRAM_BOT_TOKEN)"})
		return
	}
	sessData, _ := c.MustGet("session").(app.Session)

	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось создать код привязки"})
		return
	}
	code := hex.EncodeToString(buf)

	if err := h.App.Users.SetLinkCode(context.Background(), sessData.Username, code); err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось сохранить код привязки"})
		return
	}

	botName, err := app.TelegramBotUsername()
	if err != nil || botName == "" {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось получить имя бота"})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{
		"url": fmt.Sprintf("https://t.me/%s?start=%s", botName, code),
	})
}

// UnlinkTelegram отвязывает Telegram от пользователя
// (POST /api/me/telegram/unlink).
func (h *Handlers) UnlinkTelegram(c *httpkit.Context) {
	sessData, _ := c.MustGet("session").(app.Session)
	if err := h.App.Users.UnlinkTelegram(context.Background(), sessData.Username); err != nil {
		c.JSON(http.StatusInternalServerError, httpkit.H{"error": "Не удалось отключить Telegram"})
		return
	}
	c.JSON(http.StatusOK, httpkit.H{"ok": true})
}
