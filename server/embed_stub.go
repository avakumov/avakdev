//go:build !embed

package main

import "embed"

// frontendDist — пустая переменная, используемая сборками без тега embed
// (обычная локальная разработка). В этом режиме сервер отдаёт статику
// из файловой системы (../frontend/dist), а не из бинарника.
//
// Для продакшена собирайте с тегом embed: go build -tags embed
var frontendDist embed.FS // пустая: dev-сборка без тега embed

// practiceDist — пустая переменная без тега embed: в dev-режиме практика
// читается напрямую из exercises/ (EXERCISES_DIR), встраивание не нужно.
var practiceDist embed.FS
