package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Telegram-интеграция: отправка уведомлений и привязка пользователя.
// Требует TELEGRAM_BOT_TOKEN в окружении. Без токена всё отключено (заглушка).

// TelegramToken возвращает токен бота (пусто — не настроено).
func TelegramToken() string {
	return os.Getenv("TELEGRAM_BOT_TOKEN")
}

// TgAPICall выполняет запрос к Bot API и возвращает тело ответа.
func TgAPICall(method string, form url.Values) ([]byte, error) {
	return TgAPICallTimeout(method, form, 15*time.Second)
}

// TgAPICallTimeout — то же, но с явным таймаутом клиента.
func TgAPICallTimeout(method string, form url.Values, timeout time.Duration) ([]byte, error) {
	token := TelegramToken()
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

// TelegramSendMessage отправляет текстовое сообщение в чат.
func TelegramSendMessage(chatID, text string) error {
	_, err := TgAPICall("sendMessage", url.Values{
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

// TelegramBotUsername возвращает @username бота (через getMe, кэшируется).
func TelegramBotUsername() (string, error) {
	tgBotNameOnce.Do(func() {
		body, err := TgAPICall("getMe", nil)
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

// StartTelegramLinkWatcher запускает long polling getUpdates: обрабатывает
// /start <code> и привязывает chat_id к пользователю. Работает, только если
// задан TELEGRAM_BOT_TOKEN и есть БД.
func (a *App) StartTelegramLinkWatcher() {
	if a.DB == nil || TelegramToken() == "" {
		return
	}
	go func() {
		var offset int64
		// Long polling держит соединение ~25с (параметр timeout) + запас.
		pollClientTimeout := 70 * time.Second
		for {
			time.Sleep(2 * time.Second)
			body, err := TgAPICallTimeout("getUpdates", url.Values{
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
				username, ok := a.Users.UsernameByLinkCode(context.Background(), code)
				if !ok {
					continue // код не найден/просрочен
				}
				chatID := fmt.Sprintf("%d", msg.Chat.ID)
				if err := a.Users.LinkTelegramChat(context.Background(), username, chatID); err != nil {
					log.Printf("TELEGRAM: не удалось привязать %s: %v", username, err)
					continue
				}
				log.Printf("TELEGRAM: пользователь %s подключил Telegram (chat %s)", username, chatID)
				TelegramSendMessage(chatID, "✅ Telegram подключён — сюда будут приходить уведомления.")
			}
		}
	}()
}
