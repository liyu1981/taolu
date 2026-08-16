// Package web serves the read-only browser UI for the taolu vault alongside the
// MCP server. It exposes a small JSON API that reuses pkg/vault directly and
// serves the embedded single-page app.
package web

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"
)

//go:embed all:dist
var distFS embed.FS

// NewHandler builds the HTTP handler for the web UI: the /api/* JSON routes
// backed by the vault at vaultPath, plus the embedded SPA served from /.
func NewHandler(vaultPath string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/status", handleStatus(vaultPath))
	mux.HandleFunc("GET /api/taolus", handleTaolus(vaultPath))
	mux.HandleFunc("GET /api/taolus/{name}", handleTaolu(vaultPath))
	mux.HandleFunc("GET /api/taolus/{name}/history", handleHistory(vaultPath))
	mux.HandleFunc("GET /api/taolus/{name}/content", handleContent(vaultPath))
	mux.HandleFunc("GET /api/taolus/{name}/diff", handleDiff(vaultPath))

	mux.Handle("/", spaHandler())

	return mux
}

// spaHandler serves the embedded SPA. index.html is served without caching so a
// stale shell is never used; hashed assets are long-cacheable.
func spaHandler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("web: embedded dist missing: " + err.Error())
	}
	assets := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(sub, p); err != nil {
			// SPA fallback: any unknown path serves the shell.
			r.URL.Path = "/"
		}
		if strings.HasSuffix(r.URL.Path, "index.html") || r.URL.Path == "/" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		assets.ServeHTTP(w, r)
	})
}

// jsonOK writes a JSON response with a 200 status.
func jsonOK(w http.ResponseWriter, v any) {
	writeJSON(w, http.StatusOK, v)
}

// writeJSON marshals v as JSON. Encoding failures are logged and dropped since
// the response headers have already been written in most cases.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := jsonEncode(w, v); err != nil {
		log.Printf("web: encode response: %v", err)
	}
}

// apiError writes a JSON error body with the given status.
func apiError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

var startTime = time.Now()
