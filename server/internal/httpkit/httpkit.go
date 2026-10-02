// Package httpkit — тонкая обёртка над go-chi/chi (и, значит, над net/http).
//
// Роутинг, группы, middleware и логи/восстановление после паник даёт chi;
// Context — небольшой хелпер для JSON-ответов и разбора запроса, чтобы
// обработчики оставались компактными.
package httpkit

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// H — удобный алиас для JSON-объектов.
type H = map[string]any

// HandlerFunc — обработчик запроса.
type HandlerFunc = func(*Context)

// Middleware — стандартный middleware chi: func(http.Handler) http.Handler.
type Middleware = func(http.Handler) http.Handler

// Context — небольшой хелпер над http.ResponseWriter/http.Request: JSON-ответы,
// параметры маршрута chi, разбор JSON/multipart, cookie и значения контекста.
type Context struct {
	Request *http.Request
	Writer  http.ResponseWriter
	keys    map[string]any
}

// NewContext создаёт Context для запроса (используется и в middleware).
func NewContext(w http.ResponseWriter, r *http.Request) *Context {
	return &Context{Request: r, Writer: w}
}

// ---- Ответы ----

// JSON пишет объект в ответ в формате JSON с указанным статусом.
func (c *Context) JSON(status int, obj any) { WriteJSON(c.Writer, status, obj) }

// WriteJSON — JSON-ответ без Context (для middleware).
func WriteJSON(w http.ResponseWriter, status int, obj any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(obj)
}

// Data пишет сырые байты с указанным Content-Type.
func (c *Context) Data(status int, contentType string, data []byte) {
	c.Writer.Header().Set("Content-Type", contentType)
	c.Writer.WriteHeader(status)
	_, _ = c.Writer.Write(data)
}

// String — текстовый ответ.
func (c *Context) String(status int, s string) {
	c.Data(status, "text/plain; charset=utf-8", []byte(s))
}

// Text — текстовый ответ (аналог gin c.Text).
func (c *Context) Text(status int, s string) {
	c.Data(status, "text/plain; charset=utf-8", []byte(s))
}

// Header выставляет заголовок ответа.
func (c *Context) Header(key, value string) { c.Writer.Header().Set(key, value) }

// ---- Запрос ----

// Param возвращает параметр маршрута (chi: {name}).
func (c *Context) Param(name string) string { return chi.URLParam(c.Request, name) }

// Query возвращает значение query-параметра.
func (c *Context) Query(name string) string { return c.Request.URL.Query().Get(name) }

// ShouldBindJSON разбирает тело запроса как JSON.
func (c *Context) ShouldBindJSON(obj any) error {
	defer c.Request.Body.Close()
	return json.NewDecoder(c.Request.Body).Decode(obj)
}

// FormFile возвращает файл из multipart-формы.
func (c *Context) FormFile(name string) (*multipart.FileHeader, error) {
	if err := c.Request.ParseMultipartForm(64 << 20); err != nil {
		return nil, err
	}
	_, fh, err := c.Request.FormFile(name)
	return fh, err
}

// Cookie читает cookie.
func (c *Context) Cookie(name string) (string, error) {
	ck, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	return ck.Value, nil
}

// SetCookie устанавливает cookie.
func (c *Context) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		Secure:   secure,
		HttpOnly: httpOnly,
	})
}

// ---- Значения контекста ----

type ctxKey string

// SetRequestValue кладёт значение в контекст запроса (для chi-middleware).
func SetRequestValue(r *http.Request, key string, val any) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), ctxKey(key), val))
}

// Set кладёт значение в контекст текущего обработчика.
func (c *Context) Set(key string, value any) {
	if c.keys == nil {
		c.keys = map[string]any{}
	}
	c.keys[key] = value
}

// Get читает значение: сначала локальные, затем переданные middleware.
func (c *Context) Get(key string) (any, bool) {
	if v, ok := c.keys[key]; ok {
		return v, true
	}
	if v := c.Request.Context().Value(ctxKey(key)); v != nil {
		return v, true
	}
	return nil, false
}

// MustGet читает значение, паникуя при отсутствии.
func (c *Context) MustGet(key string) any {
	if v, ok := c.Get(key); ok {
		return v
	}
	panic(fmt.Sprintf("httpkit: значение %q не задано", key))
}

// ---- Роутер на chi ----

// Engine — обёртка над chi.Router.
type Engine struct {
	router chi.Router
}

// New создаёт роутер без встроенных middleware.
func New() *Engine { return &Engine{router: chi.NewRouter()} }

// Default — роутер со встроенными middleware: логгером запросов и
// восстановлением после паник.
func Default() *Engine {
	e := New()
	e.router.Use(middleware.Logger)
	e.router.Use(middleware.Recoverer)
	return e
}

// Use добавляет глобальный middleware.
func (e *Engine) Use(m ...Middleware) { e.router.Use(m...) }

// Group создаёт группу маршрутов с префиксом.
func (e *Engine) Group(prefix string) *Group {
	return &Group{router: e.router, prefix: prefix}
}

// NoRoute задаёт обработчик для несовпавших маршрутов (SPA-fallback и т.п.).
func (e *Engine) NoRoute(h HandlerFunc) { e.router.NotFound(adapt(h)) }

// Static отдаёт каталог по префиксу.
func (e *Engine) Static(prefix, dir string) {
	e.router.Handle(prefix+"/*", http.StripPrefix(prefix, http.FileServer(http.Dir(dir))))
}

// StaticFS отдаёт файловую систему по префиксу.
func (e *Engine) StaticFS(prefix string, fs http.FileSystem) {
	e.router.Handle(prefix+"/*", http.StripPrefix(prefix, http.FileServer(fs)))
}

// Run запускает HTTP-сервер.
func (e *Engine) Run(addr string) error { return http.ListenAndServe(addr, e.router) }

// ServeHTTP — реализация http.Handler.
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) { e.router.ServeHTTP(w, r) }

func (e *Engine) GET(p string, h HandlerFunc)    { e.router.Get(toChiPattern(p), adapt(h)) }
func (e *Engine) POST(p string, h HandlerFunc)   { e.router.Post(toChiPattern(p), adapt(h)) }
func (e *Engine) PUT(p string, h HandlerFunc)    { e.router.Put(toChiPattern(p), adapt(h)) }
func (e *Engine) DELETE(p string, h HandlerFunc) { e.router.Delete(toChiPattern(p), adapt(h)) }
func (e *Engine) PATCH(p string, h HandlerFunc)  { e.router.Patch(toChiPattern(p), adapt(h)) }

// Group — группа маршрутов с префиксом и собственным middleware.
type Group struct {
	router     chi.Router
	prefix     string
	middleware []Middleware
}

// Use добавляет middleware в группу.
func (g *Group) Use(m ...Middleware) { g.middleware = append(g.middleware, m...) }

// Group создаёт вложенную группу (префикс и middleware наследуются).
func (g *Group) Group(prefix string) *Group {
	return &Group{
		router:     g.router,
		prefix:     g.prefix + prefix,
		middleware: append([]Middleware{}, g.middleware...),
	}
}

func (g *Group) GET(p string, h HandlerFunc) {
	g.router.With(g.middleware...).Get(toChiPattern(g.prefix+p), adapt(h))
}
func (g *Group) POST(p string, h HandlerFunc) {
	g.router.With(g.middleware...).Post(toChiPattern(g.prefix+p), adapt(h))
}
func (g *Group) PUT(p string, h HandlerFunc) {
	g.router.With(g.middleware...).Put(toChiPattern(g.prefix+p), adapt(h))
}
func (g *Group) DELETE(p string, h HandlerFunc) {
	g.router.With(g.middleware...).Delete(toChiPattern(g.prefix+p), adapt(h))
}
func (g *Group) PATCH(p string, h HandlerFunc) {
	g.router.With(g.middleware...).Patch(toChiPattern(g.prefix+p), adapt(h))
}

// adapt превращает HandlerFunc в стандартный http.HandlerFunc.
func adapt(h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h(NewContext(w, r))
	}
}

// toChiPattern переводит gin-шаблон ":name" в chi-шаблон "{name}".
func toChiPattern(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		if strings.HasPrefix(p, ":") {
			parts[i] = "{" + p[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}
