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

	r := gin.Default()

	// ---- API ----
	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status": "ok",
				"time":   time.Now().Format(time.RFC3339),
			})
		})

		api.GET("/message", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Привет! Это ответ от Go (Gin) сервера 🚀",
				"server":  "gin",
			})
		})
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
