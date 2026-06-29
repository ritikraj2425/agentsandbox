'use client';

export default function Timeline({ children }) {
  return (
    <div className="relative pl-8">
      {/* Vertical line */}
      <div className="absolute left-3 top-0 bottom-0 w-px bg-border" />
      <div className="space-y-4">
        {children}
      </div>
    </div>
  );
}
