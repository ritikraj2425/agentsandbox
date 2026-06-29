'use client';

import Link from 'next/link';
import { motion } from 'framer-motion';
import StatusBadge from '@/components/StatusBadge';
import { formatDate, formatDuration, truncate } from '@/lib/utils';
import { Clock, Layers, Calendar } from 'lucide-react';

export default function RunCard({ run, index = 0 }) {
  const {
    id,
    created_at,
    duration,
    event_count,
    status = 'completed',
  } = run;

  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3, delay: index * 0.05 }}
    >
      <Link
        href={`/runs/${id}`}
        className="block bg-surface border border-border rounded-md p-4 card-interactive"
      >
        <div className="flex items-center justify-between mb-4">
          <span className="font-mono text-sm text-foreground tracking-tight">{truncate(id, 16)}</span>
          <StatusBadge status={status} size="sm" />
        </div>
        <div className="flex items-center gap-4 text-xs text-muted">
          <div className="flex items-center gap-1.5">
            <Calendar size={12} className="opacity-70" />
            <span>{formatDate(created_at)}</span>
          </div>
          {duration != null && (
            <div className="flex items-center gap-1.5">
              <Clock size={12} className="opacity-70" />
              <span>{formatDuration(duration)}</span>
            </div>
          )}
          {event_count != null && (
            <div className="flex items-center gap-1.5">
              <Layers size={12} className="opacity-70" />
              <span>{event_count} events</span>
            </div>
          )}
        </div>
      </Link>
    </motion.div>
  );
}
