import { useEffect, useState } from 'react';
import { Database, FileText, HardDrive, Clock } from 'lucide-react';
import { api } from '../api.js';

function formatBytes(bytes) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function formatUptime(seconds) {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = seconds % 60;
  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

export function Dashboard() {
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchStats = () => {
      api.stats().then(setStats).catch(console.error).finally(() => setLoading(false));
    };
    fetchStats();
    const interval = setInterval(fetchStats, 10000);
    return () => clearInterval(interval);
  }, []);

  const cards = [
    { label: 'Buckets', value: stats?.buckets ?? 0, icon: Database, color: 'bg-blue-500' },
    { label: 'Objects', value: stats?.objects ?? 0, icon: FileText, color: 'bg-emerald-500' },
    { label: 'Total Size', value: formatBytes(stats?.total_size ?? 0), icon: HardDrive, color: 'bg-purple-500' },
    { label: 'Uptime', value: formatUptime(stats?.uptime_seconds ?? 0), icon: Clock, color: 'bg-orange-500' },
  ];

  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-800 mb-6">Dashboard</h1>
      {loading ? (
        <div className="text-gray-400 text-sm">Loading stats…</div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-4 gap-6">
          {cards.map(({ label, value, icon: Icon, color }) => (
            <div key={label} className="bg-white rounded-xl shadow-sm border border-gray-100 p-6 flex items-center gap-4">
              <div className={`w-12 h-12 ${color} rounded-xl flex items-center justify-center flex-shrink-0`}>
                <Icon size={22} className="text-white" />
              </div>
              <div>
                <p className="text-sm text-gray-500">{label}</p>
                <p className="text-2xl font-bold text-gray-800">{value}</p>
              </div>
            </div>
          ))}
        </div>
      )}

      <div className="mt-8 bg-white rounded-xl shadow-sm border border-gray-100 p-6">
        <h2 className="text-lg font-semibold text-gray-800 mb-4">Quick Start</h2>
        <div className="space-y-3 text-sm text-gray-600">
          <p>Connect your AWS SDK by pointing the endpoint to this server:</p>
          <pre className="bg-gray-50 rounded-lg p-4 text-xs overflow-x-auto font-mono">
{`import boto3

s3 = boto3.client(
    's3',
    endpoint_url='http://localhost:9000',
    aws_access_key_id='minioadmin',
    aws_secret_access_key='minioadmin',
    region_name='us-east-1',
)

# Create a bucket
s3.create_bucket(Bucket='my-bucket')

# Upload a file
s3.upload_file('file.txt', 'my-bucket', 'file.txt')
`}
          </pre>
        </div>
      </div>
    </div>
  );
}
