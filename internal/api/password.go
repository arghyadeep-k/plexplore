package api

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const MinPasswordLength = 12

var ErrEmptyPassword = errors.New("password is required")
var ErrPasswordTooShort = errors.New("password must be at least 12 characters")

func HashPassword(plain string) (string, error) {
	normalized := strings.TrimSpace(plain)
	if normalized == "" {
		return "", ErrEmptyPassword
	}
	if len(normalized) < MinPasswordLength {
		return "", ErrPasswordTooShort
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(normalized), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func VerifyPassword(hash, plain string) bool {
	if strings.TrimSpace(hash) == "" || strings.TrimSpace(plain) == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(strings.TrimSpace(plain))) == nil
}
