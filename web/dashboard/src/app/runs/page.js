'use client';

import { useRuns } from '@/hooks/useRuns';
import RunCard from '@/components/RunCard';
import { History } from 'lucide-react';

export default function RunsPage() {
  const { runs, isLoading, error } = useRuns();

  const allRuns = runs || [];

  return (
    <div className="p-6 md:p-8 max-w-7xl mx-auto w-full">
      <div className="mb-8">
        <h1 className="text-2xl font-medium tracking-tight mb-1">Run History</h1>
        <p className="text-muted text-sm">Review past execution traces and replays</p>
      </div>

      {error && (
        <div className="bg-error-muted border border-error/20 text-error p-4 rounded-md mb-6 text-sm">
          Failed to load runs: {error}
        </div>
      )}

      {isLoading && !runs ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 md:gap-6">
          {[1, 2, 3, 4, 5, 6].map((i) => (
            <div key={i} className="h-32 rounded-md bg-surface animate-pulse border border-border" />
          ))}
        </div>
      ) : allRuns.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 bg-surface border border-border rounded-md">
          <History size={48} className="text-muted mb-4 opacity-50" strokeWidth={1.5} />
          <h3 className="text-lg font-medium mb-1 tracking-tight">No run history</h3>
          <p className="text-muted text-sm">Completed runs will appear here for review.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 md:gap-6">
          {allRuns.map((run, i) => (
            <RunCard key={run.id} run={run} index={i} />
          ))}
        </div>
      )}
    </div>
  );
}
