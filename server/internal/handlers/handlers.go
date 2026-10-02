// Package handlers — HTTP-слой: обработчики маршрутов. Зависимости (хранилища,
// сессии) передаются в Handlers, поэтому логика данных живёт в internal/store и
// internal/app, а не в глобальном состоянии пакета.
package handlers

import "avakumov/server/internal/app"

// Handlers — HTTP-обработчики приложения.
type Handlers struct {
	App *app.App
}

// New создаёт набор обработчиков на готовом приложении.
func New(a *app.App) *Handlers { return &Handlers{App: a} }
