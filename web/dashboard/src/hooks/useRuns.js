'use client';

import { useState, useEffect, useCallback } from 'react';
import { getRuns } from '@/lib/api';

export function useRuns() {
  const [runs, setRuns] = useState([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState(null);

  const refresh = useCallback(async () => {
    try {
      const data = await getRuns();
      setRuns(Array.isArray(data) ? data : data?.runs || []);
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
  }, [refresh]);

  return { runs, isLoading, error, refresh };
}
