package httpapi

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// spaFileServer serves files from fsys and falls back to index.html for paths
// that are not /api/* and do not match an embedded asset (SPA + /<slug>).
func spaFileServer(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		reqPath := path.Clean("/" + r.URL.Path)
		if reqPath == "/" {
			serveFile(w, r, fsys, "index.html")
			return
		}

		rel := strings.TrimPrefix(reqPath, "/")
		if fileExists(fsys, rel) {
			serveFile(w, r, fsys, rel)
			return
		}

		serveFile(w, r, fsys, "index.html")
	})
}

func fileExists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func serveFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) {
	f, err := fsys.Open(name)
	if err != nil {
		handleNotFound(w, r)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		handleNotFound(w, r)
		return
	}
	if stat.IsDir() {
		handleNotFound(w, r)
		return
	}

	rs, ok := f.(io.ReadSeeker)
	if !ok {
		// embed.FS files are ReadSeekers; fall back to buffering if not.
		data, err := io.ReadAll(f)
		if err != nil {
			handleNotFound(w, r)
			return
		}
		http.ServeContent(w, r, path.Base(name), stat.ModTime(), bytes.NewReader(data))
		return
	}
	http.ServeContent(w, r, path.Base(name), stat.ModTime(), rs)
}
