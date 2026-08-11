"use client";

import { useEffect, useRef, useState } from "react";

export type LiveState<T> = {
  data: T | null;
  error: string | null;
  loading: boolean;
  lastUpdated: Date | null;
};

/**
 * useLive - one polling primitive for every page. Fetches immediately, then on a
 * fixed interval, but PAUSES while the tab is hidden so an idle dashboard stops
 * hammering the Lambda (each open tab was ~720 invocations/hour otherwise).
 * `loading` is only true on the very first load, so refreshes never flash skeletons.
 */
export function useLive<T>(fetcher: () => Promise<T>, intervalMs = 5000): LiveState<T> {
  const [state, setState] = useState<LiveState<T>>({
    data: null,
    error: null,
    loading: true,
    lastUpdated: null,
  });
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;

  useEffect(() => {
    let alive = true;
    let timer: ReturnType<typeof setInterval> | null = null;

    const tick = async () => {
      if (document.hidden) return;
      try {
        const data = await fetcherRef.current();
        if (!alive) return;
        setState({ data, error: null, loading: false, lastUpdated: new Date() });
      } catch (e) {
        if (!alive) return;
        setState((s) => ({ ...s, error: (e as Error).message, loading: false }));
      }
    };

    tick();
    timer = setInterval(tick, intervalMs);
    const onVisible = () => {
      if (!document.hidden) tick();
    };
    document.addEventListener("visibilitychange", onVisible);

    return () => {
      alive = false;
      if (timer) clearInterval(timer);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [intervalMs]);

  return state;
}

/** secondsSince - small helper for "refreshed Ns ago" labels. */
export function useSecondsSince(ts: Date | null): number {
  const [n, setN] = useState(0);
  useEffect(() => {
    if (!ts) return;
    const t = () => setN(Math.max(0, Math.round((Date.now() - ts.getTime()) / 1000)));
    t();
    const id = setInterval(t, 1000);
    return () => clearInterval(id);
  }, [ts]);
  return n;
}
