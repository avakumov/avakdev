//go:build embed

package main

import "embed"

// frontendDist содержит собранный фронтенд (React), встроенный в бинарник.
// Сборка с тегом: go build -tags embed
// Скрипт deploy.sh копирует frontend/dist в server/frontend-dist перед сборкой.
//
//go:embed all:frontend-dist
var frontendDist embed.FS

// practiceDist — метаданные раздела «Практика»: структура тем/заданий
// (topic.json/task.json) и результаты прогонов (.results/*.json). Собирается
// Makefile'ом из exercises/ в server/practice-dist и встраивается в бинарник,
// чтобы раздел работал и на production. Код заданий (*.go) НЕ встраивается.
//
//go:embed all:practice-dist
var practiceDist embed.FS
