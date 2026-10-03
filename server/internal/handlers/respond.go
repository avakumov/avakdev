package handlers

import (
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"

	"github.com/go-chi/chi/v5"

	"avakumov/server/internal/app"
)

// ---- Сессия в контексте запроса ----

type ctxKey string

const sessionKey ctxKey = "session"

// withSession кладёт сессию в контекст запроса (используется middleware).
func withSession(r *http.Request, s app.Session) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), sessionKey, s))
}

// sessionOf извлекает сессию, положенную middleware AuthRequired.
func sessionOf(r *http.Request) (app.Session, bool) {
	s, ok := r.Context().Value(sessionKey).(app.Session)
	return s, ok
}

// ---- Ответы ----

// writeJSON пишет объект в ответ в формате JSON. HTML в строках не экранируется
// (SetEscapeHTML(false)) — в ответах есть Markdown/HTML (резюме, конспекты).
func writeJSON(w http.ResponseWriter, status int, obj any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(obj)
}

// writeData пишет сырые байты с указанным Content-Type.
func writeData(w http.ResponseWriter, status int, contentType string, data []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

// ---- Запрос ----

// param возвращает параметр маршрута chi ({name}).
func param(r *http.Request, name string) string {
	return chi.URLParam(r, name)
}

// decodeJSON разбирает тело запроса как JSON.
func decodeJSON(r *http.Request, obj any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(obj)
}

// formFile возвращает файл из multipart-формы (буфер до 64 МБ).
func formFile(r *http.Request, name string) (*multipart.FileHeader, error) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		return nil, err
	}
	_, fh, err := r.FormFile(name)
	return fh, err
}

// cookieValue читает значение cookie.
func cookieValue(r *http.Request, name string) (string, error) {
	ck, err := r.Cookie(name)
	if err != nil {
		return "", err
	}
	return ck.Value, nil
}
