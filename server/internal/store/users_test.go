package store

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// Открытый пароль проходит проверку и при этом возвращает bcrypt-хэш —
// это и есть «ленивая» домиграция при первом входе.
func TestCheckPasswordLegacyUpgrade(t *testing.T) {
	ok, newHash := checkPassword("secret", "secret")
	if !ok {
		t.Fatal("совпадающий открытый пароль должен проходить")
	}
	if newHash == "" {
		t.Fatal("при совпадении открытого пароля должен возвращаться хэш для записи")
	}
	if !isBcryptHash(newHash) {
		t.Fatalf("новый пароль не похож на bcrypt: %q", newHash)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(newHash), []byte("secret")); err != nil {
		t.Fatalf("хэш не соответствует паролю: %v", err)
	}

	if ok, _ := checkPassword("secret", "wrong"); ok {
		t.Fatal("неверный открытый пароль не должен проходить")
	}
}

// Готовый bcrypt-хэш проверяется напрямую и повторно не хэшируется.
func TestCheckPasswordBcrypt(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("hunter2"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}

	ok, newHash := checkPassword(string(hash), "hunter2")
	if !ok {
		t.Fatal("верный пароль должен проходить")
	}
	if newHash != "" {
		t.Fatal("bcrypt-пароль не нужно перехэшировать")
	}

	if ok, _ := checkPassword(string(hash), "nope"); ok {
		t.Fatal("неверный пароль не должен проходить")
	}
}
