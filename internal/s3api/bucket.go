package s3api

import (
"encoding/xml"
"net/http"
"strconv"
"time"

"github.com/gorilla/mux"
"github.com/linuskang/opens3/internal/storage"
)

// ──────────────────────────────────────────────────────────────────────────────
// ListBuckets  GET /
// ──────────────────────────────────────────────────────────────────────────────

type listBucketsResult struct {
XMLName xml.Name       `xml:"ListAllMyBucketsResult"`
Owner   owner          `xml:"Owner"`
Buckets []bucketEntry  `xml:"Buckets>Bucket"`
}

type owner struct {
ID          string `xml:"ID"`
DisplayName string `xml:"DisplayName"`
}

type bucketEntry struct {
Name         string `xml:"Name"`
CreationDate string `xml:"CreationDate"`
}

func (h *Handler) listBuckets(w http.ResponseWriter, r *http.Request) {
buckets, err := h.store.ListBuckets()
if err != nil {
writeStorageError(w, r, err)
return
}

result := listBucketsResult{
Owner: owner{ID: "opens3", DisplayName: "opens3"},
}
for _, b := range buckets {
result.Buckets = append(result.Buckets, bucketEntry{
Name:         b.Name,
CreationDate: b.CreatedAt.UTC().Format(time.RFC3339),
})
}

writeXML(w, http.StatusOK, result)
}

// ──────────────────────────────────────────────────────────────────────────────
// CreateBucket  PUT /{bucket}
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) createBucket(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]
if err := validateBucketName(bucket); err != nil {
writeError(w, r, http.StatusBadRequest, "InvalidBucketName", err.Error())
return
}

if err := h.store.CreateBucket(bucket, h.region); err != nil {
writeStorageError(w, r, err)
return
}

w.Header().Set("Location", "/"+bucket)
w.WriteHeader(http.StatusOK)
}

// ──────────────────────────────────────────────────────────────────────────────
// HeadBucket  HEAD /{bucket}
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) headBucket(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]
if _, err := h.store.HeadBucket(bucket); err != nil {
writeStorageError(w, r, err)
return
}
w.WriteHeader(http.StatusOK)
}

// ──────────────────────────────────────────────────────────────────────────────
// DeleteBucket  DELETE /{bucket}
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) deleteBucket(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]
if err := h.store.DeleteBucket(bucket); err != nil {
writeStorageError(w, r, err)
return
}
w.WriteHeader(http.StatusNoContent)
}

// ──────────────────────────────────────────────────────────────────────────────
// GetBucketLocation  GET /{bucket}?location
// ──────────────────────────────────────────────────────────────────────────────

type locationResult struct {
XMLName            xml.Name `xml:"LocationConstraint"`
Xmlns              string   `xml:"xmlns,attr"`
LocationConstraint string   `xml:",chardata"`
}

func (h *Handler) getBucketLocation(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]
if _, err := h.store.HeadBucket(bucket); err != nil {
writeStorageError(w, r, err)
return
}
result := locationResult{
Xmlns:              "http://s3.amazonaws.com/doc/2006-03-01/",
LocationConstraint: h.region,
}
writeXML(w, http.StatusOK, result)
}

// ──────────────────────────────────────────────────────────────────────────────
// GetBucketVersioning  GET /{bucket}?versioning   (stub — always Disabled)
// ──────────────────────────────────────────────────────────────────────────────

type versioningResult struct {
XMLName xml.Name `xml:"VersioningConfiguration"`
Xmlns   string   `xml:"xmlns,attr"`
Status  string   `xml:"Status,omitempty"`
}

func (h *Handler) getBucketVersioning(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]
if _, err := h.store.HeadBucket(bucket); err != nil {
writeStorageError(w, r, err)
return
}
writeXML(w, http.StatusOK, versioningResult{Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/"})
}

// ──────────────────────────────────────────────────────────────────────────────
// ListObjects  GET /{bucket}  (v1 and v2)
// ──────────────────────────────────────────────────────────────────────────────

type listObjectsV1Result struct {
XMLName        xml.Name            `xml:"ListBucketResult"`
Xmlns          string              `xml:"xmlns,attr"`
Name           string              `xml:"Name"`
Prefix         string              `xml:"Prefix"`
Marker         string              `xml:"Marker"`
NextMarker     string              `xml:"NextMarker,omitempty"`
MaxKeys        int                 `xml:"MaxKeys"`
Delimiter      string              `xml:"Delimiter,omitempty"`
IsTruncated    bool                `xml:"IsTruncated"`
Contents       []objectContent     `xml:"Contents"`
CommonPrefixes []commonPrefixEntry `xml:"CommonPrefixes"`
}

type listObjectsV2Result struct {
XMLName               xml.Name            `xml:"ListBucketResult"`
Xmlns                 string              `xml:"xmlns,attr"`
Name                  string              `xml:"Name"`
Prefix                string              `xml:"Prefix"`
ContinuationToken     string              `xml:"ContinuationToken,omitempty"`
NextContinuationToken string              `xml:"NextContinuationToken,omitempty"`
KeyCount              int                 `xml:"KeyCount"`
MaxKeys               int                 `xml:"MaxKeys"`
Delimiter             string              `xml:"Delimiter,omitempty"`
IsTruncated           bool                `xml:"IsTruncated"`
Contents              []objectContent     `xml:"Contents"`
CommonPrefixes        []commonPrefixEntry `xml:"CommonPrefixes"`
}

type objectContent struct {
Key          string `xml:"Key"`
LastModified string `xml:"LastModified"`
ETag         string `xml:"ETag"`
Size         int64  `xml:"Size"`
StorageClass string `xml:"StorageClass"`
Owner        owner  `xml:"Owner"`
}

type commonPrefixEntry struct {
Prefix string `xml:"Prefix"`
}

func (h *Handler) listObjects(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]
q := r.URL.Query()
isV2 := q.Get("list-type") == "2"

prefix := q.Get("prefix")
delimiter := q.Get("delimiter")
maxKeys := 1000
if mk := q.Get("max-keys"); mk != "" {
if v, err := strconv.Atoi(mk); err == nil && v >= 0 {
maxKeys = v
}
}

var marker string
if isV2 {
marker = q.Get("continuation-token")
if marker == "" {
marker = q.Get("start-after")
}
} else {
marker = q.Get("marker")
}

res, err := h.store.ListObjects(bucket, prefix, marker, delimiter, maxKeys)
if err != nil {
writeStorageError(w, r, err)
return
}

var contents []objectContent
for _, obj := range res.Objects {
contents = append(contents, objectContent{
Key:          obj.Key,
LastModified: obj.LastModified.UTC().Format(time.RFC3339),
ETag:         obj.ETag,
Size:         obj.Size,
StorageClass: "STANDARD",
Owner:        owner{ID: "opens3", DisplayName: "opens3"},
})
}

var prefixes []commonPrefixEntry
for _, cp := range res.CommonPrefixes {
prefixes = append(prefixes, commonPrefixEntry{Prefix: cp})
}

if isV2 {
result := listObjectsV2Result{
Xmlns:          "http://s3.amazonaws.com/doc/2006-03-01/",
Name:           bucket,
Prefix:         prefix,
MaxKeys:        maxKeys,
Delimiter:      delimiter,
IsTruncated:    res.IsTruncated,
KeyCount:       len(contents) + len(prefixes),
Contents:       contents,
CommonPrefixes: prefixes,
}
if res.IsTruncated {
result.NextContinuationToken = res.NextMarker
}
if ct := q.Get("continuation-token"); ct != "" {
result.ContinuationToken = ct
}
writeXML(w, http.StatusOK, result)
} else {
result := listObjectsV1Result{
Xmlns:          "http://s3.amazonaws.com/doc/2006-03-01/",
Name:           bucket,
Prefix:         prefix,
Marker:         marker,
MaxKeys:        maxKeys,
Delimiter:      delimiter,
IsTruncated:    res.IsTruncated,
NextMarker:     res.NextMarker,
Contents:       contents,
CommonPrefixes: prefixes,
}
writeXML(w, http.StatusOK, result)
}
}

// ──────────────────────────────────────────────────────────────────────────────
// DeleteObjects  POST /{bucket}?delete
// ──────────────────────────────────────────────────────────────────────────────

type deleteObjectsRequest struct {
XMLName xml.Name      `xml:"Delete"`
Objects []deleteEntry `xml:"Object"`
Quiet   bool          `xml:"Quiet"`
}

type deleteEntry struct {
Key       string `xml:"Key"`
VersionID string `xml:"VersionId,omitempty"`
}

type deleteObjectsResult struct {
XMLName xml.Name        `xml:"DeleteResult"`
Xmlns   string          `xml:"xmlns,attr"`
Deleted []deletedEntry  `xml:"Deleted,omitempty"`
Errors  []deleteErrEntry `xml:"Error,omitempty"`
}

type deletedEntry struct {
Key string `xml:"Key"`
}

type deleteErrEntry struct {
Key     string `xml:"Key"`
Code    string `xml:"Code"`
Message string `xml:"Message"`
}

func (h *Handler) deleteObjects(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]

var req deleteObjectsRequest
if err := xml.NewDecoder(r.Body).Decode(&req); err != nil {
writeError(w, r, http.StatusBadRequest, "MalformedXML", "malformed XML in request body")
return
}

result := deleteObjectsResult{Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/"}

for _, obj := range req.Objects {
if err := h.store.DeleteObject(bucket, obj.Key); err != nil {
if s3err, ok := err.(*storage.S3Error); ok {
result.Errors = append(result.Errors, deleteErrEntry{
Key:     obj.Key,
Code:    s3err.Code,
Message: s3err.Message,
})
continue
}
}
if !req.Quiet {
result.Deleted = append(result.Deleted, deletedEntry{Key: obj.Key})
}
}

writeXML(w, http.StatusOK, result)
}

// ──────────────────────────────────────────────────────────────────────────────
// GetBucketCors  GET /{bucket}?cors  (stub)
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) getBucketCors(w http.ResponseWriter, r *http.Request) {
writeError(w, r, http.StatusNotFound, "NoSuchCORSConfiguration", "The CORS configuration does not exist")
}

// ──────────────────────────────────────────────────────────────────────────────
// PutBucketCors  PUT /{bucket}?cors
// ──────────────────────────────────────────────────────────────────────────────

func (h *Handler) putBucketCors(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusOK)
}

// ──────────────────────────────────────────────────────────────────────────────
// GetBucketAcl  GET /{bucket}?acl
// ──────────────────────────────────────────────────────────────────────────────

type aclResult struct {
XMLName           xml.Name    `xml:"AccessControlPolicy"`
Xmlns             string      `xml:"xmlns,attr"`
Owner             owner       `xml:"Owner"`
AccessControlList []aclGrant  `xml:"AccessControlList>Grant"`
}

type aclGrant struct {
Grantee    aclGrantee `xml:"Grantee"`
Permission string     `xml:"Permission"`
}

type aclGrantee struct {
Xmlns string `xml:"xmlns:xsi,attr"`
Type  string `xml:"xsi:type,attr"`
URI   string `xml:"URI,omitempty"`
ID    string `xml:"ID,omitempty"`
}

func (h *Handler) getBucketAcl(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]
meta, err := h.store.HeadBucket(bucket)
if err != nil {
writeStorageError(w, r, err)
return
}

result := aclResult{
Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
Owner: owner{ID: "opens3", DisplayName: "opens3"},
AccessControlList: []aclGrant{
{
Grantee: aclGrantee{
Xmlns: "http://www.w3.org/2001/XMLSchema-instance",
Type:  "CanonicalUser",
ID:    "opens3",
},
Permission: "FULL_CONTROL",
},
},
}

if meta.Public {
result.AccessControlList = append(result.AccessControlList, aclGrant{
Grantee: aclGrantee{
Xmlns: "http://www.w3.org/2001/XMLSchema-instance",
Type:  "Group",
URI:   "http://acs.amazonaws.com/groups/global/AllUsers",
},
Permission: "READ",
})
}

writeXML(w, http.StatusOK, result)
}

// ──────────────────────────────────────────────────────────────────────────────
// PutBucketAcl  PUT /{bucket}?acl
// ──────────────────────────────────────────────────────────────────────────────

type putAclRequest struct {
XMLName           xml.Name    `xml:"AccessControlPolicy"`
AccessControlList []aclGrant  `xml:"AccessControlList>Grant"`
}

func (h *Handler) putBucketAcl(w http.ResponseWriter, r *http.Request) {
bucket := mux.Vars(r)["bucket"]

// Support canned ACL via header (x-amz-acl: public-read or private).
cannedACL := r.Header.Get("x-amz-acl")
if cannedACL == "" {
cannedACL = r.Header.Get("X-Amz-Acl")
}

var public bool
switch cannedACL {
case "public-read", "public-read-write":
public = true
case "private", "authenticated-read":
public = false
default:
// Parse XML body if no canned ACL header.
var req putAclRequest
if err := xml.NewDecoder(r.Body).Decode(&req); err == nil {
for _, grant := range req.AccessControlList {
if grant.Grantee.URI == "http://acs.amazonaws.com/groups/global/AllUsers" &&
(grant.Permission == "READ" || grant.Permission == "FULL_CONTROL") {
public = true
break
}
}
}
}

if err := h.store.SetBucketPublic(bucket, public); err != nil {
writeStorageError(w, r, err)
return
}
w.WriteHeader(http.StatusOK)
}
