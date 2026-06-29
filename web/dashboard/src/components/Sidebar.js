'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { LayoutDashboard, Activity, Terminal, Settings, ChevronLeft, ChevronRight } from 'lucide-react';
import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { cn } from '@/lib/utils';

const navItems = [
  { href: '/', label: 'Dashboard', icon: LayoutDashboard },
  { href: '/sessions', label: 'Sessions', icon: Activity },
  { href: '/runs', label: 'Runs', icon: Terminal },
  { href: '/settings', label: 'Settings', icon: Settings },
];

export default function Sidebar() {
  const pathname = usePathname();
  const [collapsed, setCollapsed] = useState(false);
  const [isMobile, setIsMobile] = useState(false);

  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
      if (window.innerWidth < 768) {
        setCollapsed(true);
      }
    };
    
    checkMobile();
    window.addEventListener('resize', checkMobile);
    return () => window.removeEventListener('resize', checkMobile);
  }, []);

  function isActive(href) {
    if (href === '/') return pathname === '/';
    return pathname.startsWith(href);
  }

  // Force collapsed on mobile regardless of state
  const isEffectivelyCollapsed = isMobile || collapsed;

  return (
    <motion.aside
      initial={false}
      animate={{ width: isEffectivelyCollapsed ? 64 : 256 }}
      transition={{ duration: 0.2, ease: 'easeInOut' }}
      className="shrink-0 relative h-screen bg-surface border-r border-border flex flex-col z-40"
    >
      {/* Logo */}
      <div className="h-16 flex items-center px-4 border-b border-border shrink-0">
        <Link href="/" className="flex items-center gap-3 min-w-0">
          <div className="w-8 h-8 rounded-md bg-surface border border-border flex items-center justify-center text-primary font-bold text-sm shrink-0">
            AS
          </div>
          <AnimatePresence>
            {!isEffectivelyCollapsed && (
              <motion.span
                initial={{ opacity: 0, width: 0 }}
                animate={{ opacity: 1, width: 'auto' }}
                exit={{ opacity: 0, width: 0 }}
                className="font-medium text-foreground whitespace-nowrap overflow-hidden tracking-tight"
              >
                AgentSandbox
              </motion.span>
            )}
          </AnimatePresence>
        </Link>
      </div>

      {/* Navigation */}
      <nav className="flex-1 py-4 px-3 space-y-1 overflow-y-auto">
        {navItems.map((item) => {
          const active = isActive(item.href);
          const Icon = item.icon;
          return (
            <Link
              key={item.href}
              href={item.href}
              title={isEffectivelyCollapsed ? item.label : undefined}
              className={cn(
                'flex items-center gap-3 px-2.5 py-2.5 rounded-md text-sm font-medium transition-colors relative group',
                active
                  ? 'bg-primary-muted text-primary'
                  : 'text-muted hover:text-foreground hover:bg-surface-hover'
              )}
            >
              <Icon size={18} className="shrink-0" />
              <AnimatePresence>
                {!isEffectivelyCollapsed && (
                  <motion.span
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    exit={{ opacity: 0 }}
                    className="whitespace-nowrap overflow-hidden"
                  >
                    {item.label}
                  </motion.span>
                )}
              </AnimatePresence>
              {active && (
                <motion.div
                  layoutId="sidebar-active"
                  className="absolute left-0 top-1/2 -translate-y-1/2 w-0.5 h-5 bg-primary rounded-r-full"
                  transition={{ type: 'spring', stiffness: 300, damping: 30 }}
                />
              )}
            </Link>
          );
        })}
      </nav>

      {/* Footer */}
      {!isMobile && (
        <div className="px-3 pb-4 pt-2 border-t border-border">
          <button
            onClick={() => setCollapsed(!collapsed)}
            className="flex items-center gap-3 w-full px-2.5 py-2.5 rounded-md text-sm text-muted hover:text-foreground hover:bg-surface-hover transition-colors"
          >
            {collapsed ? <ChevronRight size={18} /> : <ChevronLeft size={18} />}
            <AnimatePresence>
              {!collapsed && (
                <motion.span
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  exit={{ opacity: 0 }}
                  className="whitespace-nowrap"
                >
                  Collapse
                </motion.span>
              )}
            </AnimatePresence>
          </button>
        </div>
      )}
    </motion.aside>
  );
}
