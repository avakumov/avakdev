// Пакет httpget — задание «HTTP-запрос».
//
// Реализуйте GetUser так, чтобы прошли тесты (см. httpget_test.go).
// Тесты поднимают сервер локально через net/http/httptest, поэтому доступ
// в интернет не требуется — всё идёт через loopback.
package httpget

import (
	"errors"
	"net/http"
	"time"
)

// User — данные, которые возвращает сервер.
type User struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ErrBadStatus возвращается, когда сервер ответил статусом, отличным от 200.
// Обернуть его можно через %w, чтобы работал errors.Is.
var ErrBadStatus = errors.New("httpget: неожиданный статус ответа")

// Client — HTTP-клиент с таймаутом. Используйте его в GetUser.
var Client = &http.Client{Timeout: 10 * time.Second}

// GetUser делает GET-запрос по адресу url и декодирует JSON-ответ в User.
// Если сервер вернул статус, отличный от 200, верните ошибку, обёрнутую
// вокруг ErrBadStatus.
func GetUser(url string) (User, error) {
	// TODO: выполните запрос через Client, проверьте статус и декодируйте JSON.
	return User{}, nil
}
