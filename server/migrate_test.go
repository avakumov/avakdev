package main

import (
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Sanity-проверка встроенных миграций (без БД): goose-файлы должны иметь имя
// NNNNN_name.sql, секции "-- +goose Up" / "-- +goose Down" и уникальные
// версии по возрастанию. Ловит опечатки до запуска против реальной базы.
func TestMigrationsValid(t *testing.T) {
	var names []string
	err := fs.WalkDir(migrationsFS, "migrations", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".sql") {
			names = append(names, d.Name())
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	sort.Strings(names)
	if len(names) == 0 {
		t.Fatal("нет файлов миграций")
	}

	prev := int64(0)
	for _, name := range names {
		// Имя должно начинаться с числовой версии: NNNNN_name.sql.
		version, err := strconv.ParseInt(strings.SplitN(name, "_", 2)[0], 10, 64)
		if err != nil {
			t.Fatalf("%s: имя должно начинаться с числовой версии", name)
		}
		if version <= prev {
			t.Fatalf("%s: версия %d не возрастает (предыдущая %d)", name, version, prev)
		}
		prev = version

		b, err := fs.ReadFile(migrationsFS, "migrations/"+name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		content := string(b)
		if !strings.Contains(content, "-- +goose Up") {
			t.Fatalf("%s: нет секции -- +goose Up", name)
		}
		if !strings.Contains(content, "-- +goose Down") {
			t.Fatalf("%s: нет секции -- +goose Down", name)
		}
	}
}
