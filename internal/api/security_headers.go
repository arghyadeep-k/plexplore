package api

import (
	"net/http"
	"net/url"
	"strings"
)

const cspStaticDirectives = "default-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; object-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; font-src 'self'"

func setCommonSecurityHeaders(w http.ResponseWriter) {
	headers := w.Header()
	// HSTS is intentionally not set in-app today.
	// Production HSTS is owned by the HTTPS reverse proxy layer.
	headers.Set("X-Frame-Options", "DENY")
	headers.Set("X-Content-Type-Options", "nosniff")
	headers.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	headers.Set("Cross-Origin-Opener-Policy", "same-origin")
	headers.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
}

func setHTMLSecurityHeaders(w http.ResponseWriter, mapTiles MapTileConfig) {
	setCommonSecurityHeaders(w)
	w.Header().Set("Content-Security-Policy", buildCSPPolicy(mapTiles))
}

func buildCSPPolicy(mapTiles MapTileConfig) string {
	return cspStaticDirectives + "; " + buildImgSrcDirective(mapTiles)
}

func buildImgSrcDirective(mapTiles MapTileConfig) string {
	sources := []string{"'self'", "data:"}
	for _, origin := range allowedTileOrigins(mapTiles) {
		if !containsString(sources, origin) {
			sources = append(sources, origin)
		}
	}
	return "img-src " + strings.Join(sources, " ")
}

func allowedTileOrigins(mapTiles MapTileConfig) []string {
	mode := strings.ToLower(strings.TrimSpace(mapTiles.Mode))
	if mode == "" || mode == "none" || mode == "blank" || mode == "local" || mode == "self-hosted" {
		return nil
	}
	urlTemplate := strings.TrimSpace(mapTiles.URLTemplate)
	if urlTemplate == "" {
		return nil
	}
	origins := originsFromTileTemplate(urlTemplate)
	if mode == "osm" {
		origins = append(origins, "https://tile.openstreetmap.org")
	}
	return dedupeStrings(origins)
}

func originsFromTileTemplate(rawTemplate string) []string {
	normalized := strings.TrimSpace(rawTemplate)
	if normalized == "" {
		return nil
	}
	replacer := strings.NewReplacer("{s}", "a", "{z}", "0", "{x}", "0", "{y}", "0", "{r}", "", "{ext}", "png")
	sample := replacer.Replace(normalized)
	u, err := url.Parse(sample)
	if err != nil || u == nil {
		return nil
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "http" && scheme != "https" {
		return nil
	}
	if u.Host == "" || strings.ContainsAny(u.Host, "{}") {
		return nil
	}

	origin := scheme + "://" + strings.ToLower(u.Host)
	out := []string{origin}
	// For common {s}.host templates, allow a/b/c subdomains explicitly.
	if strings.Contains(rawTemplate, "{s}.") {
		baseHost := strings.TrimPrefix(strings.ToLower(u.Host), "a.")
		if baseHost != "" && baseHost != strings.ToLower(u.Host) {
			for _, sub := range []string{"a", "b", "c"} {
				out = append(out, scheme+"://"+sub+"."+baseHost)
			}
		}
	}
	return dedupeStrings(out)
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func containsString(values []string, needle string) bool {
	for _, v := range values {
		if v == needle {
			return true
		}
	}
	return false
}
