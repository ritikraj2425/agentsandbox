'use client';

import { useState, useCallback } from 'react';
import { cn } from '@/lib/utils';
import { Copy, Check } from 'lucide-react';

export default function CodeBlock({ children, language, showLineNumbers = false, maxHeight = '300px', className }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(children || '');
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // fallback
    }
  }, [children]);

  const lines = (children || '').split('\n');

  return (
    <div className={cn('relative group rounded-md border border-border overflow-hidden bg-black/40', className)}>
      {/* Header bar */}
      {language && (
        <div className="flex items-center justify-between px-3 py-1.5 bg-surface-hover/50 border-b border-border">
          <span className="text-[10px] uppercase tracking-wider text-muted-foreground font-semibold">{language}</span>
        </div>
      )}

      {/* Copy button */}
      <button
        onClick={handleCopy}
        className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity bg-surface border border-border rounded-md p-1.5 text-muted hover:text-foreground hover:bg-surface-hover z-10 flex items-center justify-center shadow-sm"
      >
        {copied ? <Check size={14} className="text-success" /> : <Copy size={14} />}
      </button>

      {/* Code content */}
      <div className="overflow-auto" style={{ maxHeight }}>
        <pre className="p-3 md:p-4 text-xs font-mono leading-relaxed">
          {showLineNumbers ? (
            lines.map((line, i) => (
              <div key={i} className="flex">
                <span className="select-none text-muted-foreground/40 w-8 shrink-0 text-right pr-4">
                  {i + 1}
                </span>
                <code className="text-foreground/90">{line}</code>
              </div>
            ))
          ) : (
            <code className="text-foreground/90">{children}</code>
          )}
        </pre>
      </div>
    </div>
  );
}
