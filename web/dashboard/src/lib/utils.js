import { Laptop, Container, Globe, Shield, Flame, Box } from 'lucide-react';
import React from 'react';

export function formatDate(date) {
  const d = new Date(date);
  return d.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function formatDuration(ms) {
  if (ms < 1000) return `${Math.round(ms)}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  const minutes = Math.floor(ms / 60000);
  const seconds = Math.round((ms % 60000) / 1000);
  return `${minutes}m ${seconds}s`;
}

export function formatRelativeTime(date) {
  const now = Date.now();
  const d = new Date(date).getTime();
  const diff = now - d;

  if (diff < 0) {
    const absDiff = Math.abs(diff);
    if (absDiff < 60000) return `in ${Math.round(absDiff / 1000)}s`;
    if (absDiff < 3600000) return `in ${Math.round(absDiff / 60000)}m`;
    if (absDiff < 86400000) return `in ${Math.round(absDiff / 3600000)}h`;
    return `in ${Math.round(absDiff / 86400000)}d`;
  }

  if (diff < 5000) return 'just now';
  if (diff < 60000) return `${Math.round(diff / 1000)}s ago`;
  if (diff < 3600000) return `${Math.round(diff / 60000)}m ago`;
  if (diff < 86400000) return `${Math.round(diff / 3600000)}h ago`;
  return `${Math.round(diff / 86400000)}d ago`;
}

export function truncate(str, len = 20) {
  if (!str) return '';
  if (str.length <= len) return str;
  return str.slice(0, len) + '…';
}

export function getStatusColor(status) {
  const map = {
    active: 'text-success',
    running: 'text-primary',
    completed: 'text-success',
    success: 'text-success',
    failed: 'text-error',
    error: 'text-error',
    denied: 'text-warning',
    timeout: 'text-muted',
    pending: 'text-muted',
    stopped: 'text-muted',
  };
  return map[status?.toLowerCase()] || 'text-muted';
}

export function getStatusBg(status) {
  const map = {
    active: 'bg-success-muted text-success border border-success/20',
    running: 'bg-primary-muted text-primary border border-primary/20',
    completed: 'bg-success-muted text-success border border-success/20',
    success: 'bg-success-muted text-success border border-success/20',
    failed: 'bg-error-muted text-error border border-error/20',
    error: 'bg-error-muted text-error border border-error/20',
    denied: 'bg-warning-muted text-warning border border-warning/20',
    timeout: 'bg-surface text-muted border border-border',
    pending: 'bg-surface text-muted border border-border',
    stopped: 'bg-surface text-muted border border-border',
  };
  return map[status?.toLowerCase()] || 'bg-surface text-muted border border-border';
}

export function getBackendIcon(backend) {
  const map = {
    local: <Laptop size={14} />,
    docker: <Container size={14} />,
    browser: <Globe size={14} />,
    gvisor: <Shield size={14} />,
    firecracker: <Flame size={14} />,
  };
  return map[backend?.toLowerCase()] || <Box size={14} />;
}

export function getBackendColor(backend) {
  const map = {
    local: 'bg-surface text-muted-foreground border border-border',
    docker: 'bg-info-muted text-info border border-info/20',
    browser: 'bg-primary-muted text-primary border border-primary/20',
    gvisor: 'bg-success-muted text-success border border-success/20',
    firecracker: 'bg-warning-muted text-warning border border-warning/20',
  };
  return map[backend?.toLowerCase()] || 'bg-surface text-muted-foreground border border-border';
}

export function cn(...classes) {
  return classes.filter(Boolean).join(' ');
}
