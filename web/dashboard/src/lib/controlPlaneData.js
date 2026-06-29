export const projects = [
  {
    id: 'proj_core',
    name: 'Core Coding Agents',
    owner: 'Platform',
    runtime: 'docker',
    image: 'golang:1.26-bookworm',
    browserEnabled: false,
    policy: 'coding-safe',
    sessions: 18,
    status: 'active',
    createdAt: '2026-06-24T09:12:00Z',
  },
  {
    id: 'proj_browser',
    name: 'Browser Research',
    owner: 'Growth',
    runtime: 'browser',
    image: 'agentsandbox/browser-runtime:latest',
    browserEnabled: true,
    policy: 'browser-safe',
    sessions: 7,
    status: 'active',
    createdAt: '2026-06-26T14:22:00Z',
  },
  {
    id: 'proj_eval',
    name: 'LLM Eval Harness',
    owner: 'Research',
    runtime: 'gvisor',
    image: 'python:3.13-slim',
    browserEnabled: false,
    policy: 'no-network',
    sessions: 41,
    status: 'active',
    createdAt: '2026-06-20T18:45:00Z',
  },
];

export const policies = [
  {
    id: 'pol_coding',
    name: 'coding-safe',
    effect: 'deny',
    actionTypes: ['shell.run', 'file.read', 'file.write', 'file.patch'],
    denies: ['rm -rf', 'sudo', '.env', '.git'],
    approvals: ['npm install', 'go get'],
    projects: ['Core Coding Agents'],
    updatedAt: '2026-06-28T11:10:00Z',
    valid: true,
  },
  {
    id: 'pol_browser',
    name: 'browser-safe',
    effect: 'deny',
    actionTypes: ['browser.goto', 'browser.click', 'browser.screenshot', 'browser.user_handoff'],
    denies: ['localhost', '127.0.0.1', '169.254.169.254'],
    approvals: ['accounts.google.com'],
    projects: ['Browser Research'],
    updatedAt: '2026-06-29T08:16:00Z',
    valid: true,
  },
  {
    id: 'pol_nonetwork',
    name: 'no-network',
    effect: 'deny',
    actionTypes: ['shell.run', 'file.read', 'file.write'],
    denies: ['curl', 'wget', 'ssh', 'browser.goto'],
    approvals: [],
    projects: ['LLM Eval Harness'],
    updatedAt: '2026-06-27T22:31:00Z',
    valid: true,
  },
];

export const apiKeys = [
  { id: 'key_prod', name: 'Production gateway', prefix: 'sb_live_9fd2', createdAt: '2026-06-26T10:14:00Z', lastUsed: '2026-06-29T09:04:00Z', status: 'active' },
  { id: 'key_ci', name: 'CI eval runner', prefix: 'sb_live_1aa8', createdAt: '2026-06-23T16:30:00Z', lastUsed: '2026-06-29T07:55:00Z', status: 'active' },
  { id: 'key_old', name: 'Legacy test key', prefix: 'sb_live_03be', createdAt: '2026-06-12T12:00:00Z', lastUsed: '2026-06-18T14:10:00Z', status: 'revoked' },
];

export const sessions = [
  {
    id: 'sess_browser_01',
    project: 'Browser Research',
    backend: 'browser',
    status: 'running',
    policy: 'browser-safe',
    user: 'maya@acme.dev',
    createdAt: '2026-06-29T08:57:00Z',
    expiresAt: '2026-06-29T10:57:00Z',
    workspace: '.agentsandbox/sessions/8f2/workspace',
    artifactCount: 3,
    actionCount: 9,
  },
  {
    id: 'sess_code_42',
    project: 'Core Coding Agents',
    backend: 'docker',
    status: 'completed',
    policy: 'coding-safe',
    user: 'ritik@agentsandbox.dev',
    createdAt: '2026-06-29T06:12:00Z',
    expiresAt: '2026-06-29T08:12:00Z',
    workspace: '.agentsandbox/sessions/19a/workspace',
    artifactCount: 1,
    actionCount: 14,
  },
  {
    id: 'sess_eval_77',
    project: 'LLM Eval Harness',
    backend: 'gvisor',
    status: 'denied',
    policy: 'no-network',
    user: 'evals@acme.dev',
    createdAt: '2026-06-28T23:41:00Z',
    expiresAt: '2026-06-29T01:41:00Z',
    workspace: '.agentsandbox/sessions/c31/workspace',
    artifactCount: 0,
    actionCount: 6,
  },
];

export const sessionDetail = {
  id: 'sess_browser_01',
  project: 'Browser Research',
  backend: 'browser',
  status: 'running',
  policy: 'browser-safe',
  actions: [
    { id: 'act_001', type: 'browser.goto', status: 'completed', policy: 'allow', duration: 742, summary: 'Navigated to https://example.com' },
    { id: 'act_002', type: 'browser.screenshot', status: 'completed', policy: 'allow', duration: 118, summary: 'Stored screenshot artifact' },
    { id: 'act_003', type: 'browser.goto', status: 'denied', policy: 'deny', duration: 0, summary: 'Blocked internal browser target 169.254.169.254' },
    { id: 'act_004', type: 'browser.user_handoff', status: 'waiting_for_user', policy: 'allow', duration: 0, summary: 'Created scoped user handoff stream' },
  ],
  artifacts: [
    { id: 'screenshot_20260629_085900.png', type: 'image/png', size: '184 KB', action: 'act_002' },
    { id: 'console_20260629_085901.json', type: 'application/json', size: '6 KB', action: 'act_002' },
  ],
  timeline: [
    { time: '08:57:00', type: 'session.created', actor: 'maya@acme.dev', message: 'Browser session created' },
    { time: '08:57:01', type: 'policy.check', actor: 'gateway', message: 'browser-safe allowed browser.goto' },
    { time: '08:57:02', type: 'action.completed', actor: 'runtime', message: 'Page loaded and metadata captured' },
    { time: '08:58:14', type: 'policy.check', actor: 'gateway', message: 'Denied metadata IP navigation' },
    { time: '09:01:08', type: 'human.interaction', actor: 'user', message: 'Scoped browser handoff opened' },
  ],
};

export const approvals = [
  { id: 'apr_001', project: 'Core Coding Agents', session: 'sess_code_42', action: 'npm install playwright', requester: 'agent-runner', status: 'pending', reason: 'Package installation requires approval' },
  { id: 'apr_002', project: 'Browser Research', session: 'sess_browser_01', action: 'browser.goto accounts.google.com', requester: 'research-agent', status: 'pending', reason: 'Sensitive auth domain' },
  { id: 'apr_003', project: 'LLM Eval Harness', session: 'sess_eval_77', action: 'go get github.com/acme/lib', requester: 'eval-agent', status: 'denied', reason: 'Network disabled for eval project' },
];

export const auditLogs = [
  { id: 'aud_001', time: '2026-06-29T09:04:00Z', user: 'maya@acme.dev', project: 'Browser Research', session: 'sess_browser_01', type: 'human.interaction', message: 'Opened scoped browser stream' },
  { id: 'aud_002', time: '2026-06-29T08:58:14Z', user: 'gateway', project: 'Browser Research', session: 'sess_browser_01', type: 'policy.denied', message: 'Blocked navigation to 169.254.169.254' },
  { id: 'aud_003', time: '2026-06-29T08:40:11Z', user: 'ritik@agentsandbox.dev', project: 'Core Coding Agents', session: 'sess_code_42', type: 'api_key.created', message: 'Created CI eval runner key' },
  { id: 'aud_004', time: '2026-06-28T23:44:20Z', user: 'evals@acme.dev', project: 'LLM Eval Harness', session: 'sess_eval_77', type: 'approval.denied', message: 'Denied network dependency fetch' },
];
