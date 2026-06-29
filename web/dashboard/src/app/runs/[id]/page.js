'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import * as api from '@/lib/api';
import Timeline from '@/components/Timeline';
import TimelineEvent from '@/components/TimelineEvent';
import CodeBlock from '@/components/CodeBlock';
import StatusBadge from '@/components/StatusBadge';
import { formatDate, formatDuration } from '@/lib/utils';
import { useParams } from 'next/navigation';
import { ChevronLeft, Loader2, Play } from 'lucide-react';

export default function RunReplayPage() {
  const params = useParams();
  const id = params.id;
  const [run, setRun] = useState(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    async function loadRun() {
      try {
        const data = await api.getRun(params.id);
        setRun(data);
      } catch (err) {
        setError(err.message);
      } finally {
        setIsLoading(false);
      }
    }
    loadRun();
  }, [params.id]);

  if (isLoading) {
    return (
      <div className="flex flex-col items-center justify-center p-20 text-muted gap-3">
        <Loader2 size={24} className="animate-spin opacity-50" />
        <span className="text-sm font-medium tracking-tight">Loading replay...</span>
      </div>
    );
  }
  
  if (error) {
    return <div className="p-6 md:p-8 text-error font-medium text-sm">Error: {error}</div>;
  }
  
  if (!run) {
    return <div className="p-6 md:p-8 text-center text-muted font-medium">Run not found</div>;
  }

  const events = run.events || [];

  return (
    <div className="p-6 md:p-8 max-w-7xl mx-auto w-full">
      <div className="mb-6 flex flex-col items-start gap-4">
        <div>
          <Link href="/runs" className="text-muted hover:text-foreground text-xs font-medium mb-3 inline-flex items-center transition-colors">
            <ChevronLeft size={14} className="mr-0.5" /> Back to Runs
          </Link>
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-xl md:text-2xl font-medium tracking-tight flex items-center gap-2">
              <Play size={20} className="text-primary opacity-80" />
              Run <span className="font-mono text-primary text-base md:text-lg tracking-tight">{run.id.substring(0, 12)}</span>
            </h1>
            <StatusBadge status={(run.exit_code === 0 || run.exit_code == null) ? 'completed' : 'failed'} />
          </div>
          <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted mt-3 font-medium">
            <span>{formatDate(run.created_at)}</span>
            <span className="opacity-30">•</span>
            <span>{formatDuration(run.duration_ms)}</span>
            <span className="opacity-30">•</span>
            <span>{events.length} events</span>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2">
          <div className="bg-surface border border-border rounded-md p-5 md:p-6">
            <h2 className="text-lg font-medium tracking-tight mb-6">Execution Timeline</h2>
            
            {events.length === 0 ? (
              <p className="text-muted text-center py-12 text-sm">No events recorded in this run.</p>
            ) : (
              <Timeline>
                {events.map((ev, i) => (
                  <TimelineEvent key={i} event={ev} />
                ))}
              </Timeline>
            )}
          </div>
        </div>

        <div className="space-y-6">
          <div className="bg-surface border border-border rounded-md p-5 md:p-6">
            <h3 className="text-[10px] font-semibold text-muted uppercase tracking-widest mb-4">Outputs</h3>
            
            <div className="space-y-5">
              <div>
                <h4 className="text-xs font-medium mb-2 tracking-tight">Stdout</h4>
                {run.stdout ? (
                  <CodeBlock maxHeight="250px">{run.stdout}</CodeBlock>
                ) : (
                  <div className="text-xs text-muted-foreground italic px-3 py-2 bg-background/50 border border-border rounded-md">No stdout</div>
                )}
              </div>
              
              <div>
                <h4 className="text-xs font-medium mb-2 tracking-tight text-error">Stderr</h4>
                {run.stderr ? (
                  <CodeBlock maxHeight="250px" className="text-error">{run.stderr}</CodeBlock>
                ) : (
                  <div className="text-xs text-muted-foreground italic px-3 py-2 bg-background/50 border border-border rounded-md">No stderr</div>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
