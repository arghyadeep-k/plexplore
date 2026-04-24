package api

import (
	"errors"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("test-pass-123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty password hash")
	}
	if !VerifyPassword(hash, "test-pass-123") {
		t.Fatal("expected password verification success")
	}
}

func TestVerifyPassword_WrongPasswordFails(t *testing.T) {
	hash, err := HashPassword("correct-pass-123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if VerifyPassword(hash, "wrong-pass") {
		t.Fatal("expected wrong password verification failure")
	}
}

func TestHashPassword_EmptyRejected(t *testing.T) {
	_, err := HashPassword("   ")
	if !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("expected ErrEmptyPassword, got %v", err)
	}
}

func TestHashPassword_TooShortRejected(t *testing.T) {
	_, err := HashPassword("shortpass")
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("expected ErrPasswordTooShort, got %v", err)
	}
}
