package web

import (
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":               {Data: []byte("<!doctype html><title>DataDeck shell</title>")},
		"404.html":                 {Data: []byte("<html><body>not found document</body></html>")},
		"manifest.json":            {Data: []byte(`{"name":"DataDeck Studio"}`)},
		"sw.js":                    {Data: []byte("self.addEventListener('install',()=>{})")},
		"icons/icon-192x192.png":   {Data: []byte{0x89, 0x50, 0x4e, 0x47}},
		"_next/static/chunks/a.js": {Data: []byte("console.log('chunk')")},
		"_next/static/css/a.css":   {Data: []byte("body{}")},
		"docs/index.html":          {Data: []byte("<html>docs route</html>")},
	}
}

func newTestHandler() *Handler {
	return NewHandler(testFS(), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestServeIndex(t *testing.T) {
	h := newTestHandler()
	rec := get(t, h, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "DataDeck shell") {
		t.Errorf("body = %q", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content-type = %q", ct)
	}
	if rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("cache-control = %q, want no-cache", rec.Header().Get("Cache-Control"))
	}
}

func TestServeAssetsAndPWA(t *testing.T) {
	h := newTestHandler()
	cases := []struct {
		path        string
		contentType string
		cache       string
		body        string
	}{
		{"/_next/static/chunks/a.js", "text/javascript", "immutable", "chunk"},
		{"/_next/static/css/a.css", "text/css", "immutable", "body{}"},
		{"/manifest.json", "application/json", "no-cache", "DataDeck Studio"},
		{"/sw.js", "text/javascript", "no-cache", "addEventListener"},
		{"/icons/icon-192x192.png", "image/png", "no-cache", ""},
	}
	for _, tc := range cases {
		rec := get(t, h, tc.path)
		if rec.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200", tc.path, rec.Code)
			continue
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, tc.contentType) {
			t.Errorf("%s content-type = %q, want %q", tc.path, ct, tc.contentType)
		}
		if cache := rec.Header().Get("Cache-Control"); !strings.Contains(cache, tc.cache) {
			t.Errorf("%s cache-control = %q, want %q", tc.path, cache, tc.cache)
		}
		if tc.body != "" && !strings.Contains(rec.Body.String(), tc.body) {
			t.Errorf("%s body = %q, want %q", tc.path, rec.Body.String(), tc.body)
		}
	}
}

func TestServeRouteDocuments(t *testing.T) {
	h := newTestHandler()
	for _, target := range []string{"/docs/", "/docs"} {
		rec := get(t, h, target)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "docs route") {
			t.Errorf("%s => status %d body %q", target, rec.Code, rec.Body.String())
		}
	}
}

func TestUnknownAssetServes404Document(t *testing.T) {
	h := newTestHandler()
	rec := get(t, h, "/missing-asset.js")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not found document") {
		t.Errorf("body = %q, want the 404 document", rec.Body.String())
	}
}

func TestAPIPathsAreNeverServedAsStatic(t *testing.T) {
	h := newTestHandler()
	for _, target := range []string{"/api", "/api/", "/api/v1/unknown"} {
		rec := get(t, h, target)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s status = %d, want 404", target, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "DataDeck shell") {
			t.Errorf("%s served the SPA shell", target)
		}
		if !strings.Contains(rec.Body.String(), "NOT_FOUND") {
			t.Errorf("%s body = %q, want JSON NOT_FOUND envelope", target, rec.Body.String())
		}
	}
}

func TestPathTraversalIsBlocked(t *testing.T) {
	h := newTestHandler()
	for _, target := range []string{
		"/../go.mod",
		"/../../etc/passwd",
		"/%2e%2e/backend/go.mod",
		"/_next/../../go.mod",
	} {
		rec := get(t, h, target)
		if rec.Code == http.StatusOK {
			t.Errorf("%s was served (status 200)", target)
		}
		if strings.Contains(rec.Body.String(), "module github.com") {
			t.Errorf("%s leaked file contents", target)
		}
	}
}

func TestMissingFrontendIsExplicit(t *testing.T) {
	h := NewHandler(nil, nil)
	rec := get(t, h, "/")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "not embedded") {
		t.Errorf("body = %q, want an explicit not-embedded message", rec.Body.String())
	}
	if h.Embedded() {
		t.Error("Embedded() = true with a nil filesystem")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	h := newTestHandler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func TestHeadRequestHasNoBody(t *testing.T) {
	h := newTestHandler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodHead, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("HEAD returned %d bytes", rec.Body.Len())
	}
}

// TestEmbeddedFrontendWhenBuilt serves the real embedded assets when a release
// build has populated internal/web/dist; otherwise it documents that the
// development placeholder is present (explicit 404, never stale content).
func TestEmbeddedFrontendWhenBuilt(t *testing.T) {
	frontend, ok := Frontend()
	h := NewHandler(frontend, nil)
	if !ok {
		rec := get(t, h, "/")
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "not embedded") {
			t.Fatalf("absent frontend should 404 explicitly, got %d %q", rec.Code, rec.Body.String())
		}
		t.Log("no frontend embedded (development placeholder); explicit 404 verified")
		return
	}
	rec := get(t, h, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("embedded index status = %d, want 200", rec.Code)
	}
	var _ fs.FS = frontend
}
