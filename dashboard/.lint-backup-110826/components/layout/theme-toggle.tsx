"use client";

import { useEffect, useState } from "react";
import { Sun, Moon } from "lucide-react";

/**
 * ThemeToggle - flips [data-theme] on <html> and persists the choice.
 * The no-FOUC bootstrap script in layout.tsx applies the stored theme before
 * first paint; this component only reflects and mutates it.
 */
export function ThemeToggle() {
  const [theme, setTheme] = useState<"light" | "dark">("light");

  useEffect(() => {
    setTheme(document.documentElement.dataset.theme === "dark" ? "dark" : "light");
  }, []);

  const flip = () => {
    const next = theme === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = next;
    try {
      localStorage.setItem("hm-theme", next);
    } catch {}
    setTheme(next);
  };

  return (
    <button
      onClick={flip}
      title={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
      className="flex items-center justify-center w-7 h-7 rounded-md border border-border bg-bg-inset/50 text-text-tertiary hover:text-text-primary hover:border-border-strong transition-colors"
    >
      {theme === "dark" ? <Sun size={13} /> : <Moon size={13} />}
    </button>
  );
}
