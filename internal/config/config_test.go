package config

import "testing"

func TestLoad_DefaultDeploymentModeIsDevelopment(t *testing.T) {
	t.Setenv("APP_DEPLOYMENT_MODE", "")
	t.Setenv("APP_COOKIE_SECURE_MODE", "")
	t.Setenv("APP_EXPECT_TLS_TERMINATION", "")

	cfg := Load()
	if cfg.DeploymentMode != "development" {
		t.Fatalf("expected default deployment mode development, got %q", cfg.DeploymentMode)
	}
	if cfg.CookieSecureMode != "auto" {
		t.Fatalf("expected development default cookie mode auto, got %q", cfg.CookieSecureMode)
	}
	if cfg.ExpectTLSTermination {
		t.Fatalf("expected development default expect TLS termination=false")
	}
	if cfg.AllowInsecureHTTP {
		t.Fatalf("expected default allow insecure http=false")
	}
}

func TestLoad_ProductionDefaultsEnforceTLSBackedCookieBehavior(t *testing.T) {
	t.Setenv("APP_DEPLOYMENT_MODE", "production")
	t.Setenv("APP_COOKIE_SECURE_MODE", "")
	t.Setenv("APP_EXPECT_TLS_TERMINATION", "")

	cfg := Load()
	if cfg.DeploymentMode != "production" {
		t.Fatalf("expected deployment mode production, got %q", cfg.DeploymentMode)
	}
	if cfg.CookieSecureMode != "always" {
		t.Fatalf("expected production default cookie mode always, got %q", cfg.CookieSecureMode)
	}
	if !cfg.ExpectTLSTermination {
		t.Fatalf("expected production default expect TLS termination=true")
	}
}

func TestLoad_DevelopmentExplicitInsecureCookieModeStillPossible(t *testing.T) {
	t.Setenv("APP_DEPLOYMENT_MODE", "development")
	t.Setenv("APP_COOKIE_SECURE_MODE", "never")
	t.Setenv("APP_ALLOW_INSECURE_HTTP", "true")

	cfg := Load()
	if cfg.CookieSecureMode != "never" {
		t.Fatalf("expected explicit development cookie mode never, got %q", cfg.CookieSecureMode)
	}
	if !cfg.AllowInsecureHTTP {
		t.Fatalf("expected explicit allow insecure http=true")
	}
}

func TestLoad_DefaultMapTileConfigIsPrivacyPreserving(t *testing.T) {
	t.Setenv("APP_MAP_TILE_MODE", "")
	t.Setenv("APP_MAP_TILE_URL_TEMPLATE", "")
	t.Setenv("APP_MAP_TILE_ATTRIBUTION", "")

	cfg := Load()
	if cfg.MapTileMode != "none" {
		t.Fatalf("expected default map tile mode none, got %q", cfg.MapTileMode)
	}
	if cfg.MapTileURLTemplate == "" {
		t.Fatalf("expected non-empty default tile url template")
	}
}

func TestLoad_CustomMapTileConfig(t *testing.T) {
	t.Setenv("APP_MAP_TILE_MODE", "custom")
	t.Setenv("APP_MAP_TILE_URL_TEMPLATE", "http://tiles.local/{z}/{x}/{y}.png")
	t.Setenv("APP_MAP_TILE_ATTRIBUTION", "local tile server")

	cfg := Load()
	if cfg.MapTileMode != "custom" {
		t.Fatalf("expected custom map tile mode, got %q", cfg.MapTileMode)
	}
	if cfg.MapTileURLTemplate != "http://tiles.local/{z}/{x}/{y}.png" {
		t.Fatalf("unexpected custom tile template: %q", cfg.MapTileURLTemplate)
	}
	if cfg.MapTileAttribution != "local tile server" {
		t.Fatalf("unexpected custom tile attribution: %q", cfg.MapTileAttribution)
	}
}
