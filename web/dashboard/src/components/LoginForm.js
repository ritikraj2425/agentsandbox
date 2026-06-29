'use client';

import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useRouter } from 'next/navigation';
import { Mail, Loader2, Play } from 'lucide-react'; // use Play for Google since google icon isn't standard in lucide

export default function LoginForm() {
  const router = useRouter();
  const [email, setEmail] = useState('');
  const [error, setError] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [loadingProvider, setLoadingProvider] = useState(null);
  const [shake, setShake] = useState(false);
  const [showEmail, setShowEmail] = useState(false);

  async function handleOAuthLogin(provider) {
    setLoadingProvider(provider);
    setError('');
    
    try {
      // Mock OAuth flow - usually this redirects to Github/Google
      // We simulate it returning an email based on the provider
      const mockEmail = `user+${provider}@agentsandbox.com`;
      
      const res = await fetch('/api/auth/mock-login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ provider, email: mockEmail }),
      });
      
      if (!res.ok) throw new Error('Failed to authenticate');
      
      router.push('/dashboard');
      router.refresh();
    } catch (err) {
      console.error(err);
      setError('OAuth login failed.');
      setShake(true);
      setTimeout(() => setShake(false), 600);
    } finally {
      setLoadingProvider(null);
    }
  }

  async function handleEmailLogin(e) {
    e.preventDefault();
    if (!email.trim() || !email.includes('@')) {
      setError('Please enter a valid email.');
      setShake(true);
      setTimeout(() => setShake(false), 600);
      return;
    }

    setIsLoading(true);
    setError('');

    try {
      const res = await fetch('/api/auth/mock-login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ provider: 'email', email }),
      });
      
      if (!res.ok) throw new Error('Failed to authenticate');
      
      router.push('/dashboard');
      router.refresh();
    } catch (err) {
      setError('Login failed.');
      setShake(true);
      setTimeout(() => setShake(false), 600);
    } finally {
      setIsLoading(false);
    }
  }

  return (
    <motion.div
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3, ease: 'easeOut' }}
      className={shake ? 'animate-shake' : ''}
    >
      <div className="space-y-4">
        {/* OAuth Buttons */}
        <button
          onClick={() => handleOAuthLogin('github')}
          disabled={loadingProvider !== null}
          className="w-full bg-[#24292e] hover:bg-[#2f363d] disabled:opacity-50 disabled:cursor-not-allowed text-white font-semibold py-2.5 px-4 rounded-md transition-colors flex items-center justify-center gap-2"
        >
          {loadingProvider === 'github' ? <Loader2 size={18} className="animate-spin" /> : <svg className="w-[18px] h-[18px]" viewBox="0 0 24 24" fill="currentColor" xmlns="http://www.w3.org/2000/svg"><path d="M12 2C6.477 2 2 6.477 2 12c0 4.42 2.865 8.166 6.839 9.489.5.092.682-.217.682-.482 0-.237-.008-.866-.013-1.7-2.782.604-3.369-1.34-3.369-1.34-.454-1.156-1.11-1.464-1.11-1.464-.908-.62.069-.608.069-.608 1.003.07 1.531 1.03 1.531 1.03.892 1.529 2.341 1.087 2.91.831.092-.646.35-1.086.636-1.336-2.22-.253-4.555-1.11-4.555-4.943 0-1.091.39-1.984 1.029-2.683-.103-.253-.446-1.27.098-2.647 0 0 .84-.269 2.75 1.025A9.578 9.578 0 0112 6.836c.85.004 1.705.114 2.504.336 1.909-1.294 2.747-1.025 2.747-1.025.546 1.377.203 2.394.1 2.647.64.699 1.028 1.592 1.028 2.683 0 3.842-2.339 4.687-4.566 4.935.359.309.678.919.678 1.852 0 1.336-.012 2.415-.012 2.743 0 .267.18.578.688.48C19.138 20.161 22 16.416 22 12c0-5.523-4.477-10-10-10z"/></svg>}
          Continue with GitHub
        </button>

        <button
          onClick={() => handleOAuthLogin('google')}
          disabled={loadingProvider !== null}
          className="w-full bg-white hover:bg-gray-100 disabled:opacity-50 disabled:cursor-not-allowed text-black font-semibold py-2.5 px-4 rounded-md transition-colors flex items-center justify-center gap-2"
        >
          {loadingProvider === 'google' ? <Loader2 size={18} className="animate-spin text-gray-500" /> : <svg className="w-[18px] h-[18px]" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4"/><path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/><path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/><path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/></svg>}
          Continue with Google
        </button>

        <div className="relative my-6">
          <div className="absolute inset-0 flex items-center">
            <div className="w-full border-t border-border"></div>
          </div>
          <div className="relative flex justify-center text-sm">
            <span className="px-2 bg-background text-muted-foreground">Or continue with email</span>
          </div>
        </div>

        {!showEmail ? (
          <button
            onClick={() => setShowEmail(true)}
            className="w-full bg-background border border-border hover:bg-border/30 text-foreground font-semibold py-2.5 px-4 rounded-md transition-colors flex items-center justify-center gap-2"
          >
            <Mail size={18} />
            Email Login
          </button>
        ) : (
          <form onSubmit={handleEmailLogin} className="space-y-4">
            <div>
              <input
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="you@example.com"
                className="w-full bg-background border border-border rounded-md px-3 py-2 text-foreground placeholder:text-muted-foreground text-sm focus:outline-none focus:border-primary transition-colors"
                autoFocus
              />
            </div>

            <AnimatePresence>
              {error && (
                <motion.div
                  initial={{ opacity: 0, height: 0 }}
                  animate={{ opacity: 1, height: 'auto' }}
                  exit={{ opacity: 0, height: 0 }}
                  className="bg-error-muted border border-error/20 rounded-md px-3 py-2 text-error text-sm overflow-hidden"
                >
                  {error}
                </motion.div>
              )}
            </AnimatePresence>

            <button
              type="submit"
              disabled={isLoading}
              className="w-full bg-primary hover:bg-primary-hover disabled:opacity-50 disabled:cursor-not-allowed text-background font-semibold py-2.5 px-4 rounded-md transition-colors flex items-center justify-center gap-2"
            >
              {isLoading ? <Loader2 size={16} className="animate-spin" /> : 'Sign In'}
            </button>
          </form>
        )}
      </div>
    </motion.div>
  );
}
