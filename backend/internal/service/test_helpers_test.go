package service

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func mustHashPassword(t testing.TB, password string) string {
	t.Helper()

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("generate password hash: %v", err)
	}

	return string(passwordHash)
}
