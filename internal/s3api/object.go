package s3api

import (
"encoding/xml"
"io"
"net/http"
"strconv"
"strings"
"time"

"github.com/gorilla/mux"
)

// ──────────────────────────────────────────────────────────────────────────────
// PutObject  PUT /{bucket}/{key+}
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) putObject(w http.ResponseWriter, r *http.Request) {
vars := mux.Vars(r)
bucket := vars["bucket"]
key := vars["key"]

// Handle CopyObject (x-amz-copy-source header present).
if copySource := r.Header.Get("X-Amz-Copy-Source"); copySource != "" {
h.copyObject(w, r, bucket, key, copySource)
return
}

contentType := r.Header.Get("Content-Type")
if contentType == "" {
contentType = "application/octet-stream"
}

userMeta := extractUserMeta(r)

meta, err := h.store.PutObject(bucket, key, contentType, r.Body, userMeta)
if err != nil {
writeStorageError(w, r, err)
return
}

w.Header().Set("ETag", meta.ETag)
w.WriteHeader(http.StatusOK)
}

// ──────────────────────────────────────────────────────────────────────────────
// CopyObject  PUT /{bucket}/{key+}  with X-Amz-Copy-Source header
// ──────────────────────────────────────────────────────────────────────────────

type copyObjectResult struct {
XMLName      xml.Name `xml:"CopyObjectResult"`
LastModified string   `xml:"LastModified"`
ETag         string   `xml:"ETag"`
}

func (h *Handler) copyObject(w http.ResponseWriter, r *http.Request, dstBucket, dstKey, copySource string) {
// Strip leading slash and optional version ID.
copySource = strings.TrimPrefix(copySource, "/")
parts := strings.SplitN(copySource, "/", 2)
if len(parts) != 2 {
writeError(w, r, http.StatusBadRequest, "InvalidArgument", "invalid x-amz-copy-source")
return
}
srcBucket := parts[0]
srcKey := parts[1]

// Remove URL-encoded chars if needed (simple decode of %2F).
srcKey = strings.ReplaceAll(srcKey, "%2F", "/")

var userMeta map[string]string
if r.Header.Get("X-Amz-Metadata-Directive") == "REPLACE" {
userMeta = extractUserMeta(r)
}

meta, err := h.store.CopyObject(srcBucket, srcKey, dstBucket, dstKey, userMeta)
if err != nil {
writeStorageError(w, r, err)
return
}

result := copyObjectResult{
LastModified: meta.LastModified.UTC().Format(time.RFC3339),
ETag:         meta.ETag,
}
w.Header().Set("ETag", meta.ETag)
writeXML(w, http.StatusOK, result)
}

// ──────────────────────────────────────────────────────────────────────────────
// GetObject  GET /{bucket}/{key+}
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) getObject(w http.ResponseWriter, r *http.Request) {
vars := mux.Vars(r)
bucket := vars["bucket"]
key := vars["key"]

meta, rc, err := h.store.GetObject(bucket, key)
if err != nil {
writeStorageError(w, r, err)
return
}
defer rc.Close()

// Set response headers.
w.Header().Set("Content-Type", meta.ContentType)
w.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))
w.Header().Set("ETag", meta.ETag)
w.Header().Set("Last-Modified", meta.LastModified.UTC().Format(http.TimeFormat))
for k, v := range meta.UserMeta {
w.Header().Set("X-Amz-Meta-"+k, v)
}

// Handle If-None-Match / If-Modified-Since conditional requests.
if inm := r.Header.Get("If-None-Match"); inm != "" && inm == meta.ETag {
w.WriteHeader(http.StatusNotModified)
return
}

// Handle Range request.
rangeHeader := r.Header.Get("Range")
if rangeHeader != "" {
h.serveRange(w, r, rc, meta.Size, rangeHeader)
return
}

w.WriteHeader(http.StatusOK)
io.Copy(w, rc) //nolint:errcheck
}

// serveRange handles HTTP Range requests for partial content.
func (h *Handler) serveRange(w http.ResponseWriter, r *http.Request, rc io.ReadCloser, size int64, rangeHeader string) {
// Parse "bytes=start-end"
rangeHeader = strings.TrimPrefix(rangeHeader, "bytes=")
parts := strings.SplitN(rangeHeader, "-", 2)
if len(parts) != 2 {
writeError(w, r, http.StatusRequestedRangeNotSatisfiable, "InvalidRange", "invalid range")
return
}

var start, end int64
var err error

if parts[0] == "" {
// Suffix range: bytes=-N (last N bytes)
suffixLen, parseErr := strconv.ParseInt(parts[1], 10, 64)
if parseErr != nil || suffixLen <= 0 {
writeError(w, r, http.StatusRequestedRangeNotSatisfiable, "InvalidRange", "invalid range")
return
}
start = size - suffixLen
end = size - 1
} else {
start, err = strconv.ParseInt(parts[0], 10, 64)
if err != nil {
writeError(w, r, http.StatusRequestedRangeNotSatisfiable, "InvalidRange", "invalid range")
return
}
if parts[1] == "" {
end = size - 1
} else {
end, err = strconv.ParseInt(parts[1], 10, 64)
if err != nil {
writeError(w, r, http.StatusRequestedRangeNotSatisfiable, "InvalidRange", "invalid range")
return
}
}
}

if start < 0 || end >= size || start > end {
writeError(w, r, http.StatusRequestedRangeNotSatisfiable, "InvalidRange", "range not satisfiable")
return
}

// Seek to start.
if seeker, ok := rc.(io.Seeker); ok {
if _, err := seeker.Seek(start, io.SeekStart); err != nil {
writeError(w, r, http.StatusInternalServerError, "InternalError", "seek failed")
return
}
} else {
// Discard bytes up to start.
if _, err := io.CopyN(io.Discard, rc, start); err != nil {
writeError(w, r, http.StatusInternalServerError, "InternalError", "discard failed")
return
}
}

length := end - start + 1
w.Header().Set("Content-Range", "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.FormatInt(size, 10))
w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
w.WriteHeader(http.StatusPartialContent)
io.CopyN(w, rc, length) //nolint:errcheck
}

// ──────────────────────────────────────────────────────────────────────────────
// HeadObject  HEAD /{bucket}/{key+}
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) headObject(w http.ResponseWriter, r *http.Request) {
vars := mux.Vars(r)
bucket := vars["bucket"]
key := vars["key"]

meta, err := h.store.HeadObject(bucket, key)
if err != nil {
writeStorageError(w, r, err)
return
}

w.Header().Set("Content-Type", meta.ContentType)
w.Header().Set("Content-Length", strconv.FormatInt(meta.Size, 10))
w.Header().Set("ETag", meta.ETag)
w.Header().Set("Last-Modified", meta.LastModified.UTC().Format(http.TimeFormat))
for k, v := range meta.UserMeta {
w.Header().Set("X-Amz-Meta-"+k, v)
}
w.WriteHeader(http.StatusOK)
}

// ──────────────────────────────────────────────────────────────────────────────
// DeleteObject  DELETE /{bucket}/{key+}
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) deleteObject(w http.ResponseWriter, r *http.Request) {
vars := mux.Vars(r)
bucket := vars["bucket"]
key := vars["key"]

if err := h.store.DeleteObject(bucket, key); err != nil {
writeStorageError(w, r, err)
return
}
w.WriteHeader(http.StatusNoContent)
}

// ──────────────────────────────────────────────────────────────────────────────
// Helper — extract user-defined metadata from request headers.
// ──────────────────────────────────────────────────────────────────────────────

func extractUserMeta(r *http.Request) map[string]string {
meta := make(map[string]string)
for k, vals := range r.Header {
lower := strings.ToLower(k)
if strings.HasPrefix(lower, "x-amz-meta-") {
metaKey := strings.TrimPrefix(lower, "x-amz-meta-")
if len(vals) > 0 {
meta[metaKey] = vals[0]
}
}
}
if len(meta) == 0 {
return nil
}
return meta
}
