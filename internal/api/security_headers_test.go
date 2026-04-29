package api

import (
	"strings"
	"testing"
)

func TestBuildCSPPolicy_NoneModeRestrictiveImgSrc(t *testing.T) {
	csp := buildCSPPolicy(MapTileConfig{Mode: "none"})
	if !strings.Contains(csp, "img-src 'self' data:") {
		t.Fatalf("expected restrictive img-src for none mode, got %q", csp)
	}
	if hasImgSrcWildcardScheme(csp) {
		t.Fatalf("expected no broad http/https wildcards in CSP, got %q", csp)
	}
}

func TestBuildCSPPolicy_OSMIncludesExpectedOrigins(t *testing.T) {
	csp := buildCSPPolicy(MapTileConfig{
		Mode:        "osm",
		URLTemplate: "https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png",
	})
	for _, want := range []string{
		"https://tile.openstreetmap.org",
		"https://a.tile.openstreetmap.org",
		"https://b.tile.openstreetmap.org",
		"https://c.tile.openstreetmap.org",
	} {
		if !strings.Contains(csp, want) {
			t.Fatalf("expected OSM origin %q in CSP, got %q", want, csp)
		}
	}
	if hasImgSrcWildcardScheme(csp) {
		t.Fatalf("expected no broad http/https wildcards in CSP, got %q", csp)
	}
}

func TestBuildCSPPolicy_CustomTemplateExtractsOrigin(t *testing.T) {
	csp := buildCSPPolicy(MapTileConfig{
		Mode:        "custom",
		URLTemplate: "http://tiles.local/{z}/{x}/{y}.png",
	})
	if !strings.Contains(csp, "http://tiles.local") {
		t.Fatalf("expected custom tile origin in CSP, got %q", csp)
	}
	if strings.Contains(csp, "tile.openstreetmap.org") {
		t.Fatalf("did not expect OSM origin for custom mode, got %q", csp)
	}
}

func TestBuildCSPPolicy_CustomInvalidTemplateFallsBackRestrictive(t *testing.T) {
	csp := buildCSPPolicy(MapTileConfig{
		Mode:        "custom",
		URLTemplate: "notaurl",
	})
	if strings.Contains(csp, "notaurl") {
		t.Fatalf("did not expect invalid template in CSP, got %q", csp)
	}
	if !strings.Contains(csp, "img-src 'self' data:") {
		t.Fatalf("expected restrictive fallback img-src, got %q", csp)
	}
}

func hasImgSrcWildcardScheme(csp string) bool {
	start := strings.Index(csp, "img-src ")
	if start < 0 {
		return false
	}
	img := csp[start:]
	end := strings.Index(img, ";")
	if end >= 0 {
		img = img[:end]
	}
	fields := strings.Fields(img)
	for _, f := range fields {
		if f == "http:" || f == "https:" {
			return true
		}
	}
	return false
}
