package httpkit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Проверяем: параметры маршрута (chi), значения из middleware (SetRequestValue)
// и прерывание цепочки middleware (запись ответа без вызова next).
func TestRouterBasics(t *testing.T) {
	r := New()

	api := r.Group("/api")
	api.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, SetRequestValue(req, "user", "vasya"))
		})
	})
	api.GET("/items/:id", func(c *Context) {
		c.JSON(http.StatusOK, H{"id": c.Param("id"), "user": c.MustGet("user")})
	})

	priv := r.Group("/api/private")
	priv.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			WriteJSON(w, http.StatusForbidden, H{"error": "no"})
		})
	})
	priv.GET("/x", func(c *Context) {
		c.JSON(http.StatusOK, H{"x": 1})
	})

	srv := httptest.NewServer(r)
	defer srv.Close()

	// 1) Параметры + значение из middleware.
	resp, err := http.Get(srv.URL + "/api/items/42")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	got := string(body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("статус %d, ожидался 200; тело: %q", resp.StatusCode, got)
	}
	if !strings.Contains(got, `"id":"42"`) || !strings.Contains(got, `"user":"vasya"`) {
		t.Fatalf("неожиданный ответ: %q", got)
	}

	// 2) Middleware прервал цепочку — обработчик не выполнился.
	resp2, err := http.Get(srv.URL + "/api/private/x")
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("middleware: статус %d, ожидался 403", resp2.StatusCode)
	}
}
