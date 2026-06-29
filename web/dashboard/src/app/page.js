'use client';

import Link from 'next/link';
import { useSessions } from '@/hooks/useSessions';
import { useRuns } from '@/hooks/useRuns';
import SessionCard from '@/components/SessionCard';
import RunCard from '@/components/RunCard';
import StatusBadge from '@/components/StatusBadge';
import { Activity, Terminal, CheckCircle2, ChevronRight } from 'lucide-react';

export default function DashboardHome() {
  const { sessions, isLoading: loadingSessions } = useSessions();
  const { runs, isLoading: loadingRuns } = useRuns();

  const activeCount = sessions?.length || 0;
  const totalRuns = runs?.length || 0;

  return (
    <div className="p-6 md:p-8 max-w-7xl mx-auto w-full">
      <div className="mb-8">
        <h1 className="text-2xl md:text-3xl font-medium tracking-tight mb-2">Welcome to AgentSandbox</h1>
        <p className="text-muted text-sm md:text-base">Monitor and control your AI agents in real-time.</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 md:gap-6 mb-10">
        <div className="bg-surface border border-border rounded-md p-6 relative overflow-hidden group">
          <div className="absolute top-0 right-0 p-4 opacity-10 text-primary transition-transform group-hover:scale-110">
            <Activity size={48} strokeWidth={1.5} />
          </div>
          <h3 className="text-xs uppercase tracking-wider text-muted font-medium mb-2">Active Sessions</h3>
          <div className="text-3xl font-medium font-mono tracking-tight">{loadingSessions ? '-' : activeCount}</div>
        </div>
        
        <div className="bg-surface border border-border rounded-md p-6 relative overflow-hidden group">
          <div className="absolute top-0 right-0 p-4 opacity-10 text-foreground transition-transform group-hover:scale-110">
            <Terminal size={48} strokeWidth={1.5} />
          </div>
          <h3 className="text-xs uppercase tracking-wider text-muted font-medium mb-2">Total Runs</h3>
          <div className="text-3xl font-medium font-mono tracking-tight">{loadingRuns ? '-' : totalRuns}</div>
        </div>
        
        <div className="bg-surface border border-border rounded-md p-6 relative overflow-hidden group">
          <div className="absolute top-0 right-0 p-4 opacity-10 text-success transition-transform group-hover:scale-110">
            <CheckCircle2 size={48} strokeWidth={1.5} />
          </div>
          <h3 className="text-xs uppercase tracking-wider text-muted font-medium mb-2">System Status</h3>
          <div className="mt-3">
            <StatusBadge status="running" />
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 md:gap-8">
        <div>
          <div className="flex items-center justify-between mb-4 pb-2 border-b border-border">
            <h2 className="text-lg font-medium tracking-tight">Recent Sessions</h2>
            <Link href="/sessions" className="text-xs font-medium text-muted hover:text-primary transition-colors flex items-center">
              View All <ChevronRight size={14} className="ml-0.5" />
            </Link>
          </div>
          <div className="space-y-3">
            {loadingSessions ? (
              <div className="h-24 rounded-md bg-surface animate-pulse border border-border" />
            ) : sessions?.length > 0 ? (
              sessions.slice(0, 3).map((s) => (
                <SessionCard key={s.id} session={s} onTerminate={() => {}} />
              ))
            ) : (
              <div className="p-8 text-center bg-surface border border-border rounded-md text-muted text-sm">
                No active sessions
              </div>
            )}
          </div>
        </div>

        <div>
          <div className="flex items-center justify-between mb-4 pb-2 border-b border-border">
            <h2 className="text-lg font-medium tracking-tight">Recent Runs</h2>
            <Link href="/runs" className="text-xs font-medium text-muted hover:text-primary transition-colors flex items-center">
              View All <ChevronRight size={14} className="ml-0.5" />
            </Link>
          </div>
          <div className="space-y-3">
            {loadingRuns ? (
              <div className="h-24 rounded-md bg-surface animate-pulse border border-border" />
            ) : runs?.length > 0 ? (
              runs.slice(0, 3).map((r, i) => (
                <RunCard key={r.id} run={r} index={i} />
              ))
            ) : (
              <div className="p-8 text-center bg-surface border border-border rounded-md text-muted text-sm">
                No recent runs
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
