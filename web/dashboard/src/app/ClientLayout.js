'use client';

import { usePathname, useRouter } from 'next/navigation';
import { AuthProvider, useAuth } from '@/lib/auth';
import Sidebar from '@/components/Sidebar';
import Header from '@/components/Header';
import { useEffect } from 'react';

function LayoutInner({ children }) {
  const pathname = usePathname();
  const router = useRouter();
  const { isAuthenticated, isLoading } = useAuth();
  
  const isLoginPage = pathname === '/login';

  useEffect(() => {
    if (!isLoading && !isAuthenticated && !isLoginPage) {
      router.push('/login');
    }
  }, [isLoading, isAuthenticated, isLoginPage, router]);

  if (isLoading) {
    return <div className="h-screen bg-background flex items-center justify-center text-muted">Loading...</div>;
  }

  if (isLoginPage) {
    return <>{children}</>;
  }

  if (!isAuthenticated) {
    return <div className="h-screen bg-background flex items-center justify-center text-muted">Redirecting to login...</div>;
  }

  return (
    <div className="flex h-screen bg-background overflow-hidden text-foreground">
      <Sidebar />
      <div className="flex-1 flex flex-col min-w-0">
        <Header />
        <main className="flex-1 overflow-auto">{children}</main>
      </div>
    </div>
  );
}

export default function ClientLayout({ children }) {
  return (
    <AuthProvider>
      <LayoutInner>{children}</LayoutInner>
    </AuthProvider>
  );
}
