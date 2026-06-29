'use client';

import { usePathname } from 'next/navigation';
import { useAuth } from '@/lib/auth';
import { LogOut, User } from 'lucide-react';

const pathLabels = {
  '/': 'Dashboard',
  '/sessions': 'Sessions',
  '/runs': 'Runs',
  '/settings': 'Settings',
};

export default function Header() {
  const pathname = usePathname();
  const { logout } = useAuth();

  const segments = pathname.split('/').filter(Boolean);
  const breadcrumbs = segments.length === 0
    ? [{ label: 'Dashboard', href: '/' }]
    : segments.map((seg, i) => {
        const href = '/' + segments.slice(0, i + 1).join('/');
        return {
          label: pathLabels[href] || seg,
          href,
          isLast: i === segments.length - 1,
        };
      });

  return (
    <header className="h-14 border-b border-border bg-surface/50 backdrop-blur-sm flex items-center justify-between px-4 md:px-6 sticky top-0 z-30">
      {/* Breadcrumb */}
      <nav className="flex items-center gap-2 text-sm font-medium tracking-tight overflow-x-auto whitespace-nowrap hide-scrollbar">
        <span className="text-muted-foreground hidden sm:inline-block">AgentSandbox</span>
        {breadcrumbs.map((crumb, i) => (
          <span key={i} className="flex items-center gap-2">
            <span className="text-muted-foreground/50 hidden sm:inline-block">/</span>
            <span className={crumb.isLast || breadcrumbs.length === 1 ? 'text-foreground' : 'text-muted'}>
              {crumb.label}
            </span>
          </span>
        ))}
      </nav>

      {/* User section */}
      <div className="flex items-center gap-2 md:gap-3 shrink-0 ml-4">
        <div className="w-8 h-8 rounded-md bg-surface border border-border flex items-center justify-center text-primary">
          <User size={16} />
        </div>
        <button
          onClick={logout}
          className="flex items-center gap-1.5 text-sm font-medium text-muted hover:text-foreground transition-colors px-2 py-1.5 rounded-md hover:bg-surface-hover"
        >
          <span className="hidden sm:inline-block">Logout</span>
          <LogOut size={16} className="sm:hidden" />
        </button>
      </div>
    </header>
  );
}
