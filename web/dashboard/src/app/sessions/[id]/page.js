'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import * as api from '@/lib/api';
import VNCViewer from '@/components/VNCViewer';
import EventLog from '@/components/EventLog';
import StatusBadge from '@/components/StatusBadge';
import { useVNC } from '@/hooks/useVNC';
import { createEventSocket } from '@/lib/websocket';
import { formatRelativeTime } from '@/lib/utils';
import { ChevronLeft, MonitorOff } from 'lucide-react';
import { useParams } from 'next/navigation';

export default function LiveSessionPage() {
  const params = useParams();
  const id = params.id;
  const [session, setSession] = useState(null);
  const [events, setEvents] = useState([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  // Load session meta
  useEffect(() => {
    async function load() {
      try {
        const sessions = await api.getSessions();
        const found = sessions.find((s) => s.id === id);
        if (found) {
          setSession(found);
        } else {
          setError('Session not found or expired');
        }
      } catch (err) {
        setError(err.message);
      } finally {
        setIsLoading(false);
      }
    }
    load();
  }, [id]);

  // Connect event stream
  useEffect(() => {
    const ws = createEventSocket(id);
    ws.onMessage((event) => {
      setEvents((prev) => [...prev, event]);
    });
    return () => ws.close();
  }, [id]);

  if (isLoading) return <div className="p-6 text-muted">Loading session...</div>;
  if (error) return <div className="p-6 text-error">{error}</div>;
  if (!session) return <div className="p-6 text-muted">Session not found</div>;

  return (
    <div className="h-full flex flex-col p-4 md:p-6 max-h-screen w-full">
      {/* Header bar */}
      <div className="flex flex-col md:flex-row items-start md:items-center justify-between mb-4 bg-surface border border-border p-4 rounded-md shrink-0 gap-3">
        <div>
          <Link href="/sessions" className="text-muted hover:text-foreground text-xs font-medium mb-2 inline-flex items-center transition-colors">
            <ChevronLeft size={14} className="mr-0.5" /> Back to Sessions
          </Link>
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-lg font-medium tracking-tight flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-success animate-pulse" />
              Live: <span className="font-mono text-primary text-base">{session.id.substring(0, 12)}</span>
            </h1>
            <span className="text-[10px] uppercase tracking-wider px-2 py-0.5 bg-surface-hover border border-border rounded-md text-muted">{session.backend}</span>
          </div>
        </div>
        <div className="text-xs text-muted font-medium">
          Started {formatRelativeTime(session.created_at)}
        </div>
      </div>

      {/* Main view */}
      <div className="flex-1 min-h-0 flex flex-col gap-4">

        {/* Event Log */}
        <div className="w-full flex-1 bg-surface border border-border rounded-md flex flex-col overflow-hidden">
          <div className="p-3 border-b border-border bg-black/10 shrink-0">
            <h2 className="text-xs uppercase tracking-wider font-medium text-muted">Live Event Stream</h2>
          </div>
          <div className="flex-1 min-h-0 p-3 overflow-y-auto">
            <EventLog events={events} />
          </div>
        </div>
      </div>
    </div>
  );
}
