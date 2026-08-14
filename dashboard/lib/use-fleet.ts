"use client";

import { useSyncExternalStore } from "react";
import { api, FleetStatus } from "@/lib/api";

/**
 * useFleet - ONE poll of the fleet state, shared by every component that shows it.
 *
 * The topbar chip, the Mission Control banner, the fleet command bar and the
 * infrastructure page all display "is the fleet running". Each used to poll on
 * its own interval (8s, 4s, 10s), so the same fact could be rendered three
 * different ways at once — the chip said PAUSED while the panel said running —
 * and one open tab cost three times the Lambda invocations it needed to.
 *
 * The poller is reference-counted: it starts on the first subscriber, stops on
 * the last, and pauses while the tab is hidden.
 */
export interface FleetSnapshot {
  data: FleetStatus | null;
  error: string | null;
  loading: boolean;
}

const SERVER_SNAPSHOT: FleetSnapshot = { data: null, error: null, loading: true };

let snapshot: FleetSnapshot = SERVER_SNAPSHOT;
let timer: ReturnType<typeof setInterval> | null = null;
let inFlight = false;

const listeners = new Set<() => void>();
const emit = () => listeners.forEach((notify) => notify());

async function tick() {
  if (inFlight) return;
  if (typeof document !== "undefined" && document.hidden) return;
  inFlight = true;
  try {
    snapshot = { data: await api.getFleetStatus(), error: null, loading: false };
  } catch (e) {
    // Keep the last known data: a single failed poll should dim the view, not
    // blank it out.
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
    timer = setInterval(tick, 4000);
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

export function useFleet(): FleetSnapshot {
  return useSyncExternalStore(subscribe, () => snapshot, () => SERVER_SNAPSHOT);
}

/**
 * refreshFleet - poll immediately instead of waiting out the interval. Called
 * after an action so the UI reflects it at once; without this, pressing "Start
 * fleet" left the badge reading "paused" for up to four seconds and people
 * pressed it again.
 */
export function refreshFleet(): Promise<void> {
  return tick();
}
