'use client';

import { useState, useEffect, useCallback } from 'react';
import { getSessionVNC } from '@/lib/api';

export function useVNC(sessionId) {
  const [vncUrl, setVncUrl] = useState(null);
  const [isConnected, setIsConnected] = useState(false);
  const [error, setError] = useState(null);

  const fetchUrl = useCallback(async () => {
    if (!sessionId) return;
    try {
      const data = await getSessionVNC(sessionId);
      setVncUrl(data?.url || data?.vnc_url || null);
      setError(null);
    } catch (err) {
      setError(err.message);
    }
  }, [sessionId]);

  useEffect(() => {
    fetchUrl();
  }, [fetchUrl]);

  const handleStatusChange = useCallback((status) => {
    setIsConnected(status === 'connected');
  }, []);

  const connect = useCallback(() => {
    fetchUrl();
  }, [fetchUrl]);

  const disconnect = useCallback(() => {
    setVncUrl(null);
    setIsConnected(false);
  }, []);

  return { vncUrl, isConnected, error, connect, disconnect, handleStatusChange };
}
