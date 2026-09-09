package main

import (
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// indexData читает index.html из переданной FS.
// Содержимое кэшируется в памяти (печально известный 301 http.FileServer
// на прямые запросы /index.html нас не трогает — отдаём файл напрямую).
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

func main() {
	// В продакшене (собранный бинарник) включаем release-режим.
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Подхватываем переменные из .env (ключ DeepSeek, DATABASE_URL и т.п.).
	loadEnv()

	// Режим «только агент»: отдельный процесс, который air не перезапускает.
	// Агент сам редактирует файлы (правит код, создаёт миграции) — если он
	// живёт внутри сервера, air при каждом изменении файла убивает его посреди
	// задачи. Поэтому агент запускается своим процессом (make dev / make dev-agent).
	if os.Getenv("AVAKUMOV_AGENT") == "1" {
		if startAgent() {
			select {} // агент-процесс работает вечно, HTTP-сервер не поднимает
		}
		return
	}

	// Подключаемся к PostgreSQL (если задана DATABASE_URL).
	if err := initDB(); err != nil {
		log.Fatalf("не удалось подключиться к PostgreSQL: %v", err)
	}
	// Применяем версионированные миграции БД (goose), встроенные в бинарник.
	if err := runMigrations(); err != nil {
		log.Fatalf("не удалось применить миграции БД: %v", err)
	}
	if err := initProfiles(); err != nil {
		log.Fatalf("не удалось инициализировать профиль: %v", err)
	}
	if err := initKnowledge(); err != nil {
		log.Fatalf("не удалось инициализировать конспекты: %v", err)
	}
	if err := initImportant(); err != nil {
		log.Fatalf("не удалось инициализировать важное сообщение: %v", err)
	}
	if err := initMetrics(); err != nil {
		log.Fatalf("не удалось инициализировать метрики: %v", err)
	}
	if err := initAppTasks(); err != nil {
		log.Fatalf("не удалось инициализировать задачи: %v", err)
	}
	if err := initTasks(); err != nil {
		log.Fatalf("не удалось инициализировать раздел «Задачи»: %v", err)
	}
	if err := initGoals(); err != nil {
		log.Fatalf("не удалось инициализировать раздел «Цели»: %v", err)
	}
	if err := initNotifications(); err != nil {
		log.Fatalf("не удалось инициализировать уведомления: %v", err)
	}

	// Фоновая доставка уведомлений в Telegram и обработка привязки бота.
	startNotificationScheduler()
	startTelegramLinkWatcher()
	logAuthConfig()

	// Агент по задачам приложения — только в dev-режиме, отдельным процессом
	// (AVAKUMOV_AGENT=1). Встроенный режим — лишь по явному AGENT_INPROCESS=1.
	if os.Getenv("AGENT_INPROCESS") == "1" {
		startAgent()
	}

	r := gin.Default()

	// ---- API ----
	api := r.Group("/api")

	// Публичные маршруты (без авторизации).
	{
		api.POST("/login", handleLogin)
	}

	// Маршруты, требующие активной сессии.
	authed := api.Group("")
	authed.Use(authRequired)
	{
		authed.GET("/me", handleMe)
		authed.PUT("/me", handleUpdateMe)
		authed.PUT("/me/avatar", handleUpdateAvatar)
		authed.POST("/me/telegram/link", handleLinkTelegram)
		authed.POST("/me/telegram/unlink", handleUnlinkTelegram)
		authed.POST("/logout", handleLogout)

		// Маршруты, требующие прав администратора.
		admin := authed.Group("")
		admin.Use(adminRequired)
		{
			admin.GET("/health", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"status": "ok",
					"time":   time.Now().Format(time.RFC3339),
				})
			})

			admin.GET("/message", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{
					"message": "Привет! Это ответ от Go (Gin) сервера 🚀",
					"server":  "gin",
				})
			})

			// Системные метрики сервера (CPU, память, диск, сеть).
			admin.GET("/metrics", func(c *gin.Context) {
				c.JSON(http.StatusOK, collectMetrics())
			})

			// Задачи по модификации приложения (раздел «Приложение») —
			// только для администраторов (запрос деплоя/отката изменений).
			admin.GET("/app-tasks", handleListAppTasks)
			admin.POST("/app-tasks", handleCreateAppTask)
			admin.PUT("/app-tasks/:id", handleUpdateAppTask)
			admin.DELETE("/app-tasks/:id", handleDeleteAppTask)
		}

		// Отчёты за дни (создание, редактирование, список).
		authed.GET("/reports", handleListReports)
		authed.PUT("/reports/:date", handleUpsertReport)

		// Конспекты знаний (создание, генерация, редактирование, удаление).
		authed.GET("/knowledge", handleListNotes)
		authed.POST("/knowledge", handleCreateNote)
		authed.POST("/knowledge/generate", handleGenerateNote)
		authed.PUT("/knowledge/:id", handleUpdateNote)
		authed.POST("/knowledge/:id/repeat", handleRepeatNote)
		authed.DELETE("/knowledge/:id", handleDeleteNote)

		// Озвучка конспектов (Yandex SpeechKit).
		// POST — сгенерировать и сохранить аудио, GET — получить уже готовое.
		authed.POST("/knowledge/:id/tts", handleSynthesizeNote)
		authed.GET("/knowledge/:id/tts", handleGetNoteAudio)

		// «Важное» сообщение: у каждого пользователя своё — просмотр, сохранение
		// и отметка о прочтении доступны всем авторизованным.
		authed.GET("/important", handleGetImportant)
		authed.PUT("/important", handleSaveImportant)
		authed.POST("/important/seen", handleMarkImportantSeen)

		// Пользовательские метрики: определения (тип: целое/дробное/да-нет)
		// и значения — одно на (метрика, день).
		authed.GET("/user-metrics", handleListUserMetrics)
		authed.POST("/user-metrics", handleCreateUserMetric)
		authed.PUT("/user-metrics/:id", handleUpdateUserMetric)
		authed.DELETE("/user-metrics/:id", handleDeleteUserMetric)
		authed.PUT("/user-metrics/:id/:date", handleSetUserMetricValue)
		authed.DELETE("/user-metrics/:id/:date", handleDeleteUserMetricValue)

		// Цели (первый раздел, главная страница).
		authed.GET("/goals", handleListGoals)
		authed.POST("/goals", handleCreateGoal)
		authed.POST("/goals/generate-tasks", handleGenerateGoalTasks)
		authed.PUT("/goals/:id", handleUpdateGoal)
		authed.PUT("/goals/:id/tasks-order", handleReorderGoalTasks)
		authed.DELETE("/goals/:id", handleDeleteGoal)

		// Задачи раздела «Задачи» (категории, время, дедлайн, статус).
		authed.GET("/tasks", handleListTasks)
		authed.POST("/tasks", handleCreateTask)
		authed.PUT("/tasks/:id", handleUpdateTask)
		authed.DELETE("/tasks/:id", handleDeleteTask)

		// Раздел «День»: ежедневный план (задачи + повторение знаний, метрики).
		authed.GET("/day", handleGetDay)
		authed.POST("/day/suggest", handleDaySuggest)
		authed.PUT("/day", handleSaveDay)
		authed.PUT("/day/done", handleSetDayItemDone)
		authed.GET("/day/history", handleDayHistory)

		// Уведомления пользователя (колокольчик на странице профиля).
		authed.GET("/notifications", handleListNotifications)
		authed.POST("/notifications", handleCreateNotification)
		authed.DELETE("/notifications/:id", handleDeleteNotification)
		// «Входящие»: наступившие по расписанию (колокольчик).
		authed.GET("/notifications/inbox", handleListNotificationInbox)
		authed.DELETE("/notifications/inbox/:id", handleDismissNotification)

		// Профиль и генерация резюме.
		authed.GET("/profile", handleGetProfile)
		authed.PUT("/profile", handleSaveProfile)
		authed.POST("/profile/generate", handleGenerateResume)
		authed.PUT("/profile/resume", handleSaveResume)
		authed.GET("/profile/resume", handleResumePage)

		// Фото для резюме.
		authed.POST("/profile/photo", handleUploadPhoto)
		authed.DELETE("/profile/photo", handleDeletePhoto)
	}

	// ---- Статика React ----
	// В собранном бинарнике фронтенд встроен (embed).
	// В dev-режиме, если версия без встроенной статики, читаем из ../frontend/dist.
	serveFrontend(r)

	// ---- Запуск ----
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Сервер запущен: http://localhost:%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatal(err)
	}
}

// serveFrontend отдаёт React-статистику: сначала из встроенного
// бинарника (release), при его отсутствии — из фронтовой папки (dev).
func serveFrontend(r *gin.Engine) {
	// 1) Встроенный фронтенд (собран через deploy-скрипт)
	if fsys := subFrontendDist(); hasIndex(fsys) {
		index, ok := indexData(fsys)
		if ok {
			// Статику ассетов отдаём из подпапки assets: StaticFS после
			// StripPrefix("/assets") ищет файлы прямо в корне переданной FS,
			// поэтому передаём именно подкаталог assets.
			if assetsFS, err := fs.Sub(fsys, "assets"); err == nil {
				r.StaticFS("/assets", http.FS(assetsFS))
			}

			r.NoRoute(func(c *gin.Context) {
				// http.FileServer редиректит прямые запросы /index.html
				// на ./ (301), поэтому отдаём HTML напрямую.
				c.Data(http.StatusOK, "text/html; charset=utf-8", index)
			})
			log.Printf("Frontend отдаётся из встроенного бинарника")
		}
		return
	}

	// 2) Фолбэк: файловая система (для локальной разработки)
	staticDir := filepath.Join("..", "frontend", "dist")
	if _, err := os.Stat(staticDir); err == nil {
		r.Static("/assets", filepath.Join(staticDir, "assets"))
		r.NoRoute(func(c *gin.Context) {
			index, err := os.ReadFile(filepath.Join(staticDir, "index.html"))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "no index.html"})
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", index)
		})
		log.Printf("Frontend подключён из %s", staticDir)
		return
	}

	// 3) Совсем нет фронтенда
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
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
