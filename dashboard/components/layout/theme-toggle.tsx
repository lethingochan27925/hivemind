"use client";

import { useSyncExternalStore } from "react";
import { Sun, Moon } from "lucide-react";

type Theme = "light" | "dark";

// The theme lives on <html data-theme>, written before first paint by the
// no-FOUC script in layout.tsx. That attribute is the source of truth, so this
// component observes it instead of keeping a second copy in React state - which
// also means the icon stays correct if anything else flips the attribute.
function subscribeTheme(onChange: () => void) {
  const observer = new MutationObserver(onChange);
  observer.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["data-theme"],
  });
  return () => observer.disconnect();
}

function readTheme(): Theme {
  return document.documentElement.dataset.theme === "dark" ? "dark" : "light";
}

/** ThemeToggle - flips [data-theme] on <html> and persists the choice. */
export function ThemeToggle() {
  const theme = useSyncExternalStore(subscribeTheme, readTheme, () => "light" as Theme);

  const flip = () => {
    const next: Theme = theme === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = next;
    try {
      localStorage.setItem("hm-theme", next);
    } catch {
      // Private mode: the theme still flips, it just will not be remembered.
    }
  };

  return (
    <button
      onClick={flip}
      title={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
      aria-label={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
      className="flex items-center justify-center w-7 h-7 rounded-md border border-border bg-bg-inset/50 text-text-tertiary hover:text-text-primary hover:border-border-strong transition-colors"
    >
      {theme === "dark" ? <Sun size={13} /> : <Moon size={13} />}
    </button>
  );
}
