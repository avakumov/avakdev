// Package exercise — общий помощник для учебных заданий.
//
// В каждом тестовом файле задания (мы их пишем сами, ученик не трогает) есть:
//
//	func TestMain(m *testing.M) { exercise.Main(m) }
//
// Main перехватывает вывод тест-бинарника, прогоняет тесты и записывает
// результат в exercises/.results/<путь пакета>.json. Благодаря этому статус
// задания обновляется при ЛЮБОМ запуске `go test` — в терминале, из редактора
// (кнопка «run test»), с -run и т.д. Никаких отдельных команд не нужно.
//
// Вывод при этом не теряется: он и записывается в отчёт, и по-прежнему
// печатается в терминал.
package exercise

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// maxOutput — сколько символов вывода сохраняем в отчёт.
const maxOutput = 4000

// result — результат прогона задания (одного пакета).
type result struct {
	Passed         bool    `json:"passed"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	Output         string  `json:"output"`
	At             string  `json:"at"`
}

// Main запускает тесты и записывает результат. Вызывается из TestMain:
//
//	func TestMain(m *testing.M) { exercise.Main(m) }
func Main(m *testing.M) {
	start := time.Now()

	// Перехватываем вывод: он нужен и для отчёта, и для терминала.
	realStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		// Без перехвата всё равно прогоняем тесты — просто без записи.
		os.Exit(m.Run())
	}
	os.Stdout = w

	var captured strings.Builder
	done := make(chan struct{})
	go func() {
		// Пишем одновременно в настоящий stdout (чтобы вывод был виден)
		// и в буфер (чтобы сохранить в отчёт).
		_, _ = io.Copy(io.MultiWriter(realStdout, &captured), r)
		close(done)
	}()

	code := m.Run()

	_ = w.Close()
	<-done
	os.Stdout = realStdout

	writeResult(code, captured.String(), time.Since(start))
	os.Exit(code)
}

// writeResult сохраняет результат текущего пакета в .results/<путь>.json.
// Любая проблема с записью не должна ломать прогон тестов — просто пропускаем.
func writeResult(code int, output string, elapsed time.Duration) {
	root, rel, ok := packageLocation()
	if !ok {
		return
	}

	res := result{
		Passed:         code == 0,
		ElapsedSeconds: elapsed.Seconds(),
		Output:         truncate(output, maxOutput),
		At:             time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return
	}

	path := filepath.Join(root, ".results", filepath.FromSlash(rel)) + ".json"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(path, append(data, '\n'), 0o644)
}

// packageLocation находит корень модуля (по go.mod вверх от рабочего каталога
// теста) и путь текущего пакета относительно него.
func packageLocation() (root, rel string, ok bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", false
	}

	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			root = dir
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false
		}
		dir = parent
	}

	rel, err = filepath.Rel(root, cwd)
	if err != nil {
		return "", "", false
	}
	rel = filepath.ToSlash(rel)
	if rel == "." || rel == "" || strings.HasPrefix(rel, "..") {
		return "", "", false
	}
	return root, rel, true
}

// truncate обрезает строку до max рун, приписывая «…», если что-то отброшено.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "\n…"
}
