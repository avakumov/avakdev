package store

import (
	"context"
	"testing"
	"time"
)

// hashToken детерминирован, зависит от входа и даёт 64 hex-символа (sha256).
func TestHashToken(t *testing.T) {
	a := hashToken("token-1")
	if a != hashToken("token-1") {
		t.Fatal("hashToken должен быть детерминированным")
	}
	if a == hashToken("token-2") {
		t.Fatal("разные токены должны давать разные хэши")
	}
	if len(a) != 64 {
		t.Fatalf("ожидали 64 hex-символа, получили %d (%q)", len(a), a)
	}
}

// newToken выдаёт каждый раз новый токен длиной 64 hex-символа.
func TestNewToken(t *testing.T) {
	a, err := newToken()
	if err != nil {
		t.Fatalf("newToken: %v", err)
	}
	b, err := newToken()
	if err != nil {
		t.Fatalf("newToken: %v", err)
	}
	if a == b {
		t.Fatal("два токена не должны совпадать")
	}
	if len(a) != 64 {
		t.Fatalf("ожидали 64 hex-символа, получили %d", len(a))
	}
}

// Без БД обращения к сессиям безопасны: Get/Delete ничего не находят и не
// паникуют, Create возвращает понятную ошибку.
func TestSessionsNoDB(t *testing.T) {
	s := NewSessions(nil)
	ctx := context.Background()

	if _, err := s.Create(ctx, "avakdev", time.Hour); err == nil {
		t.Fatal("без БД Create должен вернуть ошибку")
	}
	if _, ok := s.Get(ctx, "токен"); ok {
		t.Fatal("без БД сессия не должна находиться")
	}
	s.Delete(ctx, "токен") // не должно паниковать
}
