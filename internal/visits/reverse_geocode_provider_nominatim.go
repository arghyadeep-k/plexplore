package visits

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type NominatimProvider struct {
	baseURL   string
	userAgent string
	client    *http.Client
}

func NewNominatimProvider(baseURL, userAgent string, timeout time.Duration) *NominatimProvider {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://nominatim.openstreetmap.org/reverse"
	}
	if strings.TrimSpace(userAgent) == "" {
		userAgent = "plexplore/1.0 (+self-hosted)"
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return &NominatimProvider{
		baseURL:   baseURL,
		userAgent: userAgent,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (p *NominatimProvider) Name() string {
	return "nominatim"
}

func (p *NominatimProvider) ReverseGeocode(ctx context.Context, lat, lon float64) (string, error) {
	parsedURL, err := url.Parse(p.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse nominatim base url: %w", err)
	}
	query := parsedURL.Query()
	query.Set("format", "jsonv2")
	query.Set("lat", strconv.FormatFloat(lat, 'f', 6, 64))
	query.Set("lon", strconv.FormatFloat(lon, 'f', 6, 64))
	query.Set("zoom", "16")
	query.Set("addressdetails", "0")
	parsedURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("build reverse geocode request: %w", err)
	}
	req.Header.Set("User-Agent", p.userAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("reverse geocode request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("reverse geocode status %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	var parsed struct {
		DisplayName string `json:"display_name"`
		Name        string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode reverse geocode response: %w", err)
	}
	if strings.TrimSpace(parsed.DisplayName) != "" {
		return strings.TrimSpace(parsed.DisplayName), nil
	}
	return strings.TrimSpace(parsed.Name), nil
}
