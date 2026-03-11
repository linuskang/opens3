// Package s3api implements the AWS S3-compatible HTTP API.
package s3api

import (
"encoding/xml"
"log"
"net/http"

"github.com/gorilla/mux"
"github.com/linuskang/opens3/internal/auth"
"github.com/linuskang/opens3/internal/storage"
)

// Handler holds dependencies for the S3 API.
type Handler struct {
store    *storage.Backend
verifier *auth.Verifier
region   string
}

// NewHandler constructs an S3 API handler.
func NewHandler(store *storage.Backend, verifier *auth.Verifier, region string) *Handler {
return &Handler{store: store, verifier: verifier, region: region}
}

// Register mounts all S3 API routes onto the given router.
func (h *Handler) Register(r *mux.Router) {
// Wrap all routes with auth middleware.
s3 := r.NewRoute().Subrouter()
s3.Use(h.authMiddleware)
s3.Use(corsMiddleware)

// Service-level: list all buckets.
s3.HandleFunc("/", h.listBuckets).Methods(http.MethodGet)

// Bucket-level routes — query-parameter dispatch.
s3.HandleFunc("/{bucket:[a-z0-9][a-z0-9-.]*}", h.dispatchBucket).Methods(http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodPost)

// Object-level routes.
s3.HandleFunc("/{bucket:[a-z0-9][a-z0-9-.]*}/{key:.+}", h.dispatchObject).Methods(http.MethodGet, http.MethodHead, http.MethodPut, http.MethodDelete, http.MethodPost)

// Handle OPTIONS for CORS pre-flight on all routes.
r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
writeCORSHeaders(w)
w.WriteHeader(http.StatusOK)
}).Methods(http.MethodOptions)
}

// dispatchBucket routes bucket-level requests based on HTTP method and query params.
func (h *Handler) dispatchBucket(w http.ResponseWriter, r *http.Request) {
q := r.URL.Query()

switch r.Method {
case http.MethodGet:
switch {
case q.Has("location"):
h.getBucketLocation(w, r)
case q.Has("versioning"):
h.getBucketVersioning(w, r)
case q.Has("cors"):
h.getBucketCors(w, r)
case q.Has("acl"):
h.getBucketAcl(w, r)
case q.Has("uploads"):
h.listMultipartUploads(w, r)
default:
h.listObjects(w, r)
}
case http.MethodHead:
h.headBucket(w, r)
case http.MethodPut:
switch {
case q.Has("cors"):
h.putBucketCors(w, r)
case q.Has("acl"):
h.putBucketAcl(w, r)
default:
h.createBucket(w, r)
}
case http.MethodDelete:
h.deleteBucket(w, r)
case http.MethodPost:
if q.Has("delete") {
h.deleteObjects(w, r)
} else {
writeError(w, r, http.StatusNotImplemented, "NotImplemented", "not implemented")
}
default:
writeError(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed", "method not allowed")
}
}

// dispatchObject routes object-level requests based on HTTP method and query params.
func (h *Handler) dispatchObject(w http.ResponseWriter, r *http.Request) {
q := r.URL.Query()

switch r.Method {
case http.MethodGet:
switch {
case q.Has("uploadId") && !q.Has("partNumber"):
h.listParts(w, r)
default:
h.getObject(w, r)
}
case http.MethodHead:
h.headObject(w, r)
case http.MethodPut:
switch {
case q.Has("uploadId") && q.Has("partNumber"):
h.uploadPart(w, r)
default:
h.putObject(w, r)
}
case http.MethodDelete:
if q.Has("uploadId") {
h.abortMultipartUpload(w, r)
} else {
h.deleteObject(w, r)
}
case http.MethodPost:
if q.Has("uploads") {
h.createMultipartUpload(w, r)
} else if q.Has("uploadId") {
h.completeMultipartUpload(w, r)
} else {
writeError(w, r, http.StatusNotImplemented, "NotImplemented", "not implemented")
}
default:
writeError(w, r, http.StatusMethodNotAllowed, "MethodNotAllowed", "method not allowed")
}
}

// authMiddleware verifies AWS Signature V4 on all requests.
// Browser GET / with no auth is redirected to the web UI.
// Unauthenticated GET and HEAD requests on public buckets are allowed.
func (h *Handler) authMiddleware(next http.Handler) http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// Allow unauthenticated OPTIONS (CORS pre-flight).
if r.Method == http.MethodOptions {
next.ServeHTTP(w, r)
return
}

// Redirect browsers accessing root to the web UI.
if r.URL.Path == "/" && r.Method == http.MethodGet &&
r.Header.Get("Authorization") == "" &&
r.URL.Query().Get("X-Amz-Signature") == "" {
http.Redirect(w, r, "/_opens3/", http.StatusFound)
return
}

// Allow unauthenticated read-only access to public buckets.
if (r.Method == http.MethodGet || r.Method == http.MethodHead) &&
r.Header.Get("Authorization") == "" &&
r.URL.Query().Get("X-Amz-Signature") == "" {
vars := mux.Vars(r)
if bucket := vars["bucket"]; bucket != "" && h.store.IsBucketPublic(bucket) {
next.ServeHTTP(w, r)
return
}
}

if err := h.verifier.Verify(r); err != nil {
log.Printf("auth failed for %s %s: %v", r.Method, r.URL, err)
writeError(w, r, http.StatusForbidden, "AccessDenied", "access denied: "+err.Error())
return
}
next.ServeHTTP(w, r)
})
}

// corsMiddleware adds permissive CORS headers to every response.
func corsMiddleware(next http.Handler) http.Handler {
return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
writeCORSHeaders(w)
next.ServeHTTP(w, r)
})
}

func writeCORSHeaders(w http.ResponseWriter) {
w.Header().Set("Access-Control-Allow-Origin", "*")
w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE, HEAD, OPTIONS")
w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Amz-Date, X-Amz-Content-Sha256, X-Api-Key, X-Amz-Security-Token, X-Amz-User-Agent, X-Amz-Copy-Source, X-Amz-Metadata-Directive, Range")
w.Header().Set("Access-Control-Expose-Headers", "ETag, Content-Length, Content-Type, x-amz-request-id")
}

// writeXML marshals v as XML and writes it with the given status code.
func writeXML(w http.ResponseWriter, status int, v interface{}) {
data, err := xml.MarshalIndent(v, "", "  ")
if err != nil {
w.WriteHeader(http.StatusInternalServerError)
return
}
w.Header().Set("Content-Type", "application/xml")
w.WriteHeader(status)
w.Write([]byte(xml.Header))
w.Write(data)
}

// validateBucketName checks S3 bucket naming rules.
func validateBucketName(name string) error {
if len(name) < 3 || len(name) > 63 {
return &storage.S3Error{Code: "InvalidBucketName", Message: "bucket name must be between 3 and 63 characters"}
}
for _, c := range name {
if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '.') {
return &storage.S3Error{Code: "InvalidBucketName", Message: "bucket name contains invalid characters"}
}
}
return nil
}
