package httpget

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"avakumov/exercises/internal/exercise"
)

func TestMain(m *testing.M) { exercise.Main(m) }

func TestGetUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("метод = %s, ожидался GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(User{ID: 7, Name: "Ада"})
	}))
	defer srv.Close()

	got, err := GetUser(srv.URL)
	if err != nil {
		t.Fatalf("GetUser: неожиданная ошибка: %v", err)
	}
	if got.ID != 7 || got.Name != "Ада" {
		t.Errorf("GetUser = %+v, ожидалось {7 Ада}", got)
	}
}

func TestGetUserBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "нет такого пользователя", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := GetUser(srv.URL)
	if err == nil {
		t.Fatal("ожидалась ошибка при статусе 404")
	}
	if !errors.Is(err, ErrBadStatus) {
		t.Errorf("ошибка = %v, ожидалась ErrBadStatus", err)
	}
}

func TestGetUserBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{"))
	}))
	defer srv.Close()

	if _, err := GetUser(srv.URL); err == nil {
		t.Fatal("ожидалась ошибка при некорректном JSON")
	}
}
