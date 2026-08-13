package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

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
	// Сервер раздаёт собранный фронтенд из ../frontend/dist
	staticDir := filepath.Join("..", "frontend", "dist")

	if _, err := os.Stat(staticDir); err == nil {
		// Ассеты (js/css) отдаём как статику
		r.Static("/assets", filepath.Join(staticDir, "assets"))

		// Любой неизвестный маршрут (SPA-фолбэк) отдаёт index.html
		r.NoRoute(func(c *gin.Context) {
			c.File(filepath.Join(staticDir, "index.html"))
		})
		log.Printf("Frontend подключён из %s", staticDir)
	} else {
		// Фронтенд не собран — запусти vite dev (npm run dev) и проксируй через него
		r.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "not found",
				"hint":  "соберите фронтенд: cd frontend && npm run build",
			})
		})
	}

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
