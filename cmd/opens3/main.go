// Opens3 — a lightweight, S3-compatible object storage server with a web UI.
package main

import (
"embed"
"fmt"
"io/fs"
"log"
"net/http"
"strings"

"github.com/gorilla/mux"
"github.com/linuskang/opens3/internal/auth"
"github.com/linuskang/opens3/internal/config"
"github.com/linuskang/opens3/internal/s3api"
"github.com/linuskang/opens3/internal/storage"
"github.com/linuskang/opens3/internal/webui"
)

//go:embed ui/dist
var uiFS embed.FS

func main() {
cfg := config.Load()

log.Printf("opens3 starting on :%d", cfg.Port)
log.Printf("  data dir  : %s", cfg.DataDir)
log.Printf("  region    : %s", cfg.Region)
log.Printf("  access key: %s", cfg.AccessKey)
if cfg.UIEnabled {
log.Printf("  web UI    : http://localhost:%d/_opens3/", cfg.Port)
}

// Initialise storage backend.
store, err := storage.NewBackend(cfg.DataDir)
if err != nil {
log.Fatalf("storage init: %v", err)
}

// Build authentication verifier.
verifier := auth.NewVerifier(cfg.AccessKey, cfg.SecretKey, cfg.Region)

r := mux.NewRouter()
r.Use(recoveryMiddleware)

// ── S3 API routes (registered first — highest priority) ───────────────────
s3Handler := s3api.NewHandler(store, verifier, cfg.Region)
s3Handler.Register(r)

// ── Web UI routes ─────────────────────────────────────────────────────────
if cfg.UIEnabled {
uiHandler := webui.NewHandler(store)
uiHandler.Register(r)

// Object download endpoint.
r.PathPrefix("/_opens3/download/").Handler(
http.StripPrefix("/_opens3/download", downloadRouter(store)),
)

// Serve embedded static UI files.
distFS, fsErr := fs.Sub(uiFS, "ui/dist")
if fsErr != nil {
log.Fatalf("embed FS sub: %v", fsErr)
}

// The SPA handler: serve static assets; fall back to index.html for
// unknown paths (client-side routing).
r.PathPrefix("/_opens3/").HandlerFunc(spaHandler(distFS))
}

addr := fmt.Sprintf(":%d", cfg.Port)
srv := &http.Server{
Addr:    addr,
Handler: r,
}

log.Printf("listening on %s", addr)
if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
log.Fatalf("server error: %v", err)
}
}

// spaHandler returns a handler that serves files from distFS under /_opens3/,
// falling back to index.html for unknown paths to support client-side routing.
func spaHandler(distFS fs.FS) http.HandlerFunc {
return func(w http.ResponseWriter, r *http.Request) {
// Strip the /_opens3 prefix to get the asset path.
assetPath := strings.TrimPrefix(r.URL.Path, "/_opens3")
if assetPath == "" || assetPath == "/" {
assetPath = "/index.html"
}
// Remove leading slash for fs.FS.
fsPath := strings.TrimPrefix(assetPath, "/")

// Check if the file exists in the embedded FS.
f, err := distFS.Open(fsPath)
if err != nil {
// Fall back to index.html for SPA routing.
fsPath = "index.html"
} else {
f.Close()
}

http.ServeFileFS(w, r, distFS, fsPath)
}
}

// downloadRouter builds a handler for /{bucket}/{key:.+} download URLs.
func downloadRouter(store *storage.Backend) http.Handler {
sub := mux.NewRouter()
sub.Handle("/{bucket}/{key:.+}", webui.ObjectDownloadHandler(store)).Methods(http.MethodGet)
return sub
}

// recoveryMiddleware catches panics and returns a 500 error.
func recoveryMiddleware(next http.Handler) http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
defer func() {
if rec := recover(); rec != nil {
log.Printf("PANIC: %v", rec)
http.Error(w, "internal server error", http.StatusInternalServerError)
}
}()
next.ServeHTTP(w, r)
})
}
