'use client';

import dynamic from 'next/dynamic';

const VNCViewerInner = dynamic(
  () => import('@/components/VNCViewerInner'),
  {
    ssr: false,
    loading: () => (
      <div className="w-full h-full flex items-center justify-center bg-background">
        <div className="text-center">
          <div className="w-8 h-8 border-2 border-primary border-t-transparent rounded-full animate-spin mx-auto mb-3" />
          <p className="text-sm text-muted">Loading VNC viewer…</p>
        </div>
      </div>
    ),
  }
);

export default function VNCViewer(props) {
  return <VNCViewerInner {...props} />;
}
