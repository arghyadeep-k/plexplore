package visits

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type PlaceLabelCache interface {
	GetVisitPlaceLabel(ctx context.Context, provider, latKey, lonKey string) (string, bool, error)
	UpsertVisitPlaceLabel(ctx context.Context, provider, latKey, lonKey, label string) error
}

type ReverseGeocodeProvider interface {
	Name() string
	ReverseGeocode(ctx context.Context, lat, lon float64) (string, error)
}

type ReverseGeocodeConfig struct {
	Enabled              bool
	Provider             string
	NominatimURL         string
	UserAgent            string
	Timeout              time.Duration
	CacheDecimals        int
	MaxLookupsPerRequest int
}

type LabelResolver struct {
	enabled              bool
	cache                PlaceLabelCache
	provider             ReverseGeocodeProvider
	cacheDecimals        int
	maxLookupsPerRequest int
}

func NewLabelResolver(cfg ReverseGeocodeConfig, cache PlaceLabelCache) (*LabelResolver, error) {
	resolver := &LabelResolver{
		enabled:              cfg.Enabled,
		cache:                cache,
		cacheDecimals:        clampCacheDecimals(cfg.CacheDecimals),
		maxLookupsPerRequest: cfg.MaxLookupsPerRequest,
	}
	if resolver.maxLookupsPerRequest <= 0 {
		resolver.maxLookupsPerRequest = 3
	}
	if !resolver.enabled {
		return resolver, nil
	}
	if resolver.cache == nil {
		return nil, fmt.Errorf("reverse geocode cache store is required when enabled")
	}

	providerName := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if providerName == "" {
		providerName = "nominatim"
	}
	switch providerName {
	case "nominatim":
		resolver.provider = NewNominatimProvider(cfg.NominatimURL, cfg.UserAgent, cfg.Timeout)
	default:
		return nil, fmt.Errorf("unsupported reverse geocode provider: %s", providerName)
	}
	return resolver, nil
}

func (r *LabelResolver) Enabled() bool {
	return r != nil && r.enabled
}

func (r *LabelResolver) MaxProviderLookupsPerRequest() int {
	if r == nil {
		return 0
	}
	return r.maxLookupsPerRequest
}

func (r *LabelResolver) ResolveVisitLabel(ctx context.Context, lat, lon float64, allowProvider bool) (string, bool, error) {
	if r == nil || !r.enabled || r.provider == nil || r.cache == nil {
		return "", false, nil
	}

	latKey := coordKey(lat, r.cacheDecimals)
	lonKey := coordKey(lon, r.cacheDecimals)
	cached, ok, err := r.cache.GetVisitPlaceLabel(ctx, r.provider.Name(), latKey, lonKey)
	if err != nil {
		return "", false, err
	}
	if ok {
		return strings.TrimSpace(cached), false, nil
	}
	if !allowProvider {
		return "", false, nil
	}

	label, err := r.provider.ReverseGeocode(ctx, lat, lon)
	if err != nil {
		return "", true, err
	}
	label = strings.TrimSpace(label)
	if label == "" {
		return "", true, nil
	}
	if err := r.cache.UpsertVisitPlaceLabel(ctx, r.provider.Name(), latKey, lonKey, label); err != nil {
		return "", true, err
	}
	return label, true, nil
}

func clampCacheDecimals(value int) int {
	if value < 3 {
		return 3
	}
	if value > 7 {
		return 7
	}
	return value
}

func coordKey(value float64, decimals int) string {
	if decimals < 0 {
		decimals = 4
	}
	pow := math.Pow10(decimals)
	rounded := math.Round(value*pow) / pow
	return strconv.FormatFloat(rounded, 'f', decimals, 64)
}
