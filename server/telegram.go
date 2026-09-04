package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Telegram-интеграция: отправка уведомлений и привязка пользователя.
// Требует TELEGRAM_BOT_TOKEN в .env. Без токена всё отключено (заглушка).

// tgToken возвращает токен бота из окружения/.env (пусто — не настроено).
func tgToken() string {
	return getenvOrEnvFile("TELEGRAM_BOT_TOKEN", "")
}

// tgAPICall выполняет запрос к Bot API и возвращает тело ответа.
func tgAPICall(method string, form url.Values) ([]byte, error) {
	return tgAPICallTimeout(method, form, 15*time.Second)
}

// tgAPICallTimeout — то же, но с явным таймаутом клиента.
func tgAPICallTimeout(method string, form url.Values, timeout time.Duration) ([]byte, error) {
	token := tgToken()
	if token == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN не настроен")
	}
	req, err := http.NewRequest(http.MethodPost,
		"https://api.telegram.org/bot"+token+"/"+method,
		bytes.NewBufferString(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram %s: %w", method, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("telegram %s: статус %d: %s", method, resp.StatusCode, string(body))
	}
	return body, nil
}

// tgSendMessage отправляет текстовое сообщение в чат.
func tgSendMessage(chatID, text string) error {
	_, err := tgAPICall("sendMessage", url.Values{
		"chat_id": {chatID},
		"text":    {text},
	})
	return err
}

var (
	tgBotNameOnce sync.Once
	tgBotName     string
	tgBotNameErr  error
)

// tgBotUsername возвращает @username бота (через getMe, кэшируется).
func tgBotUsername() (string, error) {
	tgBotNameOnce.Do(func() {
		body, err := tgAPICall("getMe", nil)
		if err != nil {
			tgBotNameErr = err
			return
		}
		var parsed struct {
			OK     bool `json:"ok"`
			Result struct {
				Username string `json:"username"`
			} `json:"result"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil || !parsed.OK {
			tgBotNameErr = fmt.Errorf("getMe: не удалось получить имя бота")
			return
		}
		tgBotName = parsed.Result.Username
	})
	return tgBotName, tgBotNameErr
}

// tgLinkCodeTTL — срок жизни кода привязки.
const tgLinkCodeTTL = 30 * time.Minute

// handleLinkTelegram создаёт одноразовый код привязки и возвращает ссылку
// вида https://t.me/<bot>?start=<code> (POST /api/me/telegram/link).
func handleLinkTelegram(c *gin.Context) {
	if tgToken() == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Telegram-бот не настроен (нет TELEGRAM_BOT_TOKEN)"})
		return
	}
	sessData, _ := c.MustGet("session").(session)

	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось создать код привязки"})
		return
	}
	code := hex.EncodeToString(buf)

	if _, err := db.Exec(context.Background(),
		`UPDATE users SET telegram_link_code = $1, telegram_link_at = now()
		 WHERE username = $2`,
		code, sessData.username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось сохранить код привязки"})
		return
	}

	botName, err := tgBotUsername()
	if err != nil || botName == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось получить имя бота"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"url": fmt.Sprintf("https://t.me/%s?start=%s", botName, code),
	})
}

// handleUnlinkTelegram отвязывает Telegram от пользователя
// (POST /api/me/telegram/unlink).
func handleUnlinkTelegram(c *gin.Context) {
	sessData, _ := c.MustGet("session").(session)
	if _, err := db.Exec(context.Background(),
		`UPDATE users
		 SET telegram_chat_id = '', telegram_link_code = '', telegram_link_at = NULL
		 WHERE username = $1`,
		sessData.username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось отключить Telegram"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// startTelegramLinkWatcher запускает long polling getUpdates: обрабатывает
// /start <code> и привязывает chat_id к пользователю. Работает, только если
// задан TELEGRAM_BOT_TOKEN и есть БД.
func startTelegramLinkWatcher() {
	if db == nil || tgToken() == "" {
		return
	}
	go func() {
		var offset int64
		// Long polling держит соединение ~25с (параметр timeout) + запас.
		pollClientTimeout := 70 * time.Second
		for {
			time.Sleep(2 * time.Second)
			body, err := tgAPICallTimeout("getUpdates", url.Values{
				"timeout": {"25"},
				"offset":  {fmt.Sprintf("%d", offset)},
			}, pollClientTimeout)
			if err != nil {
				if !strings.Contains(err.Error(), "TELEGRAM_BOT_TOKEN") {
					log.Printf("TELEGRAM: getUpdates: %v", err)
				}
				continue
			}
			var parsed struct {
				OK     bool `json:"ok"`
				Result []struct {
					UpdateID int64 `json:"update_id"`
					Message  *struct {
						Chat struct {
							ID int64 `json:"id"`
						} `json:"chat"`
						Text string `json:"text"`
					} `json:"message"`
				} `json:"result"`
			}
			if err := json.Unmarshal(body, &parsed); err != nil || !parsed.OK {
				continue
			}
			for _, upd := range parsed.Result {
				if upd.UpdateID >= offset {
					offset = upd.UpdateID + 1
				}
				msg := upd.Message
				if msg == nil {
					continue
				}
				// Ожидаем "/start <code>".
				fields := strings.Fields(msg.Text)
				if len(fields) != 2 || fields[0] != "/start" {
					continue
				}
				code := fields[1]
				var username string
				err := db.QueryRow(context.Background(),
					`SELECT username FROM users
					 WHERE telegram_link_code = $1
					   AND telegram_link_at > now() - interval '30 minutes'`,
					code).Scan(&username)
				if err != nil {
					continue // код не найден/просрочен
				}
				chatID := fmt.Sprintf("%d", msg.Chat.ID)
				if _, err := db.Exec(context.Background(),
					`UPDATE users
					 SET telegram_chat_id = $1, telegram_link_code = '', telegram_link_at = NULL
					 WHERE username = $2`,
					chatID, username); err != nil {
					log.Printf("TELEGRAM: не удалось привязать %s: %v", username, err)
					continue
				}
				log.Printf("TELEGRAM: пользователь %s подключил Telegram (chat %s)", username, chatID)
				tgSendMessage(chatID, "✅ Telegram подключён — сюда будут приходить уведомления.")
			}
		}
	}()
}
