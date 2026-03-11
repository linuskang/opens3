import { useState, useCallback, useId, useRef } from 'react';
import { Sidebar } from './components/Sidebar.jsx';
import { Dashboard } from './components/Dashboard.jsx';
import { BucketList } from './components/BucketList.jsx';
import { ObjectBrowser } from './components/ObjectBrowser.jsx';
import { About } from './components/About.jsx';
import { ToastContainer } from './components/Toast.jsx';

function App() {
  const [page, setPage] = useState('dashboard');
  const [selectedBucket, setSelectedBucket] = useState(null);
  const [toasts, setToasts] = useState([]);
  const baseId = useId();
  const toastCounter = useRef(0);

  const showToast = useCallback((message, type = 'success') => {
    const id = `${baseId}-${Date.now()}-${toastCounter.current++}`;
    setToasts(prev => [...prev, { id, message, type }]);
  }, [baseId]);

  const removeToast = useCallback((id) => {
    setToasts(prev => prev.filter(t => t.id !== id));
  }, []);

  const handleSelectBucket = (name) => {
    setSelectedBucket(name);
    setPage('browser');
  };

  const handleNavigate = (newPage) => {
    setPage(newPage);
    if (newPage !== 'browser') setSelectedBucket(null);
  };

  return (
    <div className="flex min-h-screen bg-gray-50">
      <Sidebar activePage={selectedBucket ? 'buckets' : page} onNavigate={handleNavigate} />
      <main className="flex-1 p-8 overflow-auto">
        {page === 'dashboard' && <Dashboard />}
        {page === 'buckets' && !selectedBucket && (
          <BucketList onSelectBucket={handleSelectBucket} showToast={showToast} />
        )}
        {page === 'browser' && selectedBucket && (
          <ObjectBrowser
            bucket={selectedBucket}
            onBack={() => { setPage('buckets'); setSelectedBucket(null); }}
            showToast={showToast}
          />
        )}
        {page === 'about' && <About />}
      </main>
      <ToastContainer toasts={toasts} removeToast={removeToast} />
    </div>
  );
}

export default App;
