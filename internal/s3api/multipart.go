package s3api

import (
"encoding/xml"
"net/http"
"sort"
"strconv"
"time"

"github.com/gorilla/mux"
"github.com/linuskang/opens3/internal/storage"
)

// ──────────────────────────────────────────────────────────────────────────────
// CreateMultipartUpload  POST /{bucket}/{key+}?uploads
// ──────────────────────────────────────────────────────────────────────────────

type initiateMultipartUploadResult struct {
XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
Xmlns    string   `xml:"xmlns,attr"`
Bucket   string   `xml:"Bucket"`
Key      string   `xml:"Key"`
UploadID string   `xml:"UploadId"`
}

func (h *Handler) createMultipartUpload(w http.ResponseWriter, r *http.Request) {
vars := mux.Vars(r)
bucket := vars["bucket"]
key := vars["key"]

contentType := r.Header.Get("Content-Type")
if contentType == "" {
contentType = "application/octet-stream"
}
userMeta := extractUserMeta(r)

uploadID, err := h.store.CreateMultipartUpload(bucket, key, contentType, userMeta)
if err != nil {
writeStorageError(w, r, err)
return
}

result := initiateMultipartUploadResult{
Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
Bucket:   bucket,
Key:      key,
UploadID: uploadID,
}
writeXML(w, http.StatusOK, result)
}

// ──────────────────────────────────────────────────────────────────────────────
// UploadPart  PUT /{bucket}/{key+}?partNumber=N&uploadId=X
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) uploadPart(w http.ResponseWriter, r *http.Request) {
vars := mux.Vars(r)
bucket := vars["bucket"]
q := r.URL.Query()

uploadID := q.Get("uploadId")
partNumStr := q.Get("partNumber")
partNum, err := strconv.Atoi(partNumStr)
if err != nil || partNum < 1 || partNum > 10000 {
writeError(w, r, http.StatusBadRequest, "InvalidArgument", "invalid part number")
return
}

etag, err := h.store.UploadPart(uploadID, partNum, r.Body)
if err != nil {
writeStorageError(w, r, err)
return
}

w.Header().Set("ETag", etag)
w.Header().Set("x-amz-multipart-upload-bucket", bucket)
w.WriteHeader(http.StatusOK)
}

// ──────────────────────────────────────────────────────────────────────────────
// CompleteMultipartUpload  POST /{bucket}/{key+}?uploadId=X
// ──────────────────────────────────────────────────────────────────────────────

type completeMultipartUploadRequest struct {
XMLName xml.Name     `xml:"CompleteMultipartUpload"`
Parts   []partEntry  `xml:"Part"`
}

type partEntry struct {
PartNumber int    `xml:"PartNumber"`
ETag       string `xml:"ETag"`
}

type completeMultipartUploadResult struct {
XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
Xmlns    string   `xml:"xmlns,attr"`
Location string   `xml:"Location"`
Bucket   string   `xml:"Bucket"`
Key      string   `xml:"Key"`
ETag     string   `xml:"ETag"`
}

func (h *Handler) completeMultipartUpload(w http.ResponseWriter, r *http.Request) {
vars := mux.Vars(r)
bucket := vars["bucket"]
key := vars["key"]
uploadID := r.URL.Query().Get("uploadId")

var req completeMultipartUploadRequest
if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
writeError(w, r, http.StatusBadRequest, "MalformedXML", "malformed XML")
return
}

parts := make([]storage.Part, len(req.Parts))
for i, p := range req.Parts {
parts[i] = storage.Part{
PartNumber: p.PartNumber,
ETag:       p.ETag,
}
}
sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })

meta, err := h.store.CompleteMultipartUpload(uploadID, parts)
if err != nil {
writeStorageError(w, r, err)
return
}

result := completeMultipartUploadResult{
Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
Location: "/" + bucket + "/" + key,
Bucket:   bucket,
Key:      key,
ETag:     meta.ETag,
}
writeXML(w, http.StatusOK, result)
}

// ──────────────────────────────────────────────────────────────────────────────
// AbortMultipartUpload  DELETE /{bucket}/{key+}?uploadId=X
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) abortMultipartUpload(w http.ResponseWriter, r *http.Request) {
uploadID := r.URL.Query().Get("uploadId")

if err := h.store.AbortMultipartUpload(uploadID); err != nil {
writeStorageError(w, r, err)
return
}
w.WriteHeader(http.StatusNoContent)
}

// ──────────────────────────────────────────────────────────────────────────────
// ListParts  GET /{bucket}/{key+}?uploadId=X
// ──────────────────────────────────────────────────────────────────────────────

type listPartsResult struct {
XMLName              xml.Name    `xml:"ListPartsResult"`
Xmlns                string      `xml:"xmlns,attr"`
Bucket               string      `xml:"Bucket"`
Key                  string      `xml:"Key"`
UploadID             string      `xml:"UploadId"`
StorageClass         string      `xml:"StorageClass"`
Initiator            owner       `xml:"Initiator"`
Owner                owner       `xml:"Owner"`
MaxParts             int         `xml:"MaxParts"`
IsTruncated          bool        `xml:"IsTruncated"`
Parts                []partResult `xml:"Part"`
}

type partResult struct {
PartNumber   int    `xml:"PartNumber"`
LastModified string `xml:"LastModified"`
ETag         string `xml:"ETag"`
Size         int64  `xml:"Size"`
}

func (h *Handler) listParts(w http.ResponseWriter, r *http.Request) {
vars := mux.Vars(r)
bucket := vars["bucket"]
key := vars["key"]
uploadID := r.URL.Query().Get("uploadId")

parts, meta, err := h.store.ListParts(uploadID)
if err != nil {
writeStorageError(w, r, err)
return
}

var partResults []partResult
for _, p := range parts {
partResults = append(partResults, partResult{
PartNumber:   p.PartNumber,
LastModified: time.Now().UTC().Format(time.RFC3339),
ETag:         p.ETag,
Size:         p.Size,
})
}

_ = meta
result := listPartsResult{
Xmlns:        "http://s3.amazonaws.com/doc/2006-03-01/",
Bucket:       bucket,
Key:          key,
UploadID:     uploadID,
StorageClass: "STANDARD",
Initiator:    owner{ID: "opens3", DisplayName: "opens3"},
Owner:        owner{ID: "opens3", DisplayName: "opens3"},
MaxParts:     1000,
Parts:        partResults,
}
writeXML(w, http.StatusOK, result)
}

// ──────────────────────────────────────────────────────────────────────────────
// ListMultipartUploads  GET /{bucket}?uploads
// ──────────────────────────────────────────────────────────────────────────────

type listMultipartUploadsResult struct {
XMLName  xml.Name        `xml:"ListMultipartUploadsResult"`
Xmlns    string          `xml:"xmlns,attr"`
Bucket   string          `xml:"Bucket"`
Uploads  []uploadEntry   `xml:"Upload"`
}

type uploadEntry struct {
Key          string `xml:"Key"`
UploadID     string `xml:"UploadId"`
StorageClass string `xml:"StorageClass"`
Initiated    string `xml:"Initiated"`
Owner        owner  `xml:"Owner"`
Initiator    owner  `xml:"Initiator"`
}

func (h *Handler) listMultipartUploads(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]

uploads, err := h.store.ListMultipartUploads(bucket)
if err != nil {
writeStorageError(w, r, err)
return
}

var entries []uploadEntry
for _, u := range uploads {
entries = append(entries, uploadEntry{
Key:          u.Key,
UploadID:     u.UploadID,
StorageClass: "STANDARD",
Initiated:    u.InitiatedAt.UTC().Format(time.RFC3339),
Owner:        owner{ID: "opens3", DisplayName: "opens3"},
Initiator:    owner{ID: "opens3", DisplayName: "opens3"},
})
}

result := listMultipartUploadsResult{
Xmlns:   "http://s3.amazonaws.com/doc/2006-03-01/",
Bucket:  bucket,
Uploads: entries,
}
writeXML(w, http.StatusOK, result)
}
