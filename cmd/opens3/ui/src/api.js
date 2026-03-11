const BASE = '/_opens3/api';

async function request(method, path, body, isFormData) {
  const opts = {
    method,
    headers: {},
  };
  if (body && isFormData) {
    opts.body = body;
  } else if (body) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  const res = await fetch(BASE + path, opts);
  if (res.status === 204) return null;
  const data = await res.json().catch(() => null);
  if (!res.ok) {
    throw new Error((data && data.error) || `HTTP ${res.status}`);
  }
  return data;
}

export const api = {
  // Buckets
  listBuckets: () => request('GET', '/buckets'),
  createBucket: (name) => request('POST', '/buckets', { name }),
  deleteBucket: (name) => request('DELETE', `/buckets/${encodeURIComponent(name)}`),
  setBucketPublic: (name, isPublic) => request('PATCH', `/buckets/${encodeURIComponent(name)}`, { public: isPublic }),

  // Objects
  listObjects: (bucket, prefix = '', delimiter = '/') =>
    request('GET', `/buckets/${encodeURIComponent(bucket)}/objects?prefix=${encodeURIComponent(prefix)}&delimiter=${encodeURIComponent(delimiter)}`),

  uploadObjects: (bucket, files, prefix = '') => {
    const form = new FormData();
    form.append('prefix', prefix);
    for (const f of files) form.append('files', f);
    return request('POST', `/buckets/${encodeURIComponent(bucket)}/objects`, form, true);
  },

  deleteObject: (bucket, key) =>
    request('DELETE', `/buckets/${encodeURIComponent(bucket)}/objects/${encodeURIComponent(key)}`),

  downloadUrl: (bucket, key) =>
    `/_opens3/download/${encodeURIComponent(bucket)}/${encodeURIComponent(key)}`,

  // Stats
  stats: () => request('GET', '/stats'),
};
