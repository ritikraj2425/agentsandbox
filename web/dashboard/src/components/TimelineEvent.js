'use client';

import { useState } from 'react';
import { motion } from 'framer-motion';
import CodeBlock from '@/components/CodeBlock';
import { cn } from '@/lib/utils';
import { ChevronDown, ChevronRight } from 'lucide-react';

const typeConfig = {
  'action.received': {
    color: 'border-l-primary bg-primary-muted/20',
    dot: 'bg-primary ring-primary/20',
    label: 'Action',
    labelClass: 'bg-primary-muted text-primary border border-primary/20',
  },
  'policy.check': {
    color: 'border-l-success bg-success-muted/20',
    dot: 'bg-success ring-success/20',
    label: 'Policy',
    labelClass: 'bg-success-muted text-success border border-success/20',
  },
  'policy.denied': {
    color: 'border-l-error bg-error-muted/20',
    dot: 'bg-error ring-error/20',
    label: 'Denied',
    labelClass: 'bg-error-muted text-error border border-error/20',
  },
  'process.started': {
    color: 'border-l-muted',
    dot: 'bg-muted ring-muted/20',
    label: 'Started',
    labelClass: 'bg-surface text-muted border border-border',
  },
  'process.finished': {
    color: 'border-l-muted',
    dot: 'bg-muted ring-muted/20',
    label: 'Finished',
    labelClass: 'bg-surface text-muted border border-border',
  },
  output: {
    color: 'border-l-info bg-info-muted/10',
    dot: 'bg-info ring-info/20',
    label: 'Output',
    labelClass: 'bg-info-muted text-info border border-info/20',
  },
  error: {
    color: 'border-l-error bg-error-muted/10',
    dot: 'bg-error ring-error/20',
    label: 'Error',
    labelClass: 'bg-error-muted text-error border border-error/20',
  },
};

function getConfig(type) {
  return typeConfig[type] || typeConfig['process.started'];
}

export default function TimelineEvent({ event, isActive = false, index = 0 }) {
  const [expanded, setExpanded] = useState(false);
  const config = getConfig(event.type);

  const timestamp = event.timestamp
    ? new Date(event.timestamp).toLocaleTimeString('en-US', {
        hour12: false,
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        fractionalSecondDigits: 3,
      })
    : '';

  const hasLongContent =
    (event.output && event.output.length > 200) ||
    (event.stderr && event.stderr.length > 200);

  return (
    <motion.div
      initial={{ opacity: 0, x: -10 }}
      animate={{ opacity: 1, x: 0 }}
      transition={{ duration: 0.3, delay: index * 0.03 }}
      className="relative"
    >
      {/* Dot */}
      <div
        className={cn(
          'absolute -left-8 top-5 w-2 h-2 rounded-full ring-4 z-10',
          config.dot,
          isActive && 'ring-primary/40 scale-125'
        )}
      />

      {/* Card */}
      <div
        className={cn(
          'border border-border rounded-md border-l-2 overflow-hidden transition-all',
          config.color,
          isActive && 'ring-1 ring-primary/40 border-primary shadow-sm shadow-primary/10'
        )}
      >
        {/* Header */}
        <div
          className="flex flex-col md:flex-row items-start md:items-center justify-between px-3 md:px-4 py-2.5 cursor-pointer hover:bg-surface/50 transition-colors gap-3"
          onClick={() => hasLongContent && setExpanded(!expanded)}
        >
          <div className="flex items-center gap-3 min-w-0 w-full md:w-auto">
            <span className={cn('text-[10px] px-1.5 py-0.5 rounded font-semibold uppercase tracking-wider shrink-0', config.labelClass)}>
              {config.label}
            </span>
            {event.command && (
              <span className="text-xs font-mono text-foreground truncate max-w-full md:max-w-md bg-background/50 px-1.5 py-0.5 rounded border border-border">
                {event.command}
              </span>
            )}
            {event.message && (
              <span className="text-sm font-medium text-foreground tracking-tight truncate max-w-full md:max-w-md">{event.message}</span>
            )}
            {event.exit_code != null && (
              <span className={cn('text-[10px] font-mono px-1.5 py-0.5 rounded border', event.exit_code === 0 ? 'text-success border-success/20 bg-success-muted' : 'text-error border-error/20 bg-error-muted')}>
                exit {event.exit_code}
              </span>
            )}
            {event.duration && (
              <span className="text-xs text-muted-foreground font-mono">{event.duration}</span>
            )}
          </div>
          <div className="flex items-center justify-between w-full md:w-auto md:justify-end gap-3 self-stretch md:self-auto shrink-0">
            <span className="text-[10px] font-mono text-muted-foreground">{timestamp}</span>
            {hasLongContent && (
              <span className="text-muted p-1 hover:text-foreground hover:bg-surface rounded-md transition-colors">
                {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
              </span>
            )}
          </div>
        </div>

        {/* Expanded content */}
        {(expanded || !hasLongContent) && (event.output || event.stderr || event.screenshot) && (
          <div className="px-3 md:px-4 pb-3 pt-1 space-y-3">
            {event.output && (
              <CodeBlock maxHeight="250px" language="stdout">
                {event.output}
              </CodeBlock>
            )}
            {event.stderr && (
              <CodeBlock maxHeight="250px" language="stderr">
                {event.stderr}
              </CodeBlock>
            )}
            {event.screenshot && (
              <div className="rounded-md overflow-hidden border border-border mt-3 shadow-md shadow-black/20">
                <img
                  src={`data:image/png;base64,${event.screenshot}`}
                  alt="Screenshot"
                  className="w-full h-auto max-h-[400px] object-contain bg-background"
                />
              </div>
            )}
          </div>
        )}
      </div>
    </motion.div>
  );
}
