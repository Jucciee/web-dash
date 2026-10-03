package static

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Handler serves the built SPA. API requests are handled before this fallback.
func Handler(directory string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		clean := filepath.Clean(filepath.FromSlash("/" + r.URL.Path))
		candidate := filepath.Join(directory, strings.TrimLeft(clean, "/\\"))
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			if strings.HasPrefix(r.URL.Path, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.ServeFile(w, r, candidate)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/assets/") || filepath.Ext(r.URL.Path) != "" {
			http.NotFound(w, r)
			return
		}
		index := filepath.Join(directory, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.Error(w, "Frontend not built. Run npm --prefix web ci and npm --prefix web run build, or use the Vite dev server.", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	})
}
