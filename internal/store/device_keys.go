package store

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func hashDeviceAPIKey(apiKey string) string {
	trimmed := strings.TrimSpace(apiKey)
	if trimmed == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(sum[:])
}

func buildAPIKeyPreview(apiKey string) string {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return ""
	}
	if len(key) <= 6 {
		return key[:1] + "..." + key[len(key)-1:]
	}
	prefix := key[:4]
	suffix := key[len(key)-4:]
	return prefix + "..." + suffix
}

func generateAPIKeySentinel() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api key sentinel: %w", err)
	}
	return "hashonly:" + hex.EncodeToString(buf), nil
}
