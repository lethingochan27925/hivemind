"use client";

import { useSyncExternalStore } from "react";

/**
 * createSharedPoll - the same reference-counted, tab-visibility-aware polling
 * store as useFleet (lib/use-fleet.ts), generalised to any endpoint. Use this
 * whenever more than one component on screen at once needs the same data:
 * the topbar chip and a page's own panel used to each open their own
 * `useLive` poll of the identical endpoint at two different intervals, so a
 * single open tab cost twice the Lambda invocations it needed to, and the two
 * copies of the same fact could disagree for a moment after every poll.
 *
 * The poller starts on the first subscriber and stops on the last, so a page
 * that never mounts the shared consumer costs nothing extra.
 */
export interface SharedPollSnapshot<T> {
  data: T | null;
  error: string | null;
  loading: boolean;
  lastUpdated: Date | null;
}

export interface SharedPoll<T> {
  use: () => SharedPollSnapshot<T>;
  refresh: () => Promise<void>;
}

export function createSharedPoll<T>(fetcher: () => Promise<T>, intervalMs: number): SharedPoll<T> {
  const serverSnapshot: SharedPollSnapshot<T> = { data: null, error: null, loading: true, lastUpdated: null };
  let snapshot = serverSnapshot;
  let timer: ReturnType<typeof setInterval> | null = null;
  let inFlight = false;

  const listeners = new Set<() => void>();
  const emit = () => listeners.forEach((notify) => notify());

  async function tick() {
    if (inFlight) return;
    if (typeof document !== "undefined" && document.hidden) return;
    inFlight = true;
    try {
      snapshot = { data: await fetcher(), error: null, loading: false, lastUpdated: new Date() };
    } catch (e) {
      // Keep the last known data: one failed poll should dim the view, not blank it.
      snapshot = { ...snapshot, error: (e as Error).message, loading: false };
    } finally {
      inFlight = false;
      emit();
    }
  }

  const onVisible = () => {
    if (!document.hidden) void tick();
  };

  function subscribe(onChange: () => void) {
    listeners.add(onChange);
    if (listeners.size === 1) {
      void tick();
      timer = setInterval(tick, intervalMs);
      document.addEventListener("visibilitychange", onVisible);
    }
    return () => {
      listeners.delete(onChange);
      if (listeners.size === 0 && timer) {
        clearInterval(timer);
        timer = null;
        document.removeEventListener("visibilitychange", onVisible);
      }
    };
  }

  return {
    use: () => useSyncExternalStore(subscribe, () => snapshot, () => serverSnapshot),
    refresh: tick,
  };
}
