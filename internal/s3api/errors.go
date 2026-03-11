package s3api

import (
"encoding/xml"
"net/http"

"github.com/linuskang/opens3/internal/storage"
)

// errorResponse is the XML body returned for S3 errors.
type errorResponse struct {
XMLName   xml.Name `xml:"Error"`
Code      string   `xml:"Code"`
Message   string   `xml:"Message"`
Resource  string   `xml:"Resource,omitempty"`
RequestID string   `xml:"RequestId"`
}

// writeError writes an S3-compatible XML error response.
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
w.Header().Set("Content-Type", "application/xml")
w.WriteHeader(status)
resp := errorResponse{
Code:      code,
Message:   message,
Resource:  r.URL.Path,
RequestID: "opens3-req",
}
data, _ := xml.MarshalIndent(resp, "", "  ")
w.Write([]byte(xml.Header))
w.Write(data)
}

// writeStorageError maps storage errors to appropriate HTTP error responses.
func writeStorageError(w http.ResponseWriter, r *http.Request, err error) {
if s3err, ok := err.(*storage.S3Error); ok {
switch s3err.Code {
case "NoSuchBucket":
writeError(w, r, http.StatusNotFound, s3err.Code, s3err.Message)
case "NoSuchKey":
writeError(w, r, http.StatusNotFound, s3err.Code, s3err.Message)
case "BucketNotEmpty":
writeError(w, r, http.StatusConflict, s3err.Code, s3err.Message)
case "BucketAlreadyOwnedByYou":
writeError(w, r, http.StatusConflict, s3err.Code, s3err.Message)
case "NoSuchUpload":
writeError(w, r, http.StatusNotFound, s3err.Code, s3err.Message)
default:
writeError(w, r, http.StatusInternalServerError, s3err.Code, s3err.Message)
}
return
}
writeError(w, r, http.StatusInternalServerError, "InternalError", err.Error())
}
