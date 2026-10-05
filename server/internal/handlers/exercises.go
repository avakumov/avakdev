package handlers

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"avakumov/server/internal/env"
)

// Раздел «Практика»: сервер ничего не исполняет. Он строит иерархию заданий
// из дерева каталогов EXERCISES_DIR (вложенность папок = темы/подтемы, папка с
// task.json = задание) и подмешивает результаты прогонов тестов.
//
// В dev-режиме дерево читается с диска. На production каталога нет — тогда
// main разворачивает во временный каталог ВСТРОЕННЫЕ метаданные практики
// (структура + .results, без кода заданий) и направляет туда EXERCISES_DIR.
//
// Результаты пишут САМИ тесты (см. exercises/internal/exercise + TestMain),
// поэтому статус обновляется при любом `go test` — в терминале или редакторе.
//
// Названия тем берутся из topic.json, заданий — из task.json (заголовок и
// описание); если файла нет, заголовок выводится из имени папки.

// skipDirs — служебные каталоги, которые не являются темами/заданиями.
var skipDirs = map[string]bool{
	"internal":     true,
	"vendor":       true,
	"node_modules": true,
	"testdata":     true,
}

// exerciseRun — результат прогона задания (файл .results/<путь>.json).
type exerciseRun struct {
	Passed         bool    `json:"passed"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	Output         string  `json:"output"`
	Error          string  `json:"error"`
	At             string  `json:"at"`
}

// nodeMeta — метаданные темы/задания (topic.json / task.json).
type nodeMeta struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

// exerciseNode — узел дерева заданий (тема или задание).
type exerciseNode struct {
	Key         string         `json:"key"`
	Title       string         `json:"title"`
	Kind        string         `json:"kind"` // "topic" | "task"
	Description string         `json:"description,omitempty"`
	File        string         `json:"file,omitempty"`
	Passed      bool           `json:"passed"`
	PassedCount int            `json:"passed_count"`
	TotalCount  int            `json:"total_count"`
	Elapsed     float64        `json:"elapsed_seconds,omitempty"`
	Output      string         `json:"output,omitempty"`
	Error       string         `json:"error,omitempty"`
	Children    []exerciseNode `json:"children,omitempty"`
}

// Exercises отдаёт дерево заданий и результаты их проверки (GET /api/exercises).
// Каталог заданий ищется в EXERCISES_DIR (по умолчанию ../exercises относительно
// рабочей директории сервера).
func (h *Handlers) Exercises(w http.ResponseWriter, r *http.Request) {
	root := env.GetenvOrEnvFile("EXERCISES_DIR", "../exercises")
	resultsDir := filepath.Join(root, ".results")

	available := false
	if fi, err := os.Stat(root); err == nil && fi.IsDir() {
		available = true
	}
	nodes := walkExercises(root, resultsDir, "", "")
	if nodes == nil {
		nodes = []exerciseNode{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"generated_at": latestResultTime(resultsDir),
		"available":    available,
		"nodes":        nodes,
	})
}

// walkExercises рекурсивно строит узлы дерева для каталога dir.
// rel — путь dir относительно корня (в стиле URL, через «/»).
func walkExercises(root, resultsDir, dir, rel string) []exerciseNode {
	if dir == "" {
		dir = root
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || strings.HasPrefix(name, ".") || skipDirs[name] {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	nodes := make([]exerciseNode, 0, len(names))
	for _, name := range names {
		childDir := filepath.Join(dir, name)
		childRel := name
		if rel != "" {
			childRel = rel + "/" + name
		}
		if isTaskDir(childDir) {
			nodes = append(nodes, taskNode(childDir, childRel, resultsDir))
			continue
		}
		children := walkExercises(root, resultsDir, childDir, childRel)
		// Тема показывается даже без заданий, если помечена topic.json.
		if len(children) == 0 && !isTopicDir(childDir) {
			continue
		}
		nodes = append(nodes, topicNode(childDir, childRel, children))
	}
	return nodes
}

// isTaskDir сообщает, что каталог — это задание: в нём есть task.json.
// Маркер по метаданным (а не по *_test.go) выбран специально: на production
// до сервера доезжают только метаданные (*.json), без кода заданий.
func isTaskDir(dir string) bool {
	fi, err := os.Stat(filepath.Join(dir, "task.json"))
	return err == nil && !fi.IsDir()
}

// isTopicDir сообщает, что каталог — это тема: в нём есть topic.json.
// Благодаря этому тема отображается даже без заданий.
func isTopicDir(dir string) bool {
	fi, err := os.Stat(filepath.Join(dir, "topic.json"))
	return err == nil && !fi.IsDir()
}

// taskNode собирает узел-задание вместе с результатом прогона.
func taskNode(dir, key, resultsDir string) exerciseNode {
	meta := readMeta(filepath.Join(dir, "task.json"))
	n := exerciseNode{
		Key:         key,
		Kind:        "task",
		Title:       meta.Title,
		Description: meta.Description,
		File:        firstGoFile(dir, key),
		TotalCount:  1,
	}
	if n.Title == "" {
		n.Title = prettify(filepath.Base(dir))
	}

	var run exerciseRun
	if err := readJSONFile(filepath.Join(resultsDir, filepath.FromSlash(key)+".json"), &run); err == nil {
		n.Passed = run.Passed
		n.Elapsed = run.ElapsedSeconds
		n.Output = run.Output
		n.Error = run.Error
	}
	if n.Passed {
		n.PassedCount = 1
	}
	return n
}

// topicNode собирает узел-тему и агрегирует счётчики по детям.
func topicNode(dir, key string, children []exerciseNode) exerciseNode {
	meta := readMeta(filepath.Join(dir, "topic.json"))
	n := exerciseNode{
		Key:      key,
		Kind:     "topic",
		Title:    meta.Title,
		Children: children,
	}
	if n.Title == "" {
		n.Title = prettify(filepath.Base(dir))
	}
	for _, c := range children {
		n.PassedCount += c.PassedCount
		n.TotalCount += c.TotalCount
	}
	n.Passed = n.TotalCount > 0 && n.PassedCount == n.TotalCount
	return n
}

// latestResultTime возвращает время последнего прогона — максимум поля `at`
// по всем файлам .results. Пусто, если результатов ещё нет.
// (Строки RFC3339 в UTC сравниваются лексикографически.)
func latestResultTime(dir string) string {
	latest := ""
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		var run exerciseRun
		if readJSONFile(path, &run) == nil && run.At > latest {
			latest = run.At
		}
		return nil
	})
	return latest
}

// readMeta читает topic.json/task.json; при любой ошибке возвращает пустые
// метаданные (заголовок тогда выведется из имени папки).
func readMeta(path string) nodeMeta {
	var m nodeMeta
	_ = readJSONFile(path, &m)
	return m
}

// firstGoFile возвращает путь первого (по алфавиту) обычного .go файла задания
// относительно корня exercises — для подсказки, где писать решение.
func firstGoFile(dir, key string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return key + "/" + names[0]
}

// prettify делает человекочитаемый заголовок из имени папки: убирает числовой
// префикс (01-, 02_) и заменяет разделители пробелами.
func prettify(name string) string {
	s := name
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 && i < len(s) && (s[i] == '-' || s[i] == '_' || s[i] == '.') {
		s = s[i+1:]
	}
	s = strings.NewReplacer("-", " ", "_", " ").Replace(s)
	return strings.TrimSpace(s)
}

// readJSONFile читает и разбирает JSON-файл по указанному пути.
func readJSONFile(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
