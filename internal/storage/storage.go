// Package storage provides a filesystem-based S3 storage backend.
package storage

import (
"crypto/sha256"
"encoding/hex"
"encoding/json"
"fmt"
"io"
"os"
"path/filepath"
"sort"
"strings"
"sync"
"time"

"github.com/google/uuid"
)

// BucketMeta holds metadata for a bucket.
type BucketMeta struct {
Name      string    `json:"name"`
CreatedAt time.Time `json:"created_at"`
Region    string    `json:"region"`
Public    bool      `json:"public"`
}

// ObjectMeta holds metadata for a stored object.
type ObjectMeta struct {
Key          string            `json:"key"`
Size         int64             `json:"size"`
ETag         string            `json:"etag"`
ContentType  string            `json:"content_type"`
LastModified time.Time         `json:"last_modified"`
UserMeta     map[string]string `json:"user_meta,omitempty"`
}

// MultipartMeta holds state for an in-progress multipart upload.
type MultipartMeta struct {
UploadID    string            `json:"upload_id"`
Bucket      string            `json:"bucket"`
Key         string            `json:"key"`
ContentType string            `json:"content_type"`
UserMeta    map[string]string `json:"user_meta,omitempty"`
InitiatedAt time.Time         `json:"initiated_at"`
}

// Part represents an uploaded part.
type Part struct {
PartNumber int    `json:"part_number"`
ETag       string `json:"etag"`
Size       int64  `json:"size"`
}

// Backend is the filesystem-based storage backend.
type Backend struct {
root string
mu   sync.RWMutex
}

// NewBackend creates a new Backend rooted at dataDir.
func NewBackend(dataDir string) (*Backend, error) {
b := &Backend{root: dataDir}
for _, d := range []string{
filepath.Join(dataDir, "buckets"),
filepath.Join(dataDir, "multipart"),
} {
if err := os.MkdirAll(d, 0o750); err != nil {
return nil, fmt.Errorf("create data dir %s: %w", d, err)
}
}
return b, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Bucket operations
// ──────────────────────────────────────────────────────────────────────────────

func (b *Backend) bucketDir(name string) string {
return filepath.Join(b.root, "buckets", name)
}

func (b *Backend) objectsDir(bucket string) string {
return filepath.Join(b.bucketDir(bucket), "objects")
}

func (b *Backend) metaDir(bucket string) string {
return filepath.Join(b.bucketDir(bucket), "meta")
}

func (b *Backend) bucketMetaFile(name string) string {
return filepath.Join(b.bucketDir(name), ".bucket")
}

// CreateBucket creates a new bucket.
func (b *Backend) CreateBucket(name, region string) error {
b.mu.Lock()
defer b.mu.Unlock()

dir := b.bucketDir(name)
if _, err := os.Stat(dir); err == nil {
return ErrBucketExists
}
if err := os.MkdirAll(b.objectsDir(name), 0o750); err != nil {
return fmt.Errorf("create objects dir: %w", err)
}
if err := os.MkdirAll(b.metaDir(name), 0o750); err != nil {
return fmt.Errorf("create meta dir: %w", err)
}

meta := BucketMeta{Name: name, CreatedAt: time.Now().UTC(), Region: region}
return writeMeta(b.bucketMetaFile(name), &meta)
}

// DeleteBucket removes a bucket (must be empty of objects).
func (b *Backend) DeleteBucket(name string) error {
b.mu.Lock()
defer b.mu.Unlock()

dir := b.bucketDir(name)
if _, err := os.Stat(dir); os.IsNotExist(err) {
return ErrNoSuchBucket
}

// Walk recursively; bucket is empty only when there are no object files.
objDir := b.objectsDir(name)
hasObjects := false
_ = filepath.WalkDir(objDir, func(path string, d os.DirEntry, err error) error {
if err != nil {
return err
}
if !d.IsDir() {
hasObjects = true
return filepath.SkipAll
}
return nil
})
if hasObjects {
return ErrBucketNotEmpty
}

return os.RemoveAll(dir)
}

// HeadBucket returns bucket metadata or an error if it doesn't exist.
func (b *Backend) HeadBucket(name string) (*BucketMeta, error) {
b.mu.RLock()
defer b.mu.RUnlock()
return b.readBucketMeta(name)
}

// SetBucketPublic sets or clears the public-read flag for a bucket.
func (b *Backend) SetBucketPublic(name string, public bool) error {
b.mu.Lock()
defer b.mu.Unlock()

meta, err := b.readBucketMeta(name)
if err != nil {
return err
}
meta.Public = public
return writeMeta(b.bucketMetaFile(name), meta)
}

// IsBucketPublic reports whether the bucket exists and is public.
// It returns false (not an error) when the bucket does not exist.
func (b *Backend) IsBucketPublic(name string) bool {
b.mu.RLock()
defer b.mu.RUnlock()
meta, err := b.readBucketMeta(name)
if err != nil {
return false
}
return meta.Public
}

// ListBuckets returns all buckets.
func (b *Backend) ListBuckets() ([]BucketMeta, error) {
b.mu.RLock()
defer b.mu.RUnlock()

bucketsDir := filepath.Join(b.root, "buckets")
entries, err := os.ReadDir(bucketsDir)
if err != nil {
return nil, fmt.Errorf("read buckets dir: %w", err)
}

var buckets []BucketMeta
for _, e := range entries {
if !e.IsDir() {
continue
}
meta, err := b.readBucketMeta(e.Name())
if err != nil {
continue
}
buckets = append(buckets, *meta)
}
sort.Slice(buckets, func(i, j int) bool { return buckets[i].Name < buckets[j].Name })
return buckets, nil
}

func (b *Backend) readBucketMeta(name string) (*BucketMeta, error) {
f := b.bucketMetaFile(name)
if _, err := os.Stat(f); os.IsNotExist(err) {
return nil, ErrNoSuchBucket
}
var meta BucketMeta
if err := readMeta(f, &meta); err != nil {
return nil, err
}
return &meta, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Object operations
// ──────────────────────────────────────────────────────────────────────────────

func (b *Backend) objectDataPath(bucket, key string) string {
return filepath.Join(b.objectsDir(bucket), filepath.FromSlash(key))
}

func (b *Backend) objectMetaPath(bucket, key string) string {
return filepath.Join(b.metaDir(bucket), filepath.FromSlash(key)+".meta")
}

// PutObject stores an object, reading data from r. It returns the ObjectMeta.
func (b *Backend) PutObject(bucket, key, contentType string, r io.Reader, userMeta map[string]string) (*ObjectMeta, error) {
b.mu.Lock()
defer b.mu.Unlock()

if _, err := b.readBucketMeta(bucket); err != nil {
return nil, err
}

dataPath := b.objectDataPath(bucket, key)
if err := os.MkdirAll(filepath.Dir(dataPath), 0o750); err != nil {
return nil, fmt.Errorf("create object dir: %w", err)
}

// Write to temp file then rename for atomicity.
tmp, err := os.CreateTemp(filepath.Dir(dataPath), ".tmp-")
if err != nil {
return nil, fmt.Errorf("create temp file: %w", err)
}
tmpName := tmp.Name()
defer func() { _ = os.Remove(tmpName) }()

size, etag, err := writeAndHash(tmp, r)
tmp.Close()
if err != nil {
return nil, fmt.Errorf("write object data: %w", err)
}

if err := os.Rename(tmpName, dataPath); err != nil {
return nil, fmt.Errorf("rename temp file: %w", err)
}

meta := &ObjectMeta{
Key:          key,
Size:         size,
ETag:         fmt.Sprintf(`"%s"`, etag),
ContentType:  contentType,
LastModified: time.Now().UTC(),
UserMeta:     userMeta,
}
metaPath := b.objectMetaPath(bucket, key)
if err := os.MkdirAll(filepath.Dir(metaPath), 0o750); err != nil {
return nil, fmt.Errorf("create meta dir: %w", err)
}
if err := writeMeta(metaPath, meta); err != nil {
return nil, fmt.Errorf("write object meta: %w", err)
}

return meta, nil
}

// GetObject retrieves an object's metadata and a reader for its data.
// The caller is responsible for closing the returned ReadCloser.
func (b *Backend) GetObject(bucket, key string) (*ObjectMeta, io.ReadCloser, error) {
b.mu.RLock()
defer b.mu.RUnlock()

if _, err := b.readBucketMeta(bucket); err != nil {
return nil, nil, err
}

meta, err := b.readObjectMeta(bucket, key)
if err != nil {
return nil, nil, err
}

f, err := os.Open(b.objectDataPath(bucket, key))
if err != nil {
return nil, nil, ErrNoSuchKey
}

return meta, f, nil
}

// HeadObject returns object metadata without reading data.
func (b *Backend) HeadObject(bucket, key string) (*ObjectMeta, error) {
b.mu.RLock()
defer b.mu.RUnlock()

if _, err := b.readBucketMeta(bucket); err != nil {
return nil, err
}
return b.readObjectMeta(bucket, key)
}

// DeleteObject removes an object.
func (b *Backend) DeleteObject(bucket, key string) error {
b.mu.Lock()
defer b.mu.Unlock()

if _, err := b.readBucketMeta(bucket); err != nil {
return err
}

// S3 DeleteObject is idempotent — no error if not found.
dataPath := b.objectDataPath(bucket, key)
metaPath := b.objectMetaPath(bucket, key)
_ = os.Remove(dataPath)
_ = os.Remove(metaPath)

// Remove empty parent directories (but stop at the bucket root).
removeEmptyParents(filepath.Dir(dataPath), b.objectsDir(bucket))
removeEmptyParents(filepath.Dir(metaPath), b.metaDir(bucket))
return nil
}

// removeEmptyParents removes empty directories up to (but not including) root.
func removeEmptyParents(dir, root string) {
for dir != root && dir != "." && dir != "/" {
entries, err := os.ReadDir(dir)
if err != nil || len(entries) != 0 {
break
}
if err := os.Remove(dir); err != nil {
break
}
dir = filepath.Dir(dir)
}
}

// CopyObject copies an object from srcBucket/srcKey to dstBucket/dstKey.
func (b *Backend) CopyObject(srcBucket, srcKey, dstBucket, dstKey string, userMeta map[string]string) (*ObjectMeta, error) {
b.mu.Lock()
defer b.mu.Unlock()

srcMeta, err := b.readObjectMeta(srcBucket, srcKey)
if err != nil {
return nil, err
}

src, err := os.Open(b.objectDataPath(srcBucket, srcKey))
if err != nil {
return nil, ErrNoSuchKey
}
defer src.Close()

if _, err := b.readBucketMeta(dstBucket); err != nil {
return nil, err
}

dstDataPath := b.objectDataPath(dstBucket, dstKey)
if err := os.MkdirAll(filepath.Dir(dstDataPath), 0o750); err != nil {
return nil, fmt.Errorf("create dst dir: %w", err)
}

tmp, err := os.CreateTemp(filepath.Dir(dstDataPath), ".tmp-")
if err != nil {
return nil, fmt.Errorf("create temp file: %w", err)
}
tmpName := tmp.Name()
defer func() { _ = os.Remove(tmpName) }()

size, etag, err := writeAndHash(tmp, src)
tmp.Close()
if err != nil {
return nil, fmt.Errorf("copy data: %w", err)
}

if err := os.Rename(tmpName, dstDataPath); err != nil {
return nil, fmt.Errorf("rename: %w", err)
}

copyMeta := &ObjectMeta{
Key:          dstKey,
Size:         size,
ETag:         fmt.Sprintf(`"%s"`, etag),
ContentType:  srcMeta.ContentType,
LastModified: time.Now().UTC(),
UserMeta:     srcMeta.UserMeta,
}
if len(userMeta) > 0 {
copyMeta.UserMeta = userMeta
}

dstMetaPath := b.objectMetaPath(dstBucket, dstKey)
if err := os.MkdirAll(filepath.Dir(dstMetaPath), 0o750); err != nil {
return nil, fmt.Errorf("create meta dir: %w", err)
}
if err := writeMeta(dstMetaPath, copyMeta); err != nil {
return nil, fmt.Errorf("write meta: %w", err)
}

return copyMeta, nil
}

// ListObjectsResult holds the result of a ListObjects call.
type ListObjectsResult struct {
Objects        []ObjectMeta
CommonPrefixes []string
IsTruncated    bool
NextMarker     string
}

// ListObjects lists objects in a bucket matching prefix, with optional delimiter.
func (b *Backend) ListObjects(bucket, prefix, marker, delimiter string, maxKeys int) (*ListObjectsResult, error) {
b.mu.RLock()
defer b.mu.RUnlock()

if _, err := b.readBucketMeta(bucket); err != nil {
return nil, err
}

objDir := b.objectsDir(bucket)
var allKeys []string

err := filepath.WalkDir(objDir, func(path string, d os.DirEntry, err error) error {
if err != nil || d.IsDir() {
return err
}
key := filepath.ToSlash(strings.TrimPrefix(path, objDir+string(os.PathSeparator)))
if strings.HasPrefix(key, prefix) {
allKeys = append(allKeys, key)
}
return nil
})
if err != nil {
return nil, fmt.Errorf("walk objects: %w", err)
}
sort.Strings(allKeys)

result := &ListObjectsResult{}
prefixSet := make(map[string]struct{})
count := 0

for _, key := range allKeys {
if marker != "" && key <= marker {
continue
}

if delimiter != "" {
// Check if there is a delimiter after the prefix portion.
rest := strings.TrimPrefix(key, prefix)
idx := strings.Index(rest, delimiter)
if idx >= 0 {
cp := prefix + rest[:idx+len(delimiter)]
if _, ok := prefixSet[cp]; !ok {
if count >= maxKeys {
result.IsTruncated = true
break
}
prefixSet[cp] = struct{}{}
result.CommonPrefixes = append(result.CommonPrefixes, cp)
count++
}
continue
}
}

if count >= maxKeys {
result.IsTruncated = true
result.NextMarker = key
break
}

meta, err := b.readObjectMeta(bucket, key)
if err != nil {
continue
}
result.Objects = append(result.Objects, *meta)
count++
}

sort.Strings(result.CommonPrefixes)
return result, nil
}

func (b *Backend) readObjectMeta(bucket, key string) (*ObjectMeta, error) {
metaPath := b.objectMetaPath(bucket, key)
if _, err := os.Stat(metaPath); os.IsNotExist(err) {
return nil, ErrNoSuchKey
}
var meta ObjectMeta
if err := readMeta(metaPath, &meta); err != nil {
return nil, err
}
return &meta, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Multipart upload operations
// ──────────────────────────────────────────────────────────────────────────────

func (b *Backend) multipartDir(uploadID string) string {
return filepath.Join(b.root, "multipart", uploadID)
}

// CreateMultipartUpload initiates a multipart upload, returning the upload ID.
func (b *Backend) CreateMultipartUpload(bucket, key, contentType string, userMeta map[string]string) (string, error) {
b.mu.Lock()
defer b.mu.Unlock()

if _, err := b.readBucketMeta(bucket); err != nil {
return "", err
}

uploadID := uuid.New().String()
dir := b.multipartDir(uploadID)
if err := os.MkdirAll(filepath.Join(dir, "parts"), 0o750); err != nil {
return "", fmt.Errorf("create multipart dir: %w", err)
}

meta := &MultipartMeta{
UploadID:    uploadID,
Bucket:      bucket,
Key:         key,
ContentType: contentType,
UserMeta:    userMeta,
InitiatedAt: time.Now().UTC(),
}
if err := writeMeta(filepath.Join(dir, "meta"), meta); err != nil {
return "", fmt.Errorf("write multipart meta: %w", err)
}
return uploadID, nil
}

// UploadPart stores a single part of a multipart upload, returning its ETag.
func (b *Backend) UploadPart(uploadID string, partNumber int, r io.Reader) (string, error) {
b.mu.Lock()
defer b.mu.Unlock()

dir := b.multipartDir(uploadID)
if _, err := os.Stat(dir); os.IsNotExist(err) {
return "", ErrNoSuchUpload
}

partPath := filepath.Join(dir, "parts", fmt.Sprintf("%05d", partNumber))
f, err := os.Create(partPath)
if err != nil {
return "", fmt.Errorf("create part file: %w", err)
}
defer f.Close()

_, etag, err := writeAndHash(f, r)
if err != nil {
return "", fmt.Errorf("write part: %w", err)
}
return fmt.Sprintf(`"%s"`, etag), nil
}

// CompleteMultipartUpload assembles parts into the final object.
func (b *Backend) CompleteMultipartUpload(uploadID string, parts []Part) (*ObjectMeta, error) {
b.mu.Lock()
defer b.mu.Unlock()

dir := b.multipartDir(uploadID)
var meta MultipartMeta
if err := readMeta(filepath.Join(dir, "meta"), &meta); err != nil {
return nil, ErrNoSuchUpload
}

if _, err := b.readBucketMeta(meta.Bucket); err != nil {
return nil, err
}

dstPath := b.objectDataPath(meta.Bucket, meta.Key)
if err := os.MkdirAll(filepath.Dir(dstPath), 0o750); err != nil {
return nil, fmt.Errorf("create object dir: %w", err)
}

dst, err := os.Create(dstPath)
if err != nil {
return nil, fmt.Errorf("create object file: %w", err)
}
defer dst.Close()

h := sha256.New()
var totalSize int64

sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })

for _, p := range parts {
partPath := filepath.Join(dir, "parts", fmt.Sprintf("%05d", p.PartNumber))
f, err := os.Open(partPath)
if err != nil {
return nil, fmt.Errorf("open part %d: %w", p.PartNumber, err)
}
n, copyErr := io.Copy(io.MultiWriter(dst, h), f)
f.Close()
if copyErr != nil {
return nil, fmt.Errorf("copy part %d: %w", p.PartNumber, copyErr)
}
totalSize += n
}

etag := fmt.Sprintf(`"%s-%d"`, hex.EncodeToString(h.Sum(nil)), len(parts))

objMeta := &ObjectMeta{
Key:          meta.Key,
Size:         totalSize,
ETag:         etag,
ContentType:  meta.ContentType,
LastModified: time.Now().UTC(),
UserMeta:     meta.UserMeta,
}
metaPath := b.objectMetaPath(meta.Bucket, meta.Key)
if err := os.MkdirAll(filepath.Dir(metaPath), 0o750); err != nil {
return nil, fmt.Errorf("create meta dir: %w", err)
}
if err := writeMeta(metaPath, objMeta); err != nil {
return nil, fmt.Errorf("write object meta: %w", err)
}

// Clean up multipart data.
_ = os.RemoveAll(dir)
return objMeta, nil
}

// AbortMultipartUpload cancels and removes a multipart upload.
func (b *Backend) AbortMultipartUpload(uploadID string) error {
b.mu.Lock()
defer b.mu.Unlock()

dir := b.multipartDir(uploadID)
if _, err := os.Stat(dir); os.IsNotExist(err) {
return ErrNoSuchUpload
}
return os.RemoveAll(dir)
}

// ListParts returns the uploaded parts for a multipart upload.
func (b *Backend) ListParts(uploadID string) ([]Part, *MultipartMeta, error) {
b.mu.RLock()
defer b.mu.RUnlock()

dir := b.multipartDir(uploadID)
var meta MultipartMeta
if err := readMeta(filepath.Join(dir, "meta"), &meta); err != nil {
return nil, nil, ErrNoSuchUpload
}

partsDir := filepath.Join(dir, "parts")
entries, err := os.ReadDir(partsDir)
if err != nil {
return nil, &meta, nil
}

var parts []Part
for _, e := range entries {
if e.IsDir() {
continue
}
var partNum int
fmt.Sscanf(e.Name(), "%d", &partNum) //nolint:errcheck
info, _ := e.Info()
var size int64
if info != nil {
size = info.Size()
}
parts = append(parts, Part{PartNumber: partNum, Size: size})
}
return parts, &meta, nil
}

// ListMultipartUploads returns all in-progress multipart uploads for a bucket.
func (b *Backend) ListMultipartUploads(bucket string) ([]MultipartMeta, error) {
b.mu.RLock()
defer b.mu.RUnlock()

multipartDir := filepath.Join(b.root, "multipart")
entries, err := os.ReadDir(multipartDir)
if err != nil {
return nil, nil
}

var results []MultipartMeta
for _, e := range entries {
if !e.IsDir() {
continue
}
var meta MultipartMeta
if err := readMeta(filepath.Join(multipartDir, e.Name(), "meta"), &meta); err != nil {
continue
}
if meta.Bucket == bucket {
results = append(results, meta)
}
}
return results, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Errors
// ──────────────────────────────────────────────────────────────────────────────

// S3Error represents a storage error with an associated S3 error code.
type S3Error struct {
Code    string
Message string
}

func (e *S3Error) Error() string { return e.Message }

var (
ErrBucketExists   = &S3Error{"BucketAlreadyOwnedByYou", "bucket already exists"}
ErrBucketNotEmpty = &S3Error{"BucketNotEmpty", "bucket is not empty"}
ErrNoSuchBucket   = &S3Error{"NoSuchBucket", "the specified bucket does not exist"}
ErrNoSuchKey      = &S3Error{"NoSuchKey", "the specified key does not exist"}
ErrNoSuchUpload   = &S3Error{"NoSuchUpload", "the specified multipart upload does not exist"}
)

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func writeMeta(path string, v interface{}) error {
data, err := json.Marshal(v)
if err != nil {
return err
}
return os.WriteFile(path, data, 0o640)
}

func readMeta(path string, v interface{}) error {
data, err := os.ReadFile(path)
if err != nil {
return err
}
return json.Unmarshal(data, v)
}

func writeAndHash(w io.Writer, r io.Reader) (int64, string, error) {
h := sha256.New()
n, err := io.Copy(io.MultiWriter(w, h), r)
if err != nil {
return 0, "", err
}
return n, hex.EncodeToString(h.Sum(nil)), nil
}
