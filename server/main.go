package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"avakumov/server/internal/agent"
	"avakumov/server/internal/app"
	"avakumov/server/internal/database"
	"avakumov/server/internal/env"
	"avakumov/server/internal/handlers"
	"avakumov/server/internal/notifier"
)

// shutdownTimeout — сколько ждём завершения текущих запросов при остановке.
// Меньше TimeoutStopSec в systemd-юните (deploy.sh), чтобы успеть закрыться
// до принудительного SIGKILL.
const shutdownTimeout = 25 * time.Second

// subFrontendDist отдаёт подкаталог frontend-dist из встроенной статики.
func subFrontendDist() fs.FS {
	sub, err := fs.Sub(frontendDist, "frontend-dist")
	if err != nil {
		log.Fatalf("не удалось открыть встроенный фронтенд: %v", err)
	}
	return sub
}

func indexData(fsys fs.FS) ([]byte, bool) {
	b, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return nil, false
	}
	return b, true
}

// writeJSON пишет JSON-ответ. HTML в строках не экранируется (как в хендлерах).
func writeJSON(w http.ResponseWriter, status int, obj any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(obj)
}

func main() {
	// Подхватываем переменные из .env (ключ DeepSeek, DATABASE_URL и т.п.).
	env.Load()

	// Режим «только агент»: отдельный процесс, который air не перезапускает.
	// Агент сам редактирует файлы (правит код, создаёт миграции) — если он
	// живёт внутри сервера, air при каждом изменении файла убивает его посреди
	// задачи. Поэтому агент запускается своим процессом (make dev / make dev-agent).
	if os.Getenv("AVAKUMOV_AGENT") == "1" {
		if agent.Start() {
			select {} // агент-процесс работает вечно, HTTP-сервер не поднимает
		}
		return
	}

	// Подключаемся к PostgreSQL (если задана DATABASE_URL).
	if err := initDB(); err != nil {
		log.Fatalf("не удалось подключиться к PostgreSQL: %v", err)
	}
	defer database.Close() // пул закроется при выходе из main
	// Приложение (домен): пул БД, сессии и хранилища store.
	application := app.New(database.Pool())
	h := handlers.New(application)
	// Применяем версионированные миграции БД (goose), встроенные в бинарник.
	if err := database.Migrate(); err != nil {
		log.Fatalf("не удалось применить миграции БД: %v", err)
	}
	if err := application.InitProfile(); err != nil {
		log.Fatalf("не удалось инициализировать профиль: %v", err)
	}
	if err := application.InitKnowledge(); err != nil {
		log.Fatalf("не удалось инициализировать конспекты: %v", err)
	}
	if err := application.InitImportant(); err != nil {
		log.Fatalf("не удалось инициализировать важное сообщение: %v", err)
	}
	if err := application.InitUserMetrics(); err != nil {
		log.Fatalf("не удалось инициализировать метрики: %v", err)
	}
	if err := application.InitAppTasks(); err != nil {
		log.Fatalf("не удалось инициализировать задачи: %v", err)
	}
	if err := application.InitTasksGoals(); err != nil {
		log.Fatalf("не удалось инициализировать задачи и цели: %v", err)
	}
	if err := application.InitNotifications(); err != nil {
		log.Fatalf("не удалось инициализировать уведомления: %v", err)
	}

	// Фоновая доставка уведомлений в Telegram и обработка привязки бота.
	notifier.Start(application)
	application.StartTelegramLinkWatcher()
	logAuthConfig()

	// Агент по задачам приложения — только в dev-режиме, отдельным процессом
	// (AVAKUMOV_AGENT=1). Встроенный режим — лишь по явному AGENT_INPROCESS=1.
	if os.Getenv("AGENT_INPROCESS") == "1" {
		agent.Start()
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(api chi.Router) {
		// Публичные маршруты (без авторизации).
		api.Post("/login", h.Login)

		// Маршруты, требующие активной сессии.
		api.Group(func(authed chi.Router) {
			authed.Use(h.AuthRequired)

			authed.Get("/me", h.Me)
			authed.Put("/me", h.UpdateMe)
			authed.Put("/me/avatar", h.UpdateAvatar)
			authed.Post("/me/telegram/link", h.LinkTelegram)
			authed.Post("/me/telegram/unlink", h.UnlinkTelegram)
			authed.Post("/logout", h.Logout)

			// Маршруты, требующие прав администратора.
			authed.Group(func(admin chi.Router) {
				admin.Use(h.AdminRequired)
				admin.Get("/health", h.Health)
				admin.Get("/message", h.Message)
				admin.Get("/metrics", h.ServerMetrics)
				admin.Get("/db-schema", h.DBSchema)

				// Задачи по модификации приложения (раздел «Приложение») —
				// только для администраторов (запрос деплоя/отката изменений).
				admin.Get("/app-tasks", h.ListAppTasks)
				admin.Post("/app-tasks", h.CreateAppTask)
				admin.Put("/app-tasks/{id}", h.UpdateAppTask)
				admin.Delete("/app-tasks/{id}", h.DeleteAppTask)
			})

			// Отчёты за дни (создание, редактирование, список).
			authed.Get("/reports", h.ListReports)
			authed.Put("/reports/{date}", h.UpsertReport)

			// Раздел «Чтение»: книги (fb2/epub → HTML).
			authed.Get("/books", h.ListBooks)
			authed.Post("/books", h.UploadBook)
			authed.Get("/books/last-bookmark", h.LastBookmark)
			authed.Get("/books/{id}", h.GetBook)
			authed.Delete("/books/{id}", h.DeleteBook)
			authed.Put("/books/{id}/finished", h.SetBookFinished)
			authed.Get("/books/{id}/bookmarks", h.ListBookmarks)
			authed.Post("/books/{id}/bookmarks", h.CreateBookmark)
			authed.Delete("/books/{id}/bookmarks/{bookmarkId}", h.DeleteBookmark)

			// Время чтения по дням и цель чтения на день.
			authed.Get("/reading/time", h.GetReadingTime)
			authed.Get("/reading/history", h.ReadingHistory)
			authed.Post("/reading/time", h.AddReadingTime)
			authed.Put("/reading/goal", h.SetReadingGoal)

			// Конспекты знаний (создание, генерация, редактирование, удаление).
			authed.Get("/knowledge", h.ListNotes)
			authed.Post("/knowledge", h.CreateNote)
			authed.Post("/knowledge/generate", h.GenerateNote)
			authed.Put("/knowledge/{id}", h.UpdateNote)
			authed.Post("/knowledge/{id}/repeat", h.RepeatNote)
			authed.Delete("/knowledge/{id}", h.DeleteNote)

			// Озвучка конспектов (Yandex SpeechKit).
			// POST — сгенерировать и сохранить аудио, GET — получить уже готовое.
			authed.Post("/knowledge/{id}/tts", h.SynthesizeNote)
			authed.Get("/knowledge/{id}/tts", h.GetNoteAudio)

			// «Важное» сообщение: у каждого пользователя своё — просмотр, сохранение
			// и отметка о прочтении доступны всем авторизованным.
			authed.Get("/important", h.GetImportant)
			authed.Put("/important", h.SaveImportant)
			authed.Post("/important/seen", h.MarkImportantSeen)

			// Раздел «Заметки»: быстрые записи-черновики.
			authed.Get("/drafts", h.ListDrafts)
			authed.Post("/drafts", h.CreateDraft)
			authed.Put("/drafts/{id}", h.UpdateDraft)
			authed.Delete("/drafts/{id}", h.DeleteDraft)

			// Раздел «Лента»: элементы ленты (пока тип контента — «вопрос-ответ»).
			// /view — счётчик показов, растёт когда элемент показан в ленте.
			// /generate — черновики от ИИ (в БД не пишутся), /bulk — сохранить пачку.
			authed.Get("/feed", h.ListFeed)
			authed.Post("/feed", h.CreateFeedItem)
			authed.Post("/feed/generate", h.GenerateFeedItems)
			authed.Post("/feed/bulk", h.BulkCreateFeedItems)
			authed.Put("/feed/{id}", h.UpdateFeedItem)
			authed.Delete("/feed/{id}", h.DeleteFeedItem)
			authed.Post("/feed/{id}/view", h.FeedItemView)
			authed.Post("/feed/{id}/reaction", h.FeedItemReaction)

			// Пользовательские метрики: определения (тип: целое/дробное/да-нет)
			// и значения — одно на (метрика, день).
			authed.Get("/user-metrics", h.ListUserMetrics)
			authed.Post("/user-metrics", h.CreateUserMetric)
			authed.Put("/user-metrics/{id}", h.UpdateUserMetric)
			authed.Delete("/user-metrics/{id}", h.DeleteUserMetric)
			authed.Put("/user-metrics/{id}/{date}", h.SetUserMetricValue)
			authed.Delete("/user-metrics/{id}/{date}", h.DeleteUserMetricValue)

			// Цели (первый раздел, главная страница).
			authed.Get("/goals", h.ListGoals)
			authed.Post("/goals", h.CreateGoal)
			authed.Post("/goals/generate-tasks", h.GenerateGoalTasks)
			authed.Put("/goals/{id}", h.UpdateGoal)
			authed.Put("/goals/{id}/tasks-order", h.ReorderGoalTasks)
			authed.Delete("/goals/{id}", h.DeleteGoal)

			// Задачи раздела «Задачи» (категории, время, дедлайн, статус).
			authed.Get("/tasks", h.ListTasks)
			authed.Post("/tasks", h.CreateTask)
			authed.Put("/tasks/{id}", h.UpdateTask)
			authed.Delete("/tasks/{id}", h.DeleteTask)

			// Раздел «День»: ежедневный план (задачи + повторение знаний, метрики).
			authed.Get("/day", h.GetDay)
			authed.Post("/day/suggest", h.DaySuggest)
			authed.Put("/day", h.SaveDay)
			authed.Put("/day/done", h.SetDayItemDone)
			authed.Put("/day/spent", h.SetDayItemSpent)
			authed.Get("/day/history", h.DayHistory)

			// Уведомления пользователя (колокольчик на странице профиля).
			authed.Get("/notifications", h.ListNotifications)
			authed.Post("/notifications", h.CreateNotification)
			authed.Delete("/notifications/{id}", h.DeleteNotification)
			// «Входящие»: наступившие по расписанию (колокольчик).
			authed.Get("/notifications/inbox", h.ListNotificationInbox)
			authed.Delete("/notifications/inbox/{id}", h.DismissNotification)

			// Профиль и генерация резюме.
			authed.Get("/profile", h.GetProfile)
			authed.Put("/profile", h.SaveProfile)
			authed.Post("/profile/generate", h.GenerateResume)
			authed.Put("/profile/resume", h.SaveResume)
			authed.Get("/profile/resume", h.ResumePage)

			// Фото для резюме.
			authed.Post("/profile/photo", h.UploadPhoto)
			authed.Delete("/profile/photo", h.DeletePhoto)
		})
	})

	// ---- Статика React ----
	// В собранном бинарнике фронтенд встроен (embed).
	// В dev-режиме, если версия без встроенной статики, читаем из ../frontend/dist.
	serveFrontend(r)

	// ---- Запуск ----
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{Addr: ":" + port, Handler: r}

	// Перехватываем SIGINT/SIGTERM: даём серверу доработать текущие запросы
	// (например, длительную ИИ-генерацию или конвертацию книги), затем
	// закрываем пул БД отложенным вызовом database.Close.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("Сервер запущен: http://localhost:%s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP-сервер: %v", err)
		}
	}()

	<-ctx.Done()
	stop() // возвращаем сигналам поведение по умолчанию на время остановки
	log.Println("Получен сигнал завершения — останавливаем сервер…")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Корректная остановка не удалась: %v", err)
		_ = srv.Close()
	}
	log.Println("Сервер остановлен")
}

// serveFrontend отдаёт React-статику: сначала из встроенного
// бинарника (release), при его отсутствии — из фронтовой папки (dev).
func serveFrontend(r chi.Router) {
	// 1) Встроенный фронтенд (собран через deploy-скрипт)
	if fsys := subFrontendDist(); hasIndex(fsys) {
		index, ok := indexData(fsys)
		if ok {
			// Статику ассетов отдаём из подпапки assets: FileServer после
			// StripPrefix("/assets") ищет файлы прямо в корне переданной FS,
			// поэтому передаём именно подкаталог assets.
			if assetsFS, err := fs.Sub(fsys, "assets"); err == nil {
				r.Handle("/assets/*", http.StripPrefix("/assets", http.FileServer(http.FS(assetsFS))))
			}

			r.NotFound(func(w http.ResponseWriter, req *http.Request) {
				// http.FileServer редиректит прямые запросы /index.html
				// на ./ (301), поэтому отдаём HTML напрямую.
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(index)
			})
			log.Printf("Frontend отдаётся из встроенного бинарника")
		}
		return
	}

	// 2) Фолбэк: файловая система (для локальной разработки)
	staticDir := filepath.Join("..", "frontend", "dist")
	if _, err := os.Stat(staticDir); err == nil {
		r.Handle("/assets/*", http.StripPrefix("/assets",
			http.FileServer(http.Dir(filepath.Join(staticDir, "assets")))))
		r.NotFound(func(w http.ResponseWriter, req *http.Request) {
			index, err := os.ReadFile(filepath.Join(staticDir, "index.html"))
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "no index.html"})
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(index)
		})
		log.Printf("Frontend подключён из %s", staticDir)
		return
	}

	// 3) Совсем нет фронтенда
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"error": "not found",
			"hint":  "соберите фронтенд: cd frontend && npm run build",
		})
	})
}

// hasIndex проверяет, есть ли index.html в переданной FS.
func hasIndex(fsys fs.FS) bool {
	_, err := fs.Stat(fsys, "index.html")
	return err == nil
}

// initDB подключается к PostgreSQL по строке подключения из DATABASE_URL.
// БД не настроена (пустая переменная) — приложение работает без неё.
func initDB() error {
	return database.Init(context.Background(), os.Getenv("DATABASE_URL"))
}

// logAuthConfig печатает состояние подключения к БД при старте.
func logAuthConfig() {
	if database.Pool() == nil {
		log.Println("AUTH: база данных не настроена (DATABASE_URL пуст). Авторизация отключена.")
		return
	}
	log.Println("AUTH: подключение к PostgreSQL установлено. Авторизация включена.")
}
