import { useState, useEffect, useCallback, useRef } from 'react';
import {
  ArrowLeft, Upload, Trash2, Download, FolderOpen, File,
  RefreshCw, Plus, ChevronRight, Search
} from 'lucide-react';
import { api } from '../api.js';
import { Modal } from './Modal.jsx';

function formatBytes(bytes) {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function formatDate(d) {
  if (!d) return '—';
  return new Date(d).toLocaleString();
}

export function ObjectBrowser({ bucket, onBack, showToast }) {
  const [prefix, setPrefix] = useState('');
  const [objects, setObjects] = useState([]);
  const [loading, setLoading] = useState(false);
  const [selected, setSelected] = useState(new Set());
  const [dragging, setDragging] = useState(false);
  const [search, setSearch] = useState('');
  const [showUploadModal, setShowUploadModal] = useState(false);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef(null);

  const load = useCallback(() => {
    setLoading(true);
    setSelected(new Set());
    api.listObjects(bucket, prefix)
      .then(setObjects)
      .catch(e => showToast(e.message, 'error'))
      .finally(() => setLoading(false));
  }, [bucket, prefix, showToast]);

  useEffect(() => { load(); }, [load]);

  const navigateInto = (folderKey) => {
    setPrefix(folderKey);
    setSearch('');
  };

  const navigateBreadcrumb = (idx) => {
    const parts = prefix.split('/').filter(Boolean);
    const newPrefix = parts.slice(0, idx + 1).join('/') + '/';
    setPrefix(newPrefix);
    setSearch('');
  };

  const breadcrumbs = prefix.split('/').filter(Boolean);

  const handleUpload = async (files) => {
    if (!files || files.length === 0) return;
    setUploading(true);
    try {
      await api.uploadObjects(bucket, Array.from(files), prefix);
      showToast(`Uploaded ${files.length} file(s) successfully`, 'success');
      load();
    } catch (e) {
      showToast('Upload failed: ' + e.message, 'error');
    } finally {
      setUploading(false);
      setShowUploadModal(false);
    }
  };

  const handleDelete = async (key) => {
    if (!window.confirm(`Delete "${key}"?`)) return;
    try {
      await api.deleteObject(bucket, key);
      showToast('Object deleted', 'success');
      load();
    } catch (e) {
      showToast('Delete failed: ' + e.message, 'error');
    }
  };

  const handleDeleteSelected = async () => {
    if (!window.confirm(`Delete ${selected.size} selected object(s)?`)) return;
    let errors = 0;
    for (const key of selected) {
      try { await api.deleteObject(bucket, key); } catch { errors++; }
    }
    if (errors > 0) showToast(`${errors} deletions failed`, 'error');
    else showToast(`Deleted ${selected.size} object(s)`, 'success');
    load();
  };

  const toggleSelect = (key) => {
    setSelected(prev => {
      const next = new Set(prev);
      next.has(key) ? next.delete(key) : next.add(key);
      return next;
    });
  };

  const onDrop = (e) => {
    e.preventDefault();
    setDragging(false);
    const files = e.dataTransfer.files;
    if (files.length > 0) handleUpload(files);
  };

  const filtered = objects.filter(obj => {
    if (!search) return true;
    return obj.key.toLowerCase().includes(search.toLowerCase());
  });

  return (
    <div
      className={`flex-1 ${dragging ? 'outline-dashed outline-2 outline-blue-500 rounded-xl bg-blue-50' : ''}`}
      onDragOver={e => { e.preventDefault(); setDragging(true); }}
      onDragLeave={() => setDragging(false)}
      onDrop={onDrop}
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <button onClick={onBack} className="p-2 rounded-lg hover:bg-gray-100 text-gray-500">
            <ArrowLeft size={18} />
          </button>
          <div>
            <h1 className="text-2xl font-bold text-gray-800">{bucket}</h1>
            {/* Breadcrumb */}
            <nav className="flex items-center gap-1 text-sm text-gray-500 mt-0.5">
              <button onClick={() => setPrefix('')} className="hover:text-blue-600">root</button>
              {breadcrumbs.map((part, idx) => (
                <span key={idx} className="flex items-center gap-1">
                  <ChevronRight size={14} />
                  <button onClick={() => navigateBreadcrumb(idx)} className="hover:text-blue-600">{part}</button>
                </span>
              ))}
            </nav>
          </div>
        </div>
        <div className="flex items-center gap-2">
          {selected.size > 0 && (
            <button
              onClick={handleDeleteSelected}
              className="flex items-center gap-2 px-3 py-2 bg-red-600 text-white rounded-lg text-sm hover:bg-red-700"
            >
              <Trash2 size={16} /> Delete ({selected.size})
            </button>
          )}
          <button onClick={load} className="p-2 rounded-lg hover:bg-gray-100 text-gray-500" title="Refresh">
            <RefreshCw size={18} className={loading ? 'animate-spin' : ''} />
          </button>
          <button
            onClick={() => setShowUploadModal(true)}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700"
          >
            <Upload size={16} /> Upload
          </button>
        </div>
      </div>

      {/* Search bar */}
      <div className="relative mb-4">
        <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
        <input
          type="text"
          placeholder="Search objects…"
          value={search}
          onChange={e => setSearch(e.target.value)}
          className="w-full pl-9 pr-3 py-2 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>

      {/* Drag & Drop hint */}
      {dragging && (
        <div className="text-center py-12 text-blue-600 font-medium text-lg">
          Drop files to upload
        </div>
      )}

      {/* Object table */}
      {!dragging && (
        <div className="bg-white rounded-xl shadow-sm border border-gray-100 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-gray-50 border-b border-gray-100">
              <tr>
                <th className="w-8 px-4 py-3"><input type="checkbox" className="rounded" onChange={e => {
                  if (e.target.checked) setSelected(new Set(filtered.filter(o => !o.is_dir).map(o => o.key)));
                  else setSelected(new Set());
                }} /></th>
                <th className="text-left px-3 py-3 font-medium text-gray-600">Name</th>
                <th className="text-right px-3 py-3 font-medium text-gray-600 w-28">Size</th>
                <th className="text-left px-3 py-3 font-medium text-gray-600 w-48 hidden lg:table-cell">Last Modified</th>
                <th className="text-left px-3 py-3 font-medium text-gray-600 w-48 hidden xl:table-cell">Content Type</th>
                <th className="px-3 py-3 w-20"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-50">
              {loading && (
                <tr><td colSpan="6" className="text-center py-8 text-gray-400">Loading…</td></tr>
              )}
              {!loading && filtered.length === 0 && (
                <tr>
                  <td colSpan="6" className="text-center py-12 text-gray-400">
                    <div className="flex flex-col items-center gap-2">
                      <FolderOpen size={40} className="opacity-40" />
                      <p>{search ? 'No results' : 'No objects here. Drop files to upload.'}</p>
                    </div>
                  </td>
                </tr>
              )}
              {filtered.map(obj => (
                <tr key={obj.key} className="hover:bg-gray-50 transition-colors">
                  <td className="px-4 py-3">
                    {!obj.is_dir && (
                      <input
                        type="checkbox"
                        className="rounded"
                        checked={selected.has(obj.key)}
                        onChange={() => toggleSelect(obj.key)}
                      />
                    )}
                  </td>
                  <td className="px-3 py-3">
                    <div className="flex items-center gap-2">
                      {obj.is_dir
                        ? <FolderOpen size={16} className="text-yellow-500 flex-shrink-0" />
                        : <File size={16} className="text-blue-400 flex-shrink-0" />
                      }
                      {obj.is_dir ? (
                        <button
                          onClick={() => navigateInto(obj.key)}
                          className="text-blue-600 hover:underline font-medium truncate max-w-xs"
                        >
                          {obj.key.replace(prefix, '').replace(/\/$/, '')}
                        </button>
                      ) : (
                        <span className="text-gray-800 truncate max-w-xs">
                          {obj.key.replace(prefix, '')}
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="px-3 py-3 text-right text-gray-500">
                    {obj.is_dir ? '—' : formatBytes(obj.size)}
                  </td>
                  <td className="px-3 py-3 text-gray-500 hidden lg:table-cell">
                    {obj.is_dir ? '—' : formatDate(obj.last_modified)}
                  </td>
                  <td className="px-3 py-3 text-gray-500 hidden xl:table-cell text-xs font-mono">
                    {obj.is_dir ? '—' : (obj.content_type || 'application/octet-stream')}
                  </td>
                  <td className="px-3 py-3">
                    {!obj.is_dir && (
                      <div className="flex items-center gap-1 justify-end">
                        <a
                          href={api.downloadUrl(bucket, obj.key)}
                          download
                          className="p-1.5 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-blue-600"
                          title="Download"
                        >
                          <Download size={15} />
                        </a>
                        <button
                          onClick={() => handleDelete(obj.key)}
                          className="p-1.5 rounded-lg hover:bg-gray-100 text-gray-400 hover:text-red-600"
                          title="Delete"
                        >
                          <Trash2 size={15} />
                        </button>
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Upload modal */}
      {showUploadModal && (
        <Modal title="Upload Files" onClose={() => setShowUploadModal(false)}>
          <div className="space-y-4">
            <p className="text-sm text-gray-500">
              Select files to upload to <strong>{bucket}/{prefix}</strong>.
              You can also drag & drop files directly onto the table.
            </p>
            <div
              className="border-2 border-dashed border-gray-200 rounded-xl p-8 text-center cursor-pointer hover:border-blue-400 hover:bg-blue-50 transition-colors"
              onClick={() => fileInputRef.current?.click()}
            >
              <Upload size={32} className="mx-auto text-gray-300 mb-2" />
              <p className="text-gray-500 text-sm">Click to select files, or drag & drop</p>
            </div>
            <input
              ref={fileInputRef}
              type="file"
              multiple
              className="hidden"
              onChange={e => handleUpload(e.target.files)}
            />
            {uploading && <p className="text-blue-600 text-sm text-center animate-pulse">Uploading…</p>}
          </div>
        </Modal>
      )}
    </div>
  );
}
