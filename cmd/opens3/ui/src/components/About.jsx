import { useEffect, useState } from 'react';
import { Server, Key, Globe, Info } from 'lucide-react';

export function About() {
  const [apiPort, setApiPort] = useState(9001);

  useEffect(() => {
    fetch('/_opens3/api/config')
      .then(r => r.json())
      .then(d => { if (d.api_port) setApiPort(d.api_port); })
      .catch(() => {});
  }, []);

  const apiBase = `${window.location.protocol}//${window.location.hostname}:${apiPort}`;

  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-800 mb-6">Server Info</h1>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Server info */}
        <div className="bg-white rounded-xl shadow-sm border border-gray-100 p-6">
          <div className="flex items-center gap-2 mb-4">
            <Server size={18} className="text-blue-500" />
            <h2 className="font-semibold text-gray-800">About Opens3</h2>
          </div>
          <dl className="space-y-3 text-sm">
            <div className="flex justify-between">
              <dt className="text-gray-500">Version</dt>
              <dd className="font-medium text-gray-800">1.0.0</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-gray-500">Protocol</dt>
              <dd className="font-medium text-gray-800">AWS S3 REST API v2</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-gray-500">Storage</dt>
              <dd className="font-medium text-gray-800">Local Filesystem</dd>
            </div>
            <div className="flex justify-between">
              <dt className="text-gray-500">Auth</dt>
              <dd className="font-medium text-gray-800">AWS Signature V4</dd>
            </div>
          </dl>
        </div>

        {/* Connection info */}
        <div className="bg-white rounded-xl shadow-sm border border-gray-100 p-6">
          <div className="flex items-center gap-2 mb-4">
            <Key size={18} className="text-emerald-500" />
            <h2 className="font-semibold text-gray-800">Connection Details</h2>
          </div>
          <dl className="space-y-3 text-sm">
            <div>
              <dt className="text-gray-500 mb-1">S3 API Endpoint</dt>
              <dd className="font-mono text-xs bg-gray-50 px-3 py-2 rounded-lg text-gray-800">
                {apiBase}
              </dd>
            </div>
            <div>
              <dt className="text-gray-500 mb-1">Web UI</dt>
              <dd className="font-mono text-xs bg-gray-50 px-3 py-2 rounded-lg text-gray-800">
                {window.location.protocol}//{window.location.hostname}:{window.location.port || 9000}/_opens3/
              </dd>
            </div>
            <div>
              <dt className="text-gray-500 mb-1">Default Credentials</dt>
              <dd className="font-mono text-xs bg-gray-50 px-3 py-2 rounded-lg text-gray-800">
                Access Key: <strong>minioadmin</strong><br />
                Secret Key: <strong>minioadmin</strong>
              </dd>
            </div>
          </dl>
        </div>

        {/* SDK examples */}
        <div className="bg-white rounded-xl shadow-sm border border-gray-100 p-6 lg:col-span-2">
          <div className="flex items-center gap-2 mb-4">
            <Globe size={18} className="text-purple-500" />
            <h2 className="font-semibold text-gray-800">SDK Quick Start</h2>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <p className="text-xs font-medium text-gray-500 mb-2">Python (boto3)</p>
              <pre className="bg-gray-900 text-green-400 text-xs p-4 rounded-lg overflow-x-auto font-mono">
{"import boto3\ns3 = boto3.client(\n  's3',\n  endpoint_url='" + apiBase + "',\n  aws_access_key_id='minioadmin',\n  aws_secret_access_key='minioadmin',\n  region_name='us-east-1',\n)\ns3.create_bucket(Bucket='test')\ns3.upload_file('f.txt', 'test', 'f.txt')"}
              </pre>
            </div>
            <div>
              <p className="text-xs font-medium text-gray-500 mb-2">Node.js (AWS SDK v3)</p>
              <pre className="bg-gray-900 text-green-400 text-xs p-4 rounded-lg overflow-x-auto font-mono">
{"import { S3Client, PutObjectCommand }\n  from \"@aws-sdk/client-s3\";\nconst s3 = new S3Client({\n  endpoint: \"" + apiBase + "\",\n  region: \"us-east-1\",\n  credentials: {\n    accessKeyId: \"minioadmin\",\n    secretAccessKey: \"minioadmin\",\n  },\n  forcePathStyle: true,\n});"}
              </pre>
            </div>
            <div>
              <p className="text-xs font-medium text-gray-500 mb-2">AWS CLI</p>
              <pre className="bg-gray-900 text-green-400 text-xs p-4 rounded-lg overflow-x-auto font-mono">
{"aws configure set aws_access_key_id minioadmin\naws configure set aws_secret_access_key minioadmin\naws configure set region us-east-1\n\naws --endpoint-url " + apiBase + " \\\n  s3 ls\naws --endpoint-url " + apiBase + " \\\n  s3 mb s3://my-bucket\naws --endpoint-url " + apiBase + " \\\n  s3 cp file.txt s3://my-bucket/"}
              </pre>
            </div>
            <div>
              <p className="text-xs font-medium text-gray-500 mb-2">Go (AWS SDK v2)</p>
              <pre className="bg-gray-900 text-green-400 text-xs p-4 rounded-lg overflow-x-auto font-mono">
{"cfg, _ := config.LoadDefaultConfig(ctx,\n  config.WithRegion(\"us-east-1\"),\n  config.WithCredentialsProvider(\n    credentials.NewStaticCredentialsProvider(\n      \"minioadmin\", \"minioadmin\", \"\",\n    ),\n  ),\n)\nclient := s3.NewFromConfig(cfg, func(o *s3.Options) {\n  o.BaseEndpoint = aws.String(\"" + apiBase + "\")\n  o.UsePathStyle = true\n})"}
              </pre>
            </div>
          </div>
        </div>

        {/* Supported operations */}
        <div className="bg-white rounded-xl shadow-sm border border-gray-100 p-6 lg:col-span-2">
          <div className="flex items-center gap-2 mb-4">
            <Info size={18} className="text-orange-500" />
            <h2 className="font-semibold text-gray-800">Supported S3 Operations</h2>
          </div>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3 text-sm">
            {[
              ['ListBuckets', 'GET /'],
              ['CreateBucket', 'PUT /{bucket}'],
              ['HeadBucket', 'HEAD /{bucket}'],
              ['DeleteBucket', 'DELETE /{bucket}'],
              ['GetBucketLocation', 'GET /{bucket}?location'],
              ['GetBucketVersioning', 'GET /{bucket}?versioning'],
              ['ListObjects v1/v2', 'GET /{bucket}'],
              ['ListMultipartUploads', 'GET /{bucket}?uploads'],
              ['DeleteObjects', 'POST /{bucket}?delete'],
              ['PutObject', 'PUT /{bucket}/{key}'],
              ['GetObject', 'GET /{bucket}/{key}'],
              ['HeadObject', 'HEAD /{bucket}/{key}'],
              ['DeleteObject', 'DELETE /{bucket}/{key}'],
              ['CopyObject', 'PUT /{bucket}/{key} (copy-source)'],
              ['Range Requests', 'GET with Range header'],
              ['CreateMultipartUpload', 'POST /{bucket}/{key}?uploads'],
              ['UploadPart', 'PUT /{bucket}/{key}?partNumber'],
              ['CompleteMultipartUpload', 'POST /{bucket}/{key}?uploadId'],
              ['AbortMultipartUpload', 'DELETE /{bucket}/{key}?uploadId'],
              ['ListParts', 'GET /{bucket}/{key}?uploadId'],
            ].map(([op, endpoint]) => (
              <div key={op} className="flex items-start gap-2 p-3 bg-gray-50 rounded-lg">
                <div className="w-2 h-2 bg-green-500 rounded-full mt-1.5 flex-shrink-0" />
                <div>
                  <p className="font-medium text-gray-800 text-xs">{op}</p>
                  <p className="text-gray-400 text-xs font-mono">{endpoint}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
