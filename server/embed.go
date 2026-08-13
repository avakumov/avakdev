//go:build embed

package main

import "embed"

// frontendDist содержит собранный фронтенд (React), встроенный в бинарник.
// Сборка с тегом: go build -tags embed
// Скрипт deploy.sh копирует frontend/dist в server/frontend-dist перед сборкой.
//
//go:embed all:frontend-dist
var frontendDist embed.FS
