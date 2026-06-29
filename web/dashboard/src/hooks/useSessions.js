'use client';

import { useState, useEffect, useCallback } from 'react';
import { getSessions } from '@/lib/api';

export function useSessions(refreshInterval = 5000) {
  const [sessions, setSessions] = useState([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState(null);

  const refresh = useCallback(async () => {
    try {
      const data = await getSessions();
      setSessions(Array.isArray(data) ? data : data?.sessions || []);
      setError(null);
    } catch (err) {
      if (err.message !== 'Unauthorized') {
        setError(err.message);
      }
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
    const interval = setInterval(refresh, refreshInterval);
    return () => clearInterval(interval);
  }, [refresh, refreshInterval]);

  return { sessions, isLoading, error, refresh };
}
