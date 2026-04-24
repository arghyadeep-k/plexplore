package main

import (
	"testing"

	"plexplore/internal/config"
)

func TestValidateRuntimeSecurityConfig_ProductionRequiresSecureCookies(t *testing.T) {
	cfg := config.Config{
		DeploymentMode:    "production",
		CookieSecureMode:  "never",
		AllowInsecureHTTP: false,
	}
	if err := validateRuntimeSecurityConfig(cfg); err == nil {
		t.Fatalf("expected validation error for production insecure cookie mode")
	}
}

func TestValidateRuntimeSecurityConfig_InsecureCookieModeNeedsExplicitOptIn(t *testing.T) {
	cfg := config.Config{
		DeploymentMode:    "development",
		CookieSecureMode:  "never",
		AllowInsecureHTTP: false,
	}
	if err := validateRuntimeSecurityConfig(cfg); err == nil {
		t.Fatalf("expected validation error without explicit insecure opt-in")
	}
}

func TestValidateRuntimeSecurityConfig_DevelopmentInsecureOptInAllowed(t *testing.T) {
	cfg := config.Config{
		DeploymentMode:    "development",
		CookieSecureMode:  "never",
		AllowInsecureHTTP: true,
	}
	if err := validateRuntimeSecurityConfig(cfg); err != nil {
		t.Fatalf("expected explicit development insecure mode to pass, got %v", err)
	}
}

func TestValidateRuntimeSecurityConfig_ProductionRejectsInsecureOptIn(t *testing.T) {
	cfg := config.Config{
		DeploymentMode:    "production",
		CookieSecureMode:  "always",
		AllowInsecureHTTP: true,
	}
	if err := validateRuntimeSecurityConfig(cfg); err == nil {
		t.Fatalf("expected validation error for production with insecure opt-in")
	}
}
