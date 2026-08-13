//go:build !embed

package main

import "embed"

// frontendDist — пустая переменная, используемая сборками без тега embed
// (обычная локальная разработка). В этом режиме сервер отдаёт статику
// из файловой системы (../frontend/dist), а не из бинарника.
//
// Для продакшена собирайте с тегом embed: go build -tags embed
//
//go:embed all:frontend-dist
var frontendDist embed.FS
