'use client';

import { cn, getStatusBg } from '@/lib/utils';

const sizeClasses = {
  sm: 'text-xs px-2 py-0.5',
  md: 'text-xs px-2.5 py-1',
  lg: 'text-sm px-3 py-1',
};

export default function StatusBadge({ status, size = 'md', className }) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full font-medium capitalize',
        sizeClasses[size],
        getStatusBg(status),
        className
      )}
    >
      {status}
    </span>
  );
}
