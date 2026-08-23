package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Пути инструментов не должны выходить за пределы репозитория.
func TestAgentResolvePath(t *testing.T) {
	root := t.TempDir()
	a := &agent{repoRoot: root}

	if p, err := a.resolvePath(""); err != nil || p != root {
		t.Fatalf("resolvePath('') = %q, %v; want %q", p, err, root)
	}
	if p, err := a.resolvePath("server/main.go"); err != nil {
		t.Fatalf("resolvePath(server/main.go): %v", err)
	} else if !strings.HasPrefix(p, root+"/server") {
		t.Fatalf("resolvePath вернул путь вне корня: %q", p)
	}
	if _, err := a.resolvePath("../secret"); err == nil {
		t.Fatal("resolvePath должен запрещать выход за пределы репозитория")
	}
	if _, err := a.resolvePath("../../etc/passwd"); err == nil {
		t.Fatal("resolvePath должен запрещать глубокий выход за пределы репозитория")
	}
}

// HTTP-взаимодействие с сервером задач: вход -> cookie -> список -> обновление.
func TestAgentServerFlow(t *testing.T) {
	var sessionCookie string

	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Username != "agent" || req.Password != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		sessionCookie = "sess123"
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: sessionCookie, Path: "/api"})
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"username":"agent"}`))
	})

	mux.HandleFunc("/api/app-tasks", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(cookieName); err != nil || c.Value != sessionCookie {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"tasks":[{"id":1,"title":"Исправить отступ","description":"","status":"new"}]}`))
	})

	mux.HandleFunc("/api/app-tasks/", func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(cookieName); err != nil || c.Value != sessionCookie {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req struct {
			Status          string  `json:"status"`
			Result          *string `json:"result"`
			Log             *string `json:"log"`
			DeployRequested *bool   `json:"deploy_requested"`
			DeployedAt      *string `json:"deployed_at"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.Status != taskStatusInProgress && req.Status != taskStatusDone {
			t.Errorf("PUT со статусом %q, want in_progress или done", req.Status)
		}
		if req.Status == taskStatusDone && (req.Result == nil || *req.Result == "") && req.DeployedAt == nil {
			t.Errorf("PUT done без результата и без времени деплоя")
		}
		if req.Status == taskStatusDone && (req.Log == nil || *req.Log == "") {
			t.Errorf("PUT done без журнала выполнения")
		}
		if req.DeployedAt != nil && (req.DeployRequested == nil || *req.DeployRequested) {
			t.Errorf("после деплоя deploy_requested должен быть false")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	a := &agent{
		serverURL: srv.URL,
		username:  "agent",
		password:  "secret",
		client:    srv.Client(),
	}

	if err := a.ensureSession(); err != nil {
		t.Fatalf("ensureSession: %v", err)
	}
	if a.sessionTok == "" {
		t.Fatal("ensureSession не сохранил cookie сессии")
	}

	tasks, err := a.fetchTasks()
	if err != nil {
		t.Fatalf("fetchTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Title != "Исправить отступ" {
		t.Fatalf("tasks = %+v", tasks)
	}

	if err := a.updateTask(tasks[0].ID, tasks[0], taskStatusInProgress, nil, nil, nil, nil); err != nil {
		t.Fatalf("updateTask(in_progress): %v", err)
	}
	res := "изменён отступ"
	taskLog := "[1] read_file(...)\nвывод сборки"
	if err := a.updateTask(tasks[0].ID, tasks[0], taskStatusDone, &res, &taskLog, nil, nil); err != nil {
		t.Fatalf("updateTask(done): %v", err)
	}
	// Завершение деплоя: снимаем флаг и проставляем время.
	deployedAt := "2026-08-23T12:00:00Z"
	if err := a.updateTask(tasks[0].ID, tasks[0], taskStatusDone, nil, &taskLog, boolPtr(false), &deployedAt); err != nil {
		t.Fatalf("updateTask(deploy): %v", err)
	}
}

// Неверные учётные данные не дают сессию.
func TestAgentLoginRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	a := &agent{
		serverURL: srv.URL,
		username:  "agent",
		password:  "wrong",
		client:    srv.Client(),
	}
	if err := a.ensureSession(); err == nil {
		t.Fatal("ensureSession с неверным паролем должен падать")
	}
}

// updateTask должен переживать 401 (сессия сгорела после рестарта прода из-за
// деплоя): перелогиниться и повторить запрос, чтобы флаг деплоя снялся.
func TestAgentUpdateTaskRetriesAfter401(t *testing.T) {
	loginCount := 0
	putCount := 0

	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		loginCount++
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "sess-live", Path: "/api"})
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/app-tasks/1", func(w http.ResponseWriter, r *http.Request) {
		putCount++
		if putCount == 1 {
			// Первый запрос идёт со старой сессией, сгоревшей после рестарта.
			if c, err := r.Cookie(cookieName); err != nil || c.Value != "sess-dead" {
				t.Errorf("первый PUT должен идти со старой сессией")
			}
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// Повторный запрос — уже со свежей сессией.
		if c, err := r.Cookie(cookieName); err != nil || c.Value != "sess-live" {
			t.Errorf("повторный PUT должен идти со свежей сессией")
		}
		w.WriteHeader(http.StatusOK)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	a := &agent{
		serverURL:  srv.URL,
		username:   "agent",
		password:   "secret",
		client:     srv.Client(),
		sessionTok: "sess-dead",
	}

	task := AppTask{ID: 1, Title: "Задача"}
	if err := a.updateTask(task.ID, task, taskStatusDone, nil, nil, nil, nil); err != nil {
		t.Fatalf("updateTask: %v", err)
	}
	if putCount != 2 {
		t.Fatalf("PUT выполнен %d раз, want 2 (первый 401 + повтор)", putCount)
	}
	if loginCount != 1 {
		t.Fatalf("логин выполнен %d раз, want 1", loginCount)
	}
}
