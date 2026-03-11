import { Database, BarChart3, Server } from 'lucide-react';

export function Sidebar({ activePage, onNavigate }) {
  const items = [
    { id: 'dashboard', label: 'Dashboard', icon: BarChart3 },
    { id: 'buckets', label: 'Buckets', icon: Database },
    { id: 'about', label: 'Server Info', icon: Server },
  ];

  return (
    <aside className="w-60 min-h-screen bg-gray-900 flex flex-col">
      {/* Logo */}
      <div className="px-6 py-5 border-b border-gray-700">
        <div className="flex items-center gap-2">
          <div className="w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center">
            <Database size={18} className="text-white" />
          </div>
          <span className="text-white font-bold text-lg">Opens3</span>
        </div>
        <p className="text-gray-400 text-xs mt-1">Object Storage</p>
      </div>

      {/* Navigation */}
      <nav className="flex-1 px-3 py-4 space-y-1">
        {items.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            onClick={() => onNavigate(id)}
            className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-sm font-medium transition-colors ${
              activePage === id
                ? 'bg-blue-600 text-white'
                : 'text-gray-400 hover:bg-gray-800 hover:text-white'
            }`}
          >
            <Icon size={18} />
            {label}
          </button>
        ))}
      </nav>

      {/* Footer */}
      <div className="px-4 py-4 border-t border-gray-700">
        <p className="text-gray-500 text-xs text-center">Opens3 v1.0.0</p>
      </div>
    </aside>
  );
}
