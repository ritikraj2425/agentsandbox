'use client';

import { Settings } from 'lucide-react';

export default function SettingsPage() {
  return (
    <div className="p-6 md:p-8 max-w-7xl mx-auto w-full">
      <div className="mb-8">
        <h1 className="text-2xl font-medium tracking-tight mb-1">Settings</h1>
        <p className="text-muted text-sm">Configure your AgentSandbox preferences</p>
      </div>

      <div className="flex flex-col items-center justify-center py-20 bg-surface border border-border rounded-md">
        <Settings size={48} className="text-muted mb-4 opacity-50" strokeWidth={1.5} />
        <h3 className="text-lg font-medium mb-1 tracking-tight">Settings Coming Soon</h3>
        <p className="text-muted text-sm">Configuration options will be available in a future update.</p>
      </div>
    </div>
  );
}
