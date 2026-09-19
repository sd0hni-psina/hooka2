import { useEffect, useRef } from 'react';

import { apiUrl } from '../../api/client';
import { useAuthStore } from '../../store/auth';
import { useHallStore } from '../../store/hall';
import { WsClient } from '../../ws/client';

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
    });
    clientRef.current = client;
    client.subscribe(useHallStore.getState().hallId);
    client.start();
    return () => {
      client.stop();
      clientRef.current = null;
    };
  }, [signedIn]);

  useEffect(() => {
    useHallStore.getState().selectHall(hallId);
    clientRef.current?.subscribe(hallId);
  }, [hallId, signedIn]);
}
