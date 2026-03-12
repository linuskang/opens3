package s3api_test

import (
"encoding/json"
"net/http"
"net/http/httptest"
"os"
"strings"
"testing"

"github.com/gorilla/mux"
"github.com/linuskang/opens3/internal/auth"
"github.com/linuskang/opens3/internal/s3api"
"github.com/linuskang/opens3/internal/storage"
"github.com/linuskang/opens3/internal/webui"
)

func TestPublicBucket(t *testing.T) {
dir, err := os.MkdirTemp("", "opens3test")
if err != nil {
t.Fatalf("create temp dir: %v", err)
}
defer os.RemoveAll(dir)

store, err := storage.NewBackend(dir)
if err != nil {
t.Fatalf("create storage backend: %v", err)
}
verifier := auth.NewVerifier("minioadmin", "minioadmin", "us-east-1")

r := mux.NewRouter()
s3Handler := s3api.NewHandler(store, verifier, "us-east-1")
s3Handler.Register(r)
uiHandler := webui.NewHandler(store, 9001)
uiHandler.Register(r)

ts := httptest.NewServer(r)
defer ts.Close()

client := ts.Client()

// Create bucket via webui API
body := `{"name":"my-bucket"}`
resp, err := client.Post(ts.URL+"/_opens3/api/buckets", "application/json", strings.NewReader(body))
if err != nil || resp.StatusCode != 201 {
t.Fatalf("create bucket failed: %v status=%d", err, resp.StatusCode)
}

// Upload an object directly via storage
if _, err := store.PutObject("my-bucket", "avatar.png", "image/png", strings.NewReader("fake image data"), nil); err != nil {
t.Fatalf("put object: %v", err)
}

// GET without auth on private bucket should return 403
resp3, _ := client.Get(ts.URL + "/my-bucket/avatar.png")
if resp3.StatusCode != http.StatusForbidden {
t.Errorf("expected 403 on private bucket GET without auth, got %d", resp3.StatusCode)
}

// Set bucket to public via PATCH
patchReq, _ := http.NewRequest("PATCH", ts.URL+"/_opens3/api/buckets/my-bucket",
strings.NewReader(`{"public":true}`))
patchReq.Header.Set("Content-Type", "application/json")
resp4, _ := client.Do(patchReq)
if resp4.StatusCode != 200 {
t.Errorf("expected 200 on PATCH bucket public, got %d", resp4.StatusCode)
}

// GET without auth on public bucket should return 200
resp5, _ := client.Get(ts.URL + "/my-bucket/avatar.png")
if resp5.StatusCode != http.StatusOK {
t.Errorf("expected 200 on public bucket GET without auth, got %d", resp5.StatusCode)
}

// PUT without auth on public bucket should still return 403
putReq, _ := http.NewRequest("PUT", ts.URL+"/my-bucket/new-file.txt",
strings.NewReader("new content"))
resp6, _ := client.Do(putReq)
if resp6.StatusCode != http.StatusForbidden {
t.Errorf("expected 403 on public bucket PUT without auth, got %d", resp6.StatusCode)
}

// List buckets reflects public=true
resp7, _ := client.Get(ts.URL + "/_opens3/api/buckets")
var buckets []map[string]interface{}
json.NewDecoder(resp7.Body).Decode(&buckets)
if len(buckets) == 0 || buckets[0]["public"] != true {
t.Errorf("expected bucket public=true, got %v", buckets)
}
}
