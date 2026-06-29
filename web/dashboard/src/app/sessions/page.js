'use client';

import { useSessions } from '@/hooks/useSessions';
import SessionCard from '@/components/SessionCard';
import { useState } from 'react';
import { RefreshCw, ServerOff } from 'lucide-react';

export default function SessionsPage() {
  const { sessions, isLoading, error, refresh } = useSessions();
  const [isRefreshing, setIsRefreshing] = useState(false);

  const activeSessions = sessions || [];

  const handleRefresh = async () => {
    setIsRefreshing(true);
    await refresh();
    setTimeout(() => setIsRefreshing(false), 500);
  };

  return (
    <div className="p-6 md:p-8 max-w-7xl mx-auto w-full">
      <div className="flex flex-col md:flex-row md:items-center justify-between mb-8 gap-4">
        <div>
          <h1 className="text-2xl font-medium tracking-tight mb-1">Active Sessions</h1>
          <p className="text-muted text-sm">Manage running sandboxes and live view agents</p>
        </div>
        <button
          className="bg-surface hover:bg-surface-hover border border-border text-foreground px-4 py-2 rounded-md text-sm font-medium transition-colors flex items-center justify-center gap-2 w-full md:w-auto"
          onClick={handleRefresh}
          disabled={isRefreshing}
        >
          <RefreshCw size={14} className={isRefreshing ? 'animate-spin' : ''} />
          Refresh
        </button>
      </div>

      {error && (
        <div className="bg-error-muted border border-error/20 text-error p-4 rounded-md mb-6 text-sm">
          Failed to load sessions: {error}
        </div>
      )}

      {isLoading && !sessions ? (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-40 rounded-md bg-surface animate-pulse border border-border" />
          ))}
        </div>
      ) : activeSessions.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 bg-surface border border-border rounded-md">
          <ServerOff size={48} className="text-muted mb-4 opacity-50" strokeWidth={1.5} />
          <h3 className="text-lg font-medium mb-1 tracking-tight">No active sessions</h3>
          <p className="text-muted text-sm">There are no running sandboxes right now.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 md:gap-6">
          {activeSessions.map((session) => (
            <SessionCard
              key={session.id}
              session={session}
              onTerminate={() => {}}
            />
          ))}
        </div>
      )}
    </div>
  );
}
