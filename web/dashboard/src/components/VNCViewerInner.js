'use client';

import { useEffect, useRef, useState, useCallback } from 'react';
import { MonitorOff, AlertTriangle, Loader2 } from 'lucide-react';

export default function VNCViewerInner({ url, onStatusChange }) {
  const containerRef = useRef(null);
  const rfbRef = useRef(null);
  const [status, setStatus] = useState('disconnected');
  const [error, setError] = useState(null);

  const updateStatus = useCallback((s, err = null) => {
    setStatus(s);
    setError(err);
    onStatusChange?.(s, err);
  }, [onStatusChange]);

  useEffect(() => {
    if (!url || !containerRef.current) return;

    let rfb = null;

    async function initVNC() {
      try {
        updateStatus('connecting');

        // Dynamic import of noVNC
        const { default: RFB } = await import('@novnc/novnc');

        rfb = new RFB(containerRef.current, url, {
          credentials: { password: '' },
          wsProtocols: ['binary'],
        });

        rfb.scaleViewport = true;
        rfb.resizeSession = true;
        rfb.background = '#0A0A0B';

        rfb.addEventListener('connect', () => {
          updateStatus('connected');
        });

        rfb.addEventListener('disconnect', (e) => {
          updateStatus('disconnected', e.detail?.clean ? null : 'Connection lost');
        });

        rfb.addEventListener('securityfailure', (e) => {
          updateStatus('error', `Security failure: ${e.detail?.reason || 'Unknown'}`);
        });

        rfbRef.current = rfb;
      } catch (err) {
        updateStatus('error', err.message);
      }
    }

    initVNC();

    return () => {
      if (rfbRef.current) {
        try {
          rfbRef.current.disconnect();
        } catch {
          // ignore
        }
        rfbRef.current = null;
      }
    };
  }, [url, updateStatus]);

  return (
    <div className="relative w-full h-full">
      {/* Status overlay */}
      {status !== 'connected' && (
        <div className="absolute inset-0 flex items-center justify-center bg-background/80 backdrop-blur-sm z-10">
          <div className="text-center flex flex-col items-center">
            {status === 'connecting' && (
              <>
                <Loader2 size={32} className="animate-spin text-primary mb-3" />
                <p className="text-sm font-medium tracking-tight">Connecting to VNC…</p>
              </>
            )}
            {status === 'disconnected' && (
              <>
                <div className="w-12 h-12 rounded-full bg-surface border border-border flex items-center justify-center mb-3">
                  <MonitorOff size={24} className="text-muted" strokeWidth={1.5} />
                </div>
                <p className="text-sm font-medium tracking-tight">Disconnected</p>
              </>
            )}
            {status === 'error' && (
              <>
                <div className="w-12 h-12 rounded-full bg-error-muted flex items-center justify-center mb-3">
                  <AlertTriangle size={24} className="text-error" strokeWidth={1.5} />
                </div>
                <p className="text-sm font-medium tracking-tight text-error">{error || 'Connection error'}</p>
              </>
            )}
          </div>
        </div>
      )}

      {/* VNC canvas container */}
      <div
        ref={containerRef}
        className="w-full h-full bg-background"
        style={{ minHeight: '400px' }}
      />
    </div>
  );
}
