'use client';

import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { Key, Plus, Copy, Check, Trash2, ShieldAlert } from 'lucide-react';

export default function ApiKeysPage() {
  const [keys, setKeys] = useState([]);
  const [isLoading, setIsLoading] = useState(true);
  const [newKeyName, setNewKeyName] = useState('');
  const [generatedKey, setGeneratedKey] = useState(null);
  const [isCreating, setIsCreating] = useState(false);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    fetchKeys();
  }, []);

  async function fetchKeys() {
    try {
      const res = await fetch('/api/keys');
      if (res.ok) {
        const data = await res.json();
        setKeys(data.keys || []);
      }
    } catch (err) {
      console.error(err);
    } finally {
      setIsLoading(false);
    }
  }

  async function handleCreate(e) {
    e.preventDefault();
    if (!newKeyName.trim()) return;

    setIsCreating(true);
    try {
      const res = await fetch('/api/keys', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: newKeyName }),
      });
      
      if (res.ok) {
        const data = await res.json();
        setGeneratedKey(data.key);
        setNewKeyName('');
        fetchKeys();
      }
    } catch (err) {
      console.error(err);
    } finally {
      setIsCreating(false);
    }
  }

  function copyToClipboard(text) {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div className="p-8 max-w-5xl mx-auto">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-foreground flex items-center gap-3">
          <Key className="text-primary" size={28} />
          API Keys
        </h1>
        <p className="text-muted mt-2 text-lg">Manage your secret keys for accessing the AgentSandbox API.</p>
      </div>

      <AnimatePresence>
        {generatedKey && (
          <motion.div
            initial={{ opacity: 0, y: -20 }}
            animate={{ opacity: 1, y: 0 }}
            className="mb-8 p-6 border border-primary/30 bg-primary/10 rounded-xl relative overflow-hidden"
          >
            <div className="absolute top-0 left-0 w-1 h-full bg-primary" />
            <div className="flex items-start gap-4">
              <ShieldAlert className="text-primary shrink-0 mt-1" size={24} />
              <div className="flex-1">
                <h3 className="text-lg font-semibold text-primary mb-1">Please copy this key now</h3>
                <p className="text-muted mb-4">You won't be able to see it again! If you lose it, you'll need to generate a new one.</p>
                <div className="flex items-center gap-2">
                  <code className="flex-1 p-3 bg-background border border-border rounded-lg text-foreground font-mono text-sm break-all">
                    {generatedKey}
                  </code>
                  <button
                    onClick={() => copyToClipboard(generatedKey)}
                    className="p-3 bg-surface hover:bg-surface-hover border border-border rounded-lg text-foreground transition-colors shrink-0 flex items-center gap-2"
                  >
                    {copied ? <Check size={18} className="text-primary" /> : <Copy size={18} />}
                    {copied ? 'Copied' : 'Copy'}
                  </button>
                </div>
              </div>
            </div>
            <button 
              onClick={() => setGeneratedKey(null)}
              className="mt-6 text-sm text-primary hover:underline font-medium"
            >
              I have saved my key safely
            </button>
          </motion.div>
        )}
      </AnimatePresence>

      <div className="bg-surface border border-border rounded-xl p-6 mb-8 shadow-sm">
        <h2 className="text-xl font-semibold mb-4 text-foreground">Create New Key</h2>
        <form onSubmit={handleCreate} className="flex gap-4">
          <input
            type="text"
            value={newKeyName}
            onChange={(e) => setNewKeyName(e.target.value)}
            placeholder="e.g. Production Env"
            className="flex-1 bg-background border border-border rounded-lg px-4 py-2.5 text-foreground focus:outline-none focus:border-primary transition-colors"
          />
          <button
            type="submit"
            disabled={isCreating || !newKeyName.trim()}
            className="bg-foreground text-background font-semibold py-2.5 px-6 rounded-lg transition-colors flex items-center gap-2 disabled:opacity-50 hover:bg-foreground/90"
          >
            <Plus size={18} />
            Create Key
          </button>
        </form>
      </div>

      <div className="bg-surface border border-border rounded-xl overflow-hidden shadow-sm">
        <div className="p-6 border-b border-border">
          <h2 className="text-xl font-semibold text-foreground">Active Keys</h2>
        </div>
        
        {isLoading ? (
          <div className="p-8 text-center text-muted">Loading keys...</div>
        ) : keys.length === 0 ? (
          <div className="p-12 text-center">
            <Key size={48} className="mx-auto text-muted/30 mb-4" />
            <h3 className="text-lg font-medium text-foreground mb-1">No API keys yet</h3>
            <p className="text-muted">Create an API key above to start using the AgentSandbox API.</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead className="bg-surface-hover/50">
                <tr>
                  <th className="text-left py-3 px-6 text-xs font-semibold text-muted uppercase tracking-wider">Name</th>
                  <th className="text-left py-3 px-6 text-xs font-semibold text-muted uppercase tracking-wider">Created</th>
                  <th className="text-left py-3 px-6 text-xs font-semibold text-muted uppercase tracking-wider">Status</th>
                  <th className="text-right py-3 px-6 text-xs font-semibold text-muted uppercase tracking-wider">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {keys.map((key, i) => (
                  <tr key={i} className="hover:bg-surface-hover/30 transition-colors">
                    <td className="py-4 px-6">
                      <div className="font-medium text-foreground">{key.name}</div>
                      <div className="text-xs text-muted font-mono mt-1">sb_live_...{key.hash_preview?.substring(0, 8)}</div>
                    </td>
                    <td className="py-4 px-6 text-sm text-muted">
                      {new Date(key.created_at).toLocaleDateString()}
                    </td>
                    <td className="py-4 px-6">
                      <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-primary/10 text-primary border border-primary/20">
                        Active
                      </span>
                    </td>
                    <td className="py-4 px-6 text-right">
                      <button className="text-muted hover:text-error transition-colors p-2 rounded-md hover:bg-error/10">
                        <Trash2 size={18} />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
