'use client';

import Link from 'next/link';
import { motion } from 'framer-motion';
import StatusBadge from '@/components/StatusBadge';
import { formatRelativeTime, truncate, getBackendIcon, getBackendColor, cn } from '@/lib/utils';
import { Play, X } from 'lucide-react';

export default function SessionCard({ session, onTerminate, index = 0 }) {
  const {
    id,
    backend,
    status = 'active',
    created_at,
    expires_at,
  } = session;

  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3, delay: index * 0.05 }}
      className="bg-surface border border-border rounded-md p-5 card-interactive"
    >
      {/* Header */}
      <div className="flex items-start justify-between mb-4">
        <div className="flex items-center gap-2">
          {status === 'active' || status === 'running' ? (
            <div className="w-2 h-2 rounded-full bg-success animate-pulse-dot" />
          ) : null}
          <StatusBadge status={status} size="sm" />
        </div>
        <span className={cn('inline-flex items-center gap-1.5 text-xs px-2.5 py-1 rounded-full font-medium tracking-tight', getBackendColor(backend))}>
          <span className="opacity-80">{getBackendIcon(backend)}</span>
          <span className="uppercase">{backend}</span>
        </span>
      </div>

      {/* Session ID */}
      <div className="mb-5">
        <p className="text-[10px] uppercase tracking-widest text-muted-foreground mb-1">Session ID</p>
        <p className="font-mono text-sm text-foreground tracking-tight">{truncate(id, 24)}</p>
      </div>

      {/* Meta */}
      <div className="grid grid-cols-2 gap-3 mb-6 text-xs">
        <div>
          <p className="text-muted-foreground mb-1 tracking-wide">Created</p>
          <p className="text-muted font-medium">{formatRelativeTime(created_at)}</p>
        </div>
        <div>
          <p className="text-muted-foreground mb-1 tracking-wide">Expires</p>
          <p className="text-muted font-medium">{expires_at ? formatRelativeTime(expires_at) : '—'}</p>
        </div>
      </div>

      {/* Actions */}
      <div className="flex items-center gap-2">
        <Link
          href={`/sessions/${id}`}
          className="flex-1 inline-flex items-center justify-center gap-2 bg-primary/10 hover:bg-primary/20 text-primary text-sm font-medium py-2 px-4 rounded-md transition-colors"
        >
          <Play size={14} className="fill-primary" />
          Watch Live
        </Link>
        {onTerminate && (
          <button
            onClick={() => onTerminate(id)}
            className="inline-flex items-center justify-center bg-error-muted hover:bg-error/20 text-error text-sm font-medium py-2 px-3 rounded-md transition-colors"
          >
            <X size={16} />
          </button>
        )}
      </div>
    </motion.div>
  );
}
