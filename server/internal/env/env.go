// Package env загружает переменные окружения из .env и даёт доступ к ним.
// Используется main (загрузка при старте) и agent (чтение настроек).
package env

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Load читает переменные окружения из .env файла (формат KEY=VALUE, по
// одной на строку, пустые строки и строки с # в начале игнорируются), если
// эти переменные ещё не заданы в системном окружении.
//
// Поиск файла выполняется в текущей рабочей директории и на один уровень выше
// (для запуска из подкаталога server/). Реально существующие переменные
// окружения имеют приоритет и не перезаписываются.
func Load() {
	found := map[string]string{}

	candidates := []string{".env", filepath.Join("..", ".env")}
	seen := map[string]bool{}
	for _, p := range candidates {
		if seen[p] {
			continue
		}
		seen[p] = true
		parseFile(p, found)
	}

	for k, v := range found {
		if val := os.Getenv(k); val == "" {
			os.Setenv(k, v)
		}
	}
}

// parseFile разбирает файл .env и складывает прочитанные пары в out.
// Существующие в out ключи не перезаписываются (приоритет первого файла).
func parseFile(path string, out map[string]string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexRune(line, '=')
		if eq < 1 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		value := strings.TrimSpace(line[eq+1:])
		if key == "" {
			continue
		}
		// Не перезаписываем уже найденное значение.
		if _, ok := out[key]; !ok {
			out[key] = value
		}
	}
}

// GetenvOrEnvFile возвращает значение переменной окружения name, либо берёт его
// из файла .env (если окружение не установлено), либо возвращает значение по
// умолчанию def, если нигде не нашлось.
//
// Удобно для опциональных настроек, когда файл .env подхватывается автоматически.
func GetenvOrEnvFile(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	vals := map[string]string{}
	parseFile(".env", vals)
	if v, ok := vals[name]; ok {
		return v
	}
	parseFile(filepath.Join("..", ".env"), vals)
	if v, ok := vals[name]; ok {
		return v
	}
	return def
}
