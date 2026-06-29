const BASE_URL = process.env.NEXT_PUBLIC_API_URL || '';

async function request(path, options = {}) {
  const url = `${BASE_URL}${path}`;
  const config = {
    credentials: 'include',
    cache: 'no-store',
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
    ...options,
  };

  const res = await fetch(url, config);

  if (res.status === 401) {
    throw new Error('Unauthorized');
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || body.message || `Request failed: ${res.status}`);
  }

  if (res.status === 204) return null;
  return res.json();
}

export function login(apiKey) {
  return request('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ api_key: apiKey }),
  });
}

export function getMe() {
  return request('/api/auth/me');
}

export function getSessions() {
  return request('/api/dashboard/sessions');
}

export function getSession(id) {
  return request(`/api/dashboard/sessions/${id}`);
}

export function getSessionVNC(id) {
  return request(`/api/dashboard/sessions/${id}/vnc`);
}

export function createSession(backend, options = {}) {
  return request('/api/sessions', {
    method: 'POST',
    body: JSON.stringify({ backend, ...options }),
  });
}

export function deleteSession(id) {
  return request(`/api/sessions/${id}`, { method: 'DELETE' });
}

export function runAction(sessionId, command) {
  return request(`/api/sessions/${sessionId}/actions`, {
    method: 'POST',
    body: JSON.stringify({ command }),
  });
}

export function getRuns() {
  return request('/api/runs');
}

export function getRun(id) {
  return request(`/api/runs/${id}`);
}
