package app

import (
	"errors"
	"time"
)

// ParseDay нормализует "YYYY-MM-DD"; пусто — сегодня (локальная дата).
func ParseDay(s string) (string, error) {
	if s == "" {
		return time.Now().Format("2006-01-02"), nil
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return "", errors.New("некорректная дата (ожидается ГГГГ-ММ-ДД)")
	}
	return s, nil
}
