'use client';

import { useEffect, useRef } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { cn } from '@/lib/utils';
import { Loader2 } from 'lucide-react';

export default function EventLog({ events = [], className }) {
  const scrollRef = useRef(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [events]);

  const typeColors = {
    'action.received': 'text-primary border-primary/30',
    'policy.check': 'text-success border-success/30',
    'policy.denied': 'text-error border-error/30',
    'process.started': 'text-foreground border-border',
    'process.finished': 'text-foreground border-border',
    output: 'text-info border-info/30',
    error: 'text-error border-error/30',
  };

  const typeBg = {
    'action.received': 'bg-primary-muted',
    'policy.check': 'bg-success-muted',
    'policy.denied': 'bg-error-muted',
    'process.started': 'bg-surface-hover',
    'process.finished': 'bg-surface-hover',
    output: 'bg-info-muted',
    error: 'bg-error-muted',
  };

  return (
    <div className={cn('flex flex-col h-full bg-background/50 rounded-md overflow-hidden', className)}>
      <div ref={scrollRef} className="flex-1 overflow-y-auto p-2 space-y-1">
        <AnimatePresence initial={false}>
          {events.map((event, i) => {
            const time = event.timestamp
              ? new Date(event.timestamp).toLocaleTimeString('en-US', {
                  hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit',
                })
              : '';

            const typeLabel = (event.type || 'event').split('.').pop();
            const color = typeColors[event.type] || 'text-muted border-border';
            const bg = typeBg[event.type] || 'bg-surface-hover';

            return (
              <motion.div
                key={event.id || i}
                initial={{ opacity: 0, y: 5 }}
                animate={{ opacity: 1, y: 0 }}
                className="flex items-start gap-2.5 px-2.5 py-2 rounded-md hover:bg-surface transition-colors text-xs font-mono group border border-transparent hover:border-border"
              >
                <span className="text-muted-foreground shrink-0 pt-0.5 w-16">{time}</span>
                <span className={cn('shrink-0 px-1.5 py-0.5 rounded uppercase tracking-wider font-semibold text-[10px] border', bg, color)}>
                  {typeLabel}
                </span>
                <span className="text-foreground tracking-tight break-all">
                  {event.command || event.message || event.output?.slice(0, 120) || '—'}
                </span>
              </motion.div>
            );
          })}
        </AnimatePresence>
        {events.length === 0 && (
          <div className="flex flex-col items-center justify-center h-full text-muted gap-3">
            <Loader2 size={24} className="animate-spin opacity-50" />
            <span className="text-xs uppercase tracking-widest font-medium">Waiting for events</span>
          </div>
        )}
      </div>
    </div>
  );
}
