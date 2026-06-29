'use client';

import { useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { useAuth } from '@/lib/auth';
import { useRouter } from 'next/navigation';
import { Eye, EyeOff, Plug, Loader2 } from 'lucide-react';

export default function LoginForm() {
  const { login } = useAuth();
  const router = useRouter();
  const [apiKey, setApiKey] = useState('');
  const [serverUrl, setServerUrl] = useState('localhost:8080');
  const [showKey, setShowKey] = useState(false);
  const [error, setError] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [shake, setShake] = useState(false);

  async function handleSubmit(e) {
    e.preventDefault();
    if (!apiKey.trim()) {
      setError('API key is required');
      setShake(true);
      setTimeout(() => setShake(false), 600);
      return;
    }

    setIsLoading(true);
    setError('');

    try {
      await login(apiKey);
      router.push('/');
    } catch (err) {
      console.error(err);
      setError(err.message || 'Failed to connect. Check your API key.');
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
      <form onSubmit={handleSubmit} className="space-y-4">
        {/* API Key Field */}
        <div>
          <label htmlFor="apiKey" className="block text-xs font-medium text-muted mb-1.5 uppercase tracking-wider">
            API Key
          </label>
          <div className="relative">
            <input
              id="apiKey"
              type={showKey ? 'text' : 'password'}
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              placeholder="sk-sandbox-..."
              className="w-full bg-background border border-border rounded-md px-3 py-2 text-foreground placeholder:text-muted-foreground font-mono text-sm focus:outline-none focus:border-primary transition-colors"
              autoComplete="off"
              autoFocus
            />
            <button
              type="button"
              onClick={() => setShowKey(!showKey)}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-muted hover:text-foreground p-1 transition-colors"
            >
              {showKey ? <EyeOff size={16} /> : <Eye size={16} />}
            </button>
          </div>
        </div>

        {/* Server URL Field */}
        <div>
          <label htmlFor="serverUrl" className="block text-xs font-medium text-muted mb-1.5 uppercase tracking-wider">
            Server URL
            <span className="text-muted-foreground ml-2 font-normal normal-case">(optional)</span>
          </label>
          <input
            id="serverUrl"
            type="text"
            value={serverUrl}
            onChange={(e) => setServerUrl(e.target.value)}
            placeholder="localhost:8080"
            className="w-full bg-background border border-border rounded-md px-3 py-2 text-foreground placeholder:text-muted-foreground font-mono text-sm focus:outline-none focus:border-primary transition-colors"
          />
        </div>

        {/* Error */}
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

        {/* Submit */}
        <button
          type="submit"
          disabled={isLoading}
          className="w-full bg-primary hover:bg-primary-hover disabled:opacity-50 disabled:cursor-not-allowed text-background font-semibold py-2.5 px-4 rounded-md transition-colors flex items-center justify-center gap-2 mt-2"
        >
          {isLoading ? (
            <>
              <Loader2 size={16} className="animate-spin" />
              Connecting…
            </>
          ) : (
            <>
              <Plug size={16} />
              Connect
            </>
          )}
        </button>
      </form>
    </motion.div>
  );
}
