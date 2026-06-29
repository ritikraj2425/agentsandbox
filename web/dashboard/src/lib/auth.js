'use client';

import { createContext, useContext, useState, useEffect, useCallback } from 'react';

const AuthContext = createContext(null);

export function AuthProvider({ children }) {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [user, setUser] = useState(null);

  const checkAuth = useCallback(async () => {
    try {
      const res = await fetch('/api/auth/session');
      const data = await res.json();
      if (data.authenticated) {
        setUser(data.user);
        setIsAuthenticated(true);
      } else {
        setUser(null);
        setIsAuthenticated(false);
      }
    } catch {
      setUser(null);
      setIsAuthenticated(false);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  const logout = useCallback(async () => {
    await fetch('/api/auth/mock-login', { method: 'DELETE' });
    setUser(null);
    setIsAuthenticated(false);
    window.location.href = '/login';
  }, []);

  return (
    <AuthContext.Provider value={{ isAuthenticated, isLoading, user, logout, checkAuth }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
