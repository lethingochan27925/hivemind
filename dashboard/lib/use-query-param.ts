"use client";

import { useSyncExternalStore } from "react";

/**
 * useQueryParam - read a deep-link parameter (?id=, ?sql=, ?node=) as state.
 *
 * The URL is external state: it lives in the browser, not in React. Reading it
 * with an effect meant every deep-linked page rendered once empty, then called
 * setState, then rendered again — the cascading-render pattern React 19 warns
 * about, and a visible flash of the wrong content on the way in.
 *
 * useSyncExternalStore gives the value on the first client render instead, and
 * returns null during the static export's server render so hydration matches.
 */
function subscribe(onChange: () => void) {
  window.addEventListener("popstate", onChange);
  return () => window.removeEventListener("popstate", onChange);
}

export function useQueryParam(name: string): string | null {
  return useSyncExternalStore(
    subscribe,
    () => new URLSearchParams(window.location.search).get(name),
    () => null
  );
}
