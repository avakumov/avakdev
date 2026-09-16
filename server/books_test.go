package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Конвертация тестовых книг из папки ../books (если она есть).
// Проверяет, что FB2 и EPUB дают непустой HTML и метаданные.
func TestConvertLocalBooks(t *testing.T) {
	files, err := filepath.Glob("../books/*")
	if err != nil || len(files) == 0 {
		t.Skip("нет папки ../books с книгами для теста")
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("не удалось прочитать %s: %v", filepath.Base(f), err)
		}
		format, title, author, body, err := convertBook(filepath.Base(f), data)
		if err != nil {
			t.Errorf("%s: ошибка конвертации: %v", filepath.Base(f), err)
			continue
		}
		if format != "fb2" && format != "epub" {
			t.Errorf("%s: неожиданный формат %q", filepath.Base(f), format)
		}
		if strings.TrimSpace(title) == "" {
			t.Errorf("%s: пустое название", filepath.Base(f))
		}
		if len([]rune(body)) < 1000 {
			t.Errorf("%s: слишком короткий HTML (%d символов)",
				filepath.Base(f), len([]rune(body)))
		}
		t.Logf("%s: format=%s title=%q author=%q html=%d",
			filepath.Base(f), format, title, author, len([]rune(body)))
	}
}

// Нормализация XHTML: самозакрытые теги разворачиваются в пары, иначе
// HTML-парсер «съедает» текст (RCDATA у <title/>).
func TestNormalizeXHTMLSelfClosing(t *testing.T) {
	in := `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><title/><link rel="stylesheet" href="style.css"/></head>
<body class="z"><div class="title"/><p>Привет</p></body>
</html>`
	out := string(normalizeXHTML([]byte(in)))
	if strings.Contains(out, "<title/>") {
		t.Fatalf("самозакрытый <title/> остался: %s", out)
	}
	if strings.Contains(out, "<head>") {
		t.Fatalf("блок <head> не удалён: %s", out)
	}
	if !strings.Contains(out, "</div>") || !strings.Contains(out, "</p>") {
		t.Fatalf("пары тегов не восстановлены: %s", out)
	}
	if !strings.Contains(out, "Привет") {
		t.Fatalf("текст потерян: %s", out)
	}
}
