import { useEffect, useRef } from 'react';
import { AppState } from 'react-native';

import { apiUrl } from '../../api/client';
import { useAuthStore } from '../../store/auth';
import { useHallStore } from '../../store/hall';
import { WsClient } from '../../ws/client';
import { setTickerActive } from '../../ws/ticker';

function wsUrl(token: string): string {
  return `${apiUrl.replace(/^http/, 'ws')}/api/v1/ws?access_token=${encodeURIComponent(token)}`;
}

// useHallConnection keeps the WS open while signed in and subscribes it to
// the selected hall. Every (re)subscribe replaces the store with a snapshot.
export function useHallConnection(hallId: string | null) {
  const clientRef = useRef<WsClient | null>(null);
  const signedIn = useAuthStore((s) => s.status === 'signedIn');

  useEffect(() => {
    if (!signedIn) return;
    const client = new WsClient({
      // accessToken refreshes ahead of expiry, so each reconnect carries a live token.
      url: async () => {
        const token = await useAuthStore.getState().accessToken();
        return token ? wsUrl(token) : null;
      },
      onMessage: (msg) => useHallStore.getState().apply(msg),
      onStatus: (s) => useHallStore.getState().setConnection(s),
      // Close code 1008 means the session was revoked or the master blocked.
      // /me tells which: 401 and 403 sign out, anything else leaves the socket
      // closed until the app returns to the foreground, tech.md section 19.17.
      onRevoked: () => void useAuthStore.getState().refreshMe(),
    });
    clientRef.current = client;
    client.subscribe(useHallStore.getState().hallId);
    client.start();
    return () => {
      client.stop();
      clientRef.current = null;
    };
  }, [signedIn]);

  // A backgrounded app burns no timer and its socket may die silently. On the
  // way back the ticker fires once and the client reconnects immediately
  // instead of waiting out the backoff, tech.md section 19.15.
  useEffect(() => {
    const sub = AppState.addEventListener('change', (next) => {
      const active = next === 'active';
      setTickerActive(active);
      if (active) clientRef.current?.resume();
    });
    return () => sub.remove();
  }, []);

  useEffect(() => {
    useHallStore.getState().selectHall(hallId);
    clientRef.current?.subscribe(hallId);
  }, [hallId, signedIn]);
}
