// Package webui provides the internal REST API and static file server for the web UI.
package webui

import (
"encoding/json"
"io"
"log"
"net/http"
"path"
"sort"
"strconv"
"strings"
"time"

"github.com/gorilla/mux"
"github.com/linuskang/opens3/internal/storage"
)

// Handler provides the web UI API routes.
type Handler struct {
store   *storage.Backend
apiPort int
}

// NewHandler constructs a web UI handler.
// apiPort is the port on which the S3-compatible API server is listening.
func NewHandler(store *storage.Backend, apiPort int) *Handler {
return &Handler{store: store, apiPort: apiPort}
}

// Register mounts web UI API routes onto the given router.
// The static UI files are served by the caller.
func (h *Handler) Register(r *mux.Router) {
api := r.PathPrefix("/_opens3/api").Subrouter()
api.HandleFunc("/buckets", h.listBuckets).Methods(http.MethodGet)
api.HandleFunc("/buckets", h.createBucket).Methods(http.MethodPost)
api.HandleFunc("/buckets/{bucket}", h.deleteBucket).Methods(http.MethodDelete)
api.HandleFunc("/buckets/{bucket}", h.updateBucket).Methods(http.MethodPatch)
api.HandleFunc("/buckets/{bucket}/objects", h.listObjects).Methods(http.MethodGet)
api.HandleFunc("/buckets/{bucket}/objects", h.uploadObject).Methods(http.MethodPost)
api.HandleFunc("/buckets/{bucket}/objects/{key:.+}", h.deleteObject).Methods(http.MethodDelete)
api.HandleFunc("/buckets/{bucket}/objects/{key:.+}", h.getObjectMeta).Methods(http.MethodHead)
api.HandleFunc("/stats", h.stats).Methods(http.MethodGet)
api.HandleFunc("/config", h.config).Methods(http.MethodGet)
}

// ──────────────────────────────────────────────────────────────────────────────
// Response helpers
// ──────────────────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(status)
if err := json.NewEncoder(w).Encode(v); err != nil {
log.Printf("webui: json encode error: %v", err)
}
}

func writeAPIError(w http.ResponseWriter, status int, message string) {
writeJSON(w, status, map[string]string{"error": message})
}

// ──────────────────────────────────────────────────────────────────────────────
// Bucket endpoints
// ──────────────────────────────────────────────────────────────────────────────

type bucketInfo struct {
Name      string    `json:"name"`
CreatedAt time.Time `json:"created_at"`
Objects   int       `json:"objects"`
Size      int64     `json:"size"`
Public    bool      `json:"public"`
}

func (h *Handler) listBuckets(w http.ResponseWriter, r *http.Request) {
buckets, err := h.store.ListBuckets()
if err != nil {
writeAPIError(w, http.StatusInternalServerError, err.Error())
return
}

result := make([]bucketInfo, 0, len(buckets))
for _, b := range buckets {
info := bucketInfo{Name: b.Name, CreatedAt: b.CreatedAt, Public: b.Public}
// Count objects and sum sizes using a reasonable page size.
res, _ := h.store.ListObjects(b.Name, "", "", "", 10000)
if res != nil {
info.Objects = len(res.Objects)
for _, obj := range res.Objects {
info.Size += obj.Size
}
}
result = append(result, info)
}
writeJSON(w, http.StatusOK, result)
}

func (h *Handler) createBucket(w http.ResponseWriter, r *http.Request) {
var req struct {
Name string `json:"name"`
}
if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
writeAPIError(w, http.StatusBadRequest, "invalid request body; 'name' field required")
return
}
if err := h.store.CreateBucket(req.Name, "us-east-1"); err != nil {
writeAPIError(w, http.StatusConflict, err.Error())
return
}
writeJSON(w, http.StatusCreated, map[string]string{"name": req.Name})
}

func (h *Handler) deleteBucket(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]
if err := h.store.DeleteBucket(bucket); err != nil {
writeAPIError(w, http.StatusConflict, err.Error())
return
}
w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) updateBucket(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]
var req struct {
Public *bool `json:"public"`
}
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
writeAPIError(w, http.StatusBadRequest, "invalid JSON format")
return
}
if req.Public == nil {
writeAPIError(w, http.StatusBadRequest, "missing required field: public")
return
}
if err := h.store.SetBucketPublic(bucket, *req.Public); err != nil {
if s3err, ok := err.(*storage.S3Error); ok && s3err.Code == "NoSuchBucket" {
writeAPIError(w, http.StatusNotFound, err.Error())
return
}
writeAPIError(w, http.StatusInternalServerError, err.Error())
return
}
writeJSON(w, http.StatusOK, map[string]interface{}{"name": bucket, "public": *req.Public})
}

// ──────────────────────────────────────────────────────────────────────────────
// Object endpoints
// ──────────────────────────────────────────────────────────────────────────────

type objectInfo struct {
Key          string    `json:"key"`
Size         int64     `json:"size"`
ETag         string    `json:"etag"`
ContentType  string    `json:"content_type"`
LastModified time.Time `json:"last_modified"`
IsDir        bool      `json:"is_dir"`
}

func (h *Handler) listObjects(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]
q := r.URL.Query()
prefix := q.Get("prefix")
delimiter := q.Get("delimiter")
if delimiter == "" {
delimiter = "/"
}

res, err := h.store.ListObjects(bucket, prefix, "", delimiter, 10000)
if err != nil {
writeAPIError(w, http.StatusNotFound, err.Error())
return
}

result := make([]objectInfo, 0)

// Add common prefixes as "directories".
for _, cp := range res.CommonPrefixes {
result = append(result, objectInfo{
Key:   cp,
IsDir: true,
})
}

for _, obj := range res.Objects {
result = append(result, objectInfo{
Key:          obj.Key,
Size:         obj.Size,
ETag:         obj.ETag,
ContentType:  obj.ContentType,
LastModified: obj.LastModified,
})
}

sort.Slice(result, func(i, j int) bool {
if result[i].IsDir != result[j].IsDir {
return result[i].IsDir
}
return result[i].Key < result[j].Key
})

writeJSON(w, http.StatusOK, result)
}

func (h *Handler) uploadObject(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]

// Parse multipart form (max 10 GiB per file).
if err := r.ParseMultipartForm(32 << 20); err != nil {
writeAPIError(w, http.StatusBadRequest, "failed to parse multipart form: "+err.Error())
return
}

prefix := r.FormValue("prefix")

files := r.MultipartForm.File["files"]
if len(files) == 0 {
writeAPIError(w, http.StatusBadRequest, "no files provided")
return
}

type uploadResult struct {
Key  string `json:"key"`
Size int64  `json:"size"`
ETag string `json:"etag"`
}

var results []uploadResult
for _, fh := range files {
f, err := fh.Open()
if err != nil {
writeAPIError(w, http.StatusInternalServerError, "open file: "+err.Error())
return
}

key := path.Join(prefix, fh.Filename)
key = strings.TrimPrefix(key, "/")
contentType := fh.Header.Get("Content-Type")
if contentType == "" {
contentType = "application/octet-stream"
}

meta, putErr := h.store.PutObject(bucket, key, contentType, f, nil)
f.Close()
if putErr != nil {
writeAPIError(w, http.StatusInternalServerError, "put object: "+putErr.Error())
return
}
results = append(results, uploadResult{Key: meta.Key, Size: meta.Size, ETag: meta.ETag})
}

writeJSON(w, http.StatusCreated, results)
}

func (h *Handler) deleteObject(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]
key := mux.Vars(r)["key"]

if err := h.store.DeleteObject(bucket, key); err != nil {
writeAPIError(w, http.StatusInternalServerError, err.Error())
return
}
w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) getObjectMeta(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]
key := mux.Vars(r)["key"]

meta, err := h.store.HeadObject(bucket, key)
if err != nil {
writeAPIError(w, http.StatusNotFound, err.Error())
return
}
w.Header().Set("Content-Type", meta.ContentType)
w.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))
w.Header().Set("ETag", meta.ETag)
w.WriteHeader(http.StatusOK)
}

// ──────────────────────────────────────────────────────────────────────────────
// Stats endpoint
// ──────────────────────────────────────────────────────────────────────────────

type serverStats struct {
Buckets     int   `json:"buckets"`
Objects     int   `json:"objects"`
TotalSize   int64 `json:"total_size"`
Uptime      int64 `json:"uptime_seconds"`
}

var startTime = time.Now()

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
buckets, _ := h.store.ListBuckets()
var totalObjects int
var totalSize int64
for _, b := range buckets {
res, _ := h.store.ListObjects(b.Name, "", "", "", 10000)
if res != nil {
totalObjects += len(res.Objects)
for _, obj := range res.Objects {
totalSize += obj.Size
}
}
}
writeJSON(w, http.StatusOK, serverStats{
Buckets:   len(buckets),
Objects:   totalObjects,
TotalSize: totalSize,
Uptime:    int64(time.Since(startTime).Seconds()),
})
}

// ObjectDownloadHandler returns an http.Handler that streams an object to the client.
// It is meant to be mounted at /_opens3/download/{bucket}/{key:.+}
func ObjectDownloadHandler(store *storage.Backend) http.HandlerFunc {
return func(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]
key := mux.Vars(r)["key"]

meta, rc, err := store.GetObject(bucket, key)
if err != nil {
writeAPIError(w, http.StatusNotFound, err.Error())
return
}
defer rc.Close()

name := path.Base(key)
w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
w.Header().Set("Content-Type", meta.ContentType)
w.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))
w.Header().Set("ETag", meta.ETag)
w.WriteHeader(http.StatusOK)
io.Copy(w, rc) //nolint:errcheck
}
}

// ──────────────────────────────────────────────────────────────────────────────
// Config endpoint
// ──────────────────────────────────────────────────────────────────────────────

type serverConfig struct {
APIPort int `json:"api_port"`
}

func (h *Handler) config(w http.ResponseWriter, r *http.Request) {
writeJSON(w, http.StatusOK, serverConfig{APIPort: h.apiPort})
}
