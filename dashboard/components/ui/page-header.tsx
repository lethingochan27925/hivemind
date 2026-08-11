"use client";

import { ReactNode } from "react";
import { RefreshCw, WifiOff } from "lucide-react";
import { useSecondsSince } from "@/lib/use-live";

/**
 * PageHeader - the consistent top bar for every page: a bold title, a one-line
 * description, a live "refreshed Ns ago" clock, a connection state, and actions.
 */
export function PageHeader({
  title,
  description,
  lastUpdated,
  error,
  actions,
}: {
  title: string;
  description?: string;
  lastUpdated?: Date | null;
  error?: string | null;
  actions?: ReactNode;
}) {
  const secs = useSecondsSince(lastUpdated ?? null);

  return (
    <div className="sticky top-0 z-10 flex items-center justify-between gap-4 px-6 h-16 border-b border-border bg-bg-primary/90 backdrop-blur-sm">
      <div className="min-w-0">
        <h1 className="text-[22px] font-bold text-text-primary tracking-tight leading-tight">
          {title}
        </h1>
        {description && (
          <p className="text-[13px] text-text-secondary mt-0.5 truncate">{description}</p>
        )}
      </div>
      <div className="flex items-center gap-3 shrink-0">
        {actions}
        {error ? (
          <span className="flex items-center gap-1.5 text-[13px] font-medium text-red">
            <WifiOff size={14} />
            Disconnected
          </span>
        ) : lastUpdated ? (
          <span className="flex items-center gap-1.5 text-[13px] text-text-tertiary">
            <span className="w-2 h-2 rounded-full bg-green hm-pulse" />
            <RefreshCw size={13} />
            {secs}s ago
          </span>
        ) : null}
      </div>
    </div>
  );
}
