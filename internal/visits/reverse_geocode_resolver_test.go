package visits

import (
	"context"
	"errors"
	"testing"
)

type fakePlaceCache struct {
	values map[string]string
}

func (f *fakePlaceCache) GetVisitPlaceLabel(_ context.Context, provider, latKey, lonKey string) (string, bool, error) {
	value, ok := f.values[provider+"|"+latKey+"|"+lonKey]
	return value, ok, nil
}

func (f *fakePlaceCache) UpsertVisitPlaceLabel(_ context.Context, provider, latKey, lonKey, label string) error {
	if f.values == nil {
		f.values = map[string]string{}
	}
	f.values[provider+"|"+latKey+"|"+lonKey] = label
	return nil
}

type fakeProvider struct {
	label string
	err   error
	calls int
}

func (f *fakeProvider) Name() string { return "fake" }

func (f *fakeProvider) ReverseGeocode(_ context.Context, _, _ float64) (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return f.label, nil
}

func TestLabelResolver_CacheHitSkipsProvider(t *testing.T) {
	cache := &fakePlaceCache{
		values: map[string]string{
			"fake|41.1234|-87.9876": "Cached Label",
		},
	}
	provider := &fakeProvider{label: "Remote Label"}
	resolver := &LabelResolver{
		enabled:              true,
		cache:                cache,
		provider:             provider,
		cacheDecimals:        4,
		maxLookupsPerRequest: 3,
	}

	label, usedProvider, err := resolver.ResolveVisitLabel(context.Background(), 41.1234, -87.9876, true)
	if err != nil {
		t.Fatalf("ResolveVisitLabel failed: %v", err)
	}
	if usedProvider {
		t.Fatal("expected cache hit without provider usage")
	}
	if label != "Cached Label" {
		t.Fatalf("expected cached label, got %q", label)
	}
	if provider.calls != 0 {
		t.Fatalf("expected provider calls 0, got %d", provider.calls)
	}
}

func TestLabelResolver_CacheMissStoresValue(t *testing.T) {
	cache := &fakePlaceCache{values: map[string]string{}}
	provider := &fakeProvider{label: "Remote Label"}
	resolver := &LabelResolver{
		enabled:              true,
		cache:                cache,
		provider:             provider,
		cacheDecimals:        4,
		maxLookupsPerRequest: 3,
	}

	first, usedFirst, err := resolver.ResolveVisitLabel(context.Background(), 41.10001, -87.10001, true)
	if err != nil {
		t.Fatalf("first ResolveVisitLabel failed: %v", err)
	}
	if !usedFirst {
		t.Fatal("expected provider usage on first cache miss")
	}
	if first != "Remote Label" {
		t.Fatalf("expected remote label, got %q", first)
	}

	second, usedSecond, err := resolver.ResolveVisitLabel(context.Background(), 41.10000, -87.10000, false)
	if err != nil {
		t.Fatalf("second ResolveVisitLabel failed: %v", err)
	}
	if usedSecond {
		t.Fatal("expected second lookup from cache only")
	}
	if second != "Remote Label" {
		t.Fatalf("expected cached label on second lookup, got %q", second)
	}
	if provider.calls != 1 {
		t.Fatalf("expected provider called once, got %d", provider.calls)
	}
}

func TestLabelResolver_DisallowProviderOnMiss(t *testing.T) {
	cache := &fakePlaceCache{values: map[string]string{}}
	provider := &fakeProvider{label: "Remote Label"}
	resolver := &LabelResolver{
		enabled:              true,
		cache:                cache,
		provider:             provider,
		cacheDecimals:        4,
		maxLookupsPerRequest: 3,
	}

	label, usedProvider, err := resolver.ResolveVisitLabel(context.Background(), 41.0, -87.0, false)
	if err != nil {
		t.Fatalf("ResolveVisitLabel failed: %v", err)
	}
	if label != "" {
		t.Fatalf("expected empty label when provider disallowed, got %q", label)
	}
	if usedProvider {
		t.Fatal("expected provider not used when disallowed")
	}
	if provider.calls != 0 {
		t.Fatalf("expected provider calls 0, got %d", provider.calls)
	}
}

func TestLabelResolver_ProviderError(t *testing.T) {
	cache := &fakePlaceCache{values: map[string]string{}}
	provider := &fakeProvider{err: errors.New("provider down")}
	resolver := &LabelResolver{
		enabled:              true,
		cache:                cache,
		provider:             provider,
		cacheDecimals:        4,
		maxLookupsPerRequest: 3,
	}

	_, usedProvider, err := resolver.ResolveVisitLabel(context.Background(), 41.0, -87.0, true)
	if !usedProvider {
		t.Fatal("expected provider attempt on cache miss")
	}
	if err == nil {
		t.Fatal("expected provider error")
	}
}
