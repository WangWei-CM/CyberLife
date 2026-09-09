package httpapi

import (
	"testing"

	"cyberlife/server/internal/config"
)

func TestReaderKeyRoutesAreWriterOwned(t *testing.T) {
	routes := New(config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).Router().Routes()
	seen := map[string]bool{}
	for _, route := range routes {
		seen[route.Method+" "+route.Path] = true
	}
	for _, route := range []struct{ method, path string }{
		{"GET", "/api/v1/now/reader-keys"},
		{"POST", "/api/v1/now/reader-keys"},
		{"POST", "/api/v1/now/reader-keys/:id/revoke"},
	} {
		if !seen[route.method+" "+route.path] {
			t.Fatalf("missing writer reader-key route %s %s", route.method, route.path)
		}
	}
	for _, route := range []string{
		"GET /api/v1/admin/writers/:lifeID/reader-keys",
		"POST /api/v1/admin/writers/:lifeID/reader-keys",
		"POST /api/v1/admin/reader-keys/:id/revoke",
	} {
		if seen[route] {
			t.Fatalf("reader-key route still exposed in admin domain: %s", route)
		}
	}
}
