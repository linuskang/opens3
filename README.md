# Opens3

A lightweight, self-hosted, **AWS S3-compatible** object storage server with a web UI, packaged as a single Docker container.

## Features

- **Full S3 API compatibility** — works with any existing AWS S3 SDK (Python/boto3, Node.js, Go, AWS CLI, etc.)
- **Web UI** — bucket browser, drag-and-drop upload, object download/delete, dashboard stats
- **AWS Signature V4** authentication
- **Multipart upload** support (large files)
- **Range requests** (streaming, resume)
- **CopyObject**, **DeleteObjects** (batch delete)
- **Filesystem-based storage** — data persists in a mounted Docker volume
- **Single Docker container** — no external dependencies

## Quick Start with Docker

```bash
docker run -d \
  --name opens3 \
  -p 9000:9000 \
  -p 9001:9001 \
  -v opens3-data:/data \
  -e OPENS3_ACCESS_KEY=minioadmin \
  -e OPENS3_SECRET_KEY=minioadmin \
  ghcr.io/linuskang/opens3:latest
```

Then open **http://localhost:9000** (redirects to the Web UI at `http://localhost:9000/_opens3/`).
The S3-compatible API is available at **http://localhost:9001**.

## Build from Source

**Prerequisites:** Go 1.24+, Node.js 18+

```bash
# Build the React UI
cd cmd/opens3/ui
npm install
npm run build
cd ../../..

# Build the Go binary (embeds the UI)
go build -o opens3 ./cmd/opens3/

# Run
./opens3
```

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|---|---|---|
| `OPENS3_API_PORT` | `9001` | Port for the S3-compatible bucket API |
| `OPENS3_UI_PORT` | `9000` | Port for the web UI |
| `OPENS3_DATA_DIR` | `/data` | Directory for data storage |
| `OPENS3_ACCESS_KEY` | `minioadmin` | S3 access key |
| `OPENS3_SECRET_KEY` | `minioadmin` | S3 secret key |
| `OPENS3_REGION` | `us-east-1` | S3 region name |
| `OPENS3_UI_DISABLED` | `false` | Set to `true` to disable the web UI |

## Using with AWS SDKs

### Python (boto3)

```python
import boto3

s3 = boto3.client(
    's3',
    endpoint_url='http://localhost:9001',
    aws_access_key_id='minioadmin',
    aws_secret_access_key='minioadmin',
    region_name='us-east-1',
)

s3.create_bucket(Bucket='my-bucket')
s3.upload_file('file.txt', 'my-bucket', 'file.txt')
response = s3.get_object(Bucket='my-bucket', Key='file.txt')
```

## S3 API Reference

### Service operations

| Operation | Method | Endpoint |
|---|---|---|
| ListBuckets | `GET` | `/` |

### Bucket operations

| Operation | Method | Endpoint |
|---|---|---|
| CreateBucket | `PUT` | `/{bucket}` |
| HeadBucket | `HEAD` | `/{bucket}` |
| DeleteBucket | `DELETE` | `/{bucket}` |
| GetBucketLocation | `GET` | `/{bucket}?location` |
| GetBucketVersioning | `GET` | `/{bucket}?versioning` |
| ListObjects v1 | `GET` | `/{bucket}` |
| ListObjects v2 | `GET` | `/{bucket}?list-type=2` |
| ListMultipartUploads | `GET` | `/{bucket}?uploads` |
| DeleteObjects | `POST` | `/{bucket}?delete` |

### Object operations

| Operation | Method | Endpoint |
|---|---|---|
| PutObject | `PUT` | `/{bucket}/{key}` |
| GetObject | `GET` | `/{bucket}/{key}` |
| HeadObject | `HEAD` | `/{bucket}/{key}` |
| DeleteObject | `DELETE` | `/{bucket}/{key}` |
| CopyObject | `PUT` | `/{bucket}/{key}` + `X-Amz-Copy-Source` header |
| CreateMultipartUpload | `POST` | `/{bucket}/{key}?uploads` |
| UploadPart | `PUT` | `/{bucket}/{key}?partNumber=N&uploadId=X` |
| CompleteMultipartUpload | `POST` | `/{bucket}/{key}?uploadId=X` |
| AbortMultipartUpload | `DELETE` | `/{bucket}/{key}?uploadId=X` |
| ListParts | `GET` | `/{bucket}/{key}?uploadId=X` |

## Web UI

The web UI is served at `http://localhost:9000/_opens3/` and provides:

- **Dashboard** — live stats (buckets, objects, total size, uptime) + SDK quick-start code
- **Buckets** — create/delete buckets, see object count and total size
- **Object Browser** — navigate folder hierarchy, upload (drag & drop or file picker), download, delete
- **Server Info** — connection details and supported operations

## License

MIT
