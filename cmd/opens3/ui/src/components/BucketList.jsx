import { useState, useEffect } from 'react';
import { Database, Plus, Trash2, ArrowRight, RefreshCw, Globe, Lock } from 'lucide-react';
import { api } from '../api.js';
import { Modal } from './Modal.jsx';

function formatBytes(bytes) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

export function BucketList({ onSelectBucket, showToast }) {
  const [buckets, setBuckets] = useState([]);
  const [loading, setLoading] = useState(false);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newBucketName, setNewBucketName] = useState('');
  const [creating, setCreating] = useState(false);

  const load = () => {
    setLoading(true);
    api.listBuckets()
      .then(setBuckets)
      .catch(e => showToast(e.message, 'error'))
      .finally(() => setLoading(false));
  };

  useEffect(() => { load(); }, []);

  const handleCreate = async (e) => {
    e.preventDefault();
    if (!newBucketName.trim()) return;
    setCreating(true);
    try {
      await api.createBucket(newBucketName.trim());
      showToast(`Bucket "${newBucketName}" created`, 'success');
      setNewBucketName('');
      setShowCreateModal(false);
      load();
    } catch (err) {
      showToast('Create failed: ' + err.message, 'error');
    } finally {
      setCreating(false);
    }
  };

  const handleDelete = async (name, e) => {
    e.stopPropagation();
    if (!window.confirm(`Delete bucket "${name}"? This cannot be undone.`)) return;
    try {
      await api.deleteBucket(name);
      showToast(`Bucket "${name}" deleted`, 'success');
      load();
    } catch (err) {
      showToast('Delete failed: ' + err.message, 'error');
    }
  };

  const handleTogglePublic = async (name, currentlyPublic, e) => {
    e.stopPropagation();
    try {
      await api.setBucketPublic(name, !currentlyPublic);
      showToast(`Bucket "${name}" is now ${!currentlyPublic ? 'public' : 'private'}`, 'success');
      load();
    } catch (err) {
      showToast('Update failed: ' + err.message, 'error');
    }
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-800">Buckets</h1>
        <div className="flex items-center gap-2">
          <button onClick={load} className="p-2 rounded-lg hover:bg-gray-100 text-gray-500" title="Refresh">
            <RefreshCw size={18} className={loading ? 'animate-spin' : ''} />
          </button>
          <button
            onClick={() => setShowCreateModal(true)}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700"
          >
            <Plus size={16} /> New Bucket
          </button>
        </div>
      </div>

      {loading && <p className="text-gray-400 text-sm">Loading…</p>}

      {!loading && buckets.length === 0 && (
        <div className="text-center py-16 text-gray-400">
          <Database size={48} className="mx-auto mb-3 opacity-30" />
          <p className="text-lg font-medium">No buckets yet</p>
          <p className="text-sm mt-1">Click "New Bucket" to get started</p>
        </div>
      )}

      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
        {buckets.map(b => (
          <div
            key={b.name}
            onClick={() => onSelectBucket(b.name)}
            className="bg-white rounded-xl shadow-sm border border-gray-100 p-5 cursor-pointer hover:shadow-md hover:border-blue-200 transition-all group"
          >
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 bg-blue-50 rounded-xl flex items-center justify-center">
                  <Database size={20} className="text-blue-500" />
                </div>
                <div>
                  <h3 className="font-semibold text-gray-800 group-hover:text-blue-600 transition-colors">{b.name}</h3>
                  <p className="text-xs text-gray-400 mt-0.5">
                    {new Date(b.created_at).toLocaleDateString()}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                <button
                  onClick={(e) => handleTogglePublic(b.name, b.public, e)}
                  className={`p-1.5 rounded-lg text-gray-400 ${b.public ? 'hover:bg-orange-50 hover:text-orange-600' : 'hover:bg-green-50 hover:text-green-600'}`}
                  title={b.public ? 'Make private' : 'Make public'}
                >
                  {b.public ? <Lock size={15} /> : <Globe size={15} />}
                </button>
                <button
                  onClick={(e) => handleDelete(b.name, e)}
                  className="p-1.5 rounded-lg hover:bg-red-50 text-gray-400 hover:text-red-600"
                  title="Delete bucket"
                >
                  <Trash2 size={15} />
                </button>
                <ArrowRight size={16} className="text-gray-400" />
              </div>
            </div>
            <div className="mt-4 grid grid-cols-2 gap-3 text-sm">
              <div>
                <p className="text-gray-400 text-xs">Objects</p>
                <p className="font-semibold text-gray-700">{b.objects}</p>
              </div>
              <div>
                <p className="text-gray-400 text-xs">Size</p>
                <p className="font-semibold text-gray-700">{formatBytes(b.size)}</p>
              </div>
            </div>
            <div className="mt-3">
              {b.public ? (
                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-green-50 text-green-700">
                  <Globe size={11} /> Public
                </span>
              ) : (
                <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-gray-50 text-gray-500">
                  <Lock size={11} /> Private
                </span>
              )}
            </div>
          </div>
        ))}
      </div>

      {showCreateModal && (
        <Modal title="Create Bucket" onClose={() => setShowCreateModal(false)}>
          <form onSubmit={handleCreate} className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Bucket name</label>
              <input
                type="text"
                value={newBucketName}
                onChange={e => setNewBucketName(e.target.value)}
                placeholder="my-bucket"
                className="w-full px-3 py-2 border border-gray-200 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                autoFocus
                pattern="[a-z0-9][a-z0-9\-\.]{1,61}[a-z0-9]"
                title="3-63 characters: lowercase letters, numbers, hyphens, dots"
              />
              <p className="text-xs text-gray-400 mt-1">3-63 characters; lowercase letters, numbers, hyphens, dots</p>
            </div>
            <div className="flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setShowCreateModal(false)}
                className="px-4 py-2 text-sm text-gray-600 hover:bg-gray-100 rounded-lg"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={creating || !newBucketName.trim()}
                className="px-4 py-2 text-sm bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
              >
                {creating ? 'Creating…' : 'Create'}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </div>
  );
}
