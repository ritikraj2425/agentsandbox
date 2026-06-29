'use client';

import { createContext, useContext, useState, useEffect, useCallback } from 'react';
import { login as apiLogin, getMe } from '@/lib/api';

const AuthContext = createContext(null);

export function AuthProvider({ children }) {
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [isLoading, setIsLoading] = useState(true);
  const [user, setUser] = useState(null);

  const checkAuth = useCallback(async () => {
    try {
      const me = await getMe();
      setUser(me);
      setIsAuthenticated(true);
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

  const login = useCallback(async (apiKey) => {
    const result = await apiLogin(apiKey);
    setUser(result);
    setIsAuthenticated(true);
    return result;
  }, []);

  const logout = useCallback(() => {
    document.cookie = 'session=; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT';
    document.cookie = 'token=; Path=/; Expires=Thu, 01 Jan 1970 00:00:00 GMT';
    setUser(null);
    setIsAuthenticated(false);
    window.location.href = '/login';
  }, []);

  return (
    <AuthContext.Provider value={{ isAuthenticated, isLoading, user, login, logout, checkAuth }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within AuthProvider');
  return ctx;
}
