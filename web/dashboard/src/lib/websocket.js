export function createEventSocket(sessionId) {
  const protocol = typeof window !== 'undefined' && window.location.protocol === 'https:' ? 'wss' : 'ws';
  const host = process.env.NEXT_PUBLIC_WS_HOST || 'localhost:8080';
  const url = `${protocol}://${host}/v1/sessions/${sessionId}/events`;

  let ws = null;
  let listeners = [];
  let reconnectAttempts = 0;
  let maxReconnects = 10;
  let closed = false;
  let reconnectTimer = null;

  function connect() {
    if (closed) return;

    ws = new WebSocket(url);

    ws.onopen = () => {
      reconnectAttempts = 0;
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        listeners.forEach((cb) => cb(data));
      } catch {
        listeners.forEach((cb) => cb({ raw: event.data }));
      }
    };

    ws.onclose = () => {
      if (closed) return;
      const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 30000);
      reconnectAttempts++;
      if (reconnectAttempts <= maxReconnects) {
        reconnectTimer = setTimeout(connect, delay);
      }
    };

    ws.onerror = () => {
      ws?.close();
    };
  }

  connect();

  return {
    onMessage(callback) {
      listeners.push(callback);
      return () => {
        listeners = listeners.filter((cb) => cb !== callback);
      };
    },
    close() {
      closed = true;
      clearTimeout(reconnectTimer);
      ws?.close();
      listeners = [];
    },
    getState() {
      return ws?.readyState;
    },
  };
}
