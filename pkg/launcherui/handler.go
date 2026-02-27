package launcherui

import (
	"embed"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"
)

//go:embed all:dist
var embeddedDistFS embed.FS

// Handler serves embedded launcher UI assets with SPA fallback.
func Handler() http.Handler {
	distFS, err := fs.Sub(embeddedDistFS, "dist")
	if err != nil {
		return http.NotFoundHandler()
	}
	return newSPAHandler(distFS)
}

func newSPAHandler(distFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		cleanPath := path.Clean("/" + strings.TrimSpace(r.URL.Path))
		if cleanPath == "." {
			cleanPath = "/"
		}
		candidate := strings.TrimPrefix(cleanPath, "/")
		if candidate == "" {
			candidate = "index.html"
		}

		if fileExists(distFS, candidate) {
			fileServer.ServeHTTP(w, r)
			return
		}

		if path.Ext(candidate) != "" {
			http.NotFound(w, r)
			return
		}

		indexBytes, err := fs.ReadFile(distFS, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(indexBytes)
	})
}

func fileExists(distFS fs.FS, name string) bool {
	info, err := fs.Stat(distFS, name)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func cloneURL(in *url.URL) *url.URL {
	if in == nil {
		return &url.URL{Path: "/"}
	}
	copyURL := *in
	return &copyURL
}
