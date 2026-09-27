package web

import (
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/datadeck/datadeck/backend/internal/api/response"
)

// Handler serves the embedded frontend for non-API routes.
//
// Routing follows the M6-T01 static-export contract: `/` and `/route/` resolve
// to index documents, unknown paths return the exported 404 document, and
// `/api*` is never intercepted (it always answers with the JSON envelope so a
// missing API route is a JSON 404, not the SPA shell).
type Handler struct {
	frontend    fs.FS
	hasFrontend bool
	logger      *slog.Logger
}

// NewHandler builds a static handler over frontend. A nil or incomplete FS
// yields explicit 404s rather than a panic.
func NewHandler(frontend fs.FS, logger *slog.Logger) *Handler {
	has := false
	if frontend != nil {
		if _, err := fs.Stat(frontend, "index.html"); err == nil {
			has = true
		}
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{frontend: frontend, hasFrontend: has, logger: logger}
}

// Embedded reports whether real frontend assets are available.
func (h *Handler) Embedded() bool { return h.hasFrontend }

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		response.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED",
			"method not allowed for static assets")
		return
	}

	clean := path.Clean("/" + r.URL.Path)
	if clean == "/api" || strings.HasPrefix(clean, "/api/") {
		// API routes are registered before this handler; reaching here means no
		// API route matched, so answer with the contract envelope.
		response.WriteError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}

	if !h.hasFrontend {
		response.WriteError(w, http.StatusNotFound, "NOT_FOUND",
			"frontend assets are not embedded in this build")
		return
	}

	if name, ok := h.resolve(clean); ok {
		h.serve(w, r, name, http.StatusOK)
		return
	}
	h.serve(w, r, "404.html", http.StatusNotFound)
}

// resolve maps a cleaned URL path to an embedded file name. It only ever
// returns names validated by fs.ValidPath, so traversal cannot escape the
// embedded filesystem.
func (h *Handler) resolve(clean string) (string, bool) {
	relative := strings.TrimPrefix(clean, "/")
	candidates := make([]string, 0, 3)
	switch {
	case relative == "":
		candidates = append(candidates, "index.html")
	case strings.HasSuffix(clean, "/"):
		candidates = append(candidates, relative+"index.html")
	default:
		candidates = append(candidates,
			relative,
			relative+"/index.html",
			relative+".html",
		)
	}

	for _, candidate := range candidates {
		if !fs.ValidPath(candidate) {
			continue
		}
		info, err := fs.Stat(h.frontend, candidate)
		if err != nil || info.IsDir() {
			continue
		}
		return candidate, true
	}
	return "", false
}

func (h *Handler) serve(w http.ResponseWriter, r *http.Request, name string, status int) {
	data, err := fs.ReadFile(h.frontend, name)
	if err != nil {
		h.logger.Warn("static_asset_missing", slog.String("asset", name))
		response.WriteError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}

	header := w.Header()
	header.Set("Content-Type", contentTypeFor(name))
	header.Set("X-Content-Type-Options", "nosniff")
	if strings.HasPrefix(name, "_next/static/") {
		// Content-hashed build output: safe to cache aggressively.
		header.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		// HTML, manifest, service worker and icons must be revalidated so a new
		// release is picked up (the service worker especially must not be
		// cached aggressively).
		header.Set("Cache-Control", "no-cache")
	}

	w.WriteHeader(status)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(data)
}

func contentTypeFor(name string) string {
	switch {
	case strings.HasSuffix(name, ".js"), strings.HasSuffix(name, ".mjs"):
		return "text/javascript; charset=utf-8"
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".html"):
		return "text/html; charset=utf-8"
	case strings.HasSuffix(name, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(name, ".map"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(name, ".webmanifest"):
		return "application/manifest+json"
	}
	if detected := mime.TypeByExtension(path.Ext(name)); detected != "" {
		return detected
	}
	return "application/octet-stream"
}
