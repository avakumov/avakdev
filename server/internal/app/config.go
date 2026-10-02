package app

import "os"

// DeepSeekAPIKey возвращает ключ DeepSeek из окружения (пусто — не настроен).
func DeepSeekAPIKey() string {
	return os.Getenv("DEEPSEEK_API_KEY")
}
