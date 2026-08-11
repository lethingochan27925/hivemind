"use client";

import Link from "next/link";
import { api } from "@/lib/api";
import { useLive } from "@/lib/use-live";
import { Command, CircleDot, AlertTriangle, DollarSign } from "lucide-react";

/**
 * Topbar - the platform's always-visible system strip (Datadog/Grafana style).
 * One glance from any page: is the fleet running, how deep is the queue, is
 * anything alarming, what has today cost. Every chip navigates to its page.
 */
export function Topbar() {
  const fleet = useLive(api.getFleetStatus, 8000);
  const infra = useLive(api.getInfrastructure, 20000);
  const cost = useLive(api.getCost, 60000);

  const running = fleet.data?.running ?? false;
  const tasks = fleet.data?.tasks ?? {};
  const pending = tasks["pending"] ?? 0;
  const investigating = tasks["investigating"] ?? 0;
  const alarms = (infra.data?.services ?? []).filter((s) => s.alarm_state === "ALARM").length;
  const spend = cost.data?.estimated_cost_usd_today;

  const chip =
    "flex items-center gap-1.5 px-2.5 h-7 rounded-md border text-[12px] transition-colors";

  return (
    <header className="h-11 shrink-0 border-b border-border bg-bg-panel/70 backdrop-blur flex items-center gap-2 px-4 sticky top-0 z-40">
      {/* Fleet state */}
      <Link
        href="/"
        className={`${chip} ${
          running
            ? "border-green/30 bg-green/10 text-green"
            : "border-border bg-bg-inset/50 text-text-tertiary"
        }`}
      >
        <span className={`w-1.5 h-1.5 rounded-full ${running ? "bg-green hm-pulse" : "bg-text-tertiary"}`} />
        {running ? "FLEET RUNNING" : "FLEET PAUSED"}
      </Link>

      {/* Queue depth */}
      <Link href="/" className={`${chip} border-border bg-bg-inset/50 text-text-secondary hover:text-text-primary`}>
        <CircleDot size={11} className="text-blue" />
        <span className="tnum">{pending}</span> pending
        <span className="text-text-tertiary">·</span>
        <span className="tnum">{investigating}</span> active
      </Link>

      {/* Alarms */}
      <Link
        href="/infrastructure"
        className={`${chip} ${
          alarms > 0
            ? "border-red/40 bg-red/10 text-red"
            : "border-border bg-bg-inset/50 text-text-secondary hover:text-text-primary"
        }`}
      >
        <AlertTriangle size={11} className={alarms > 0 ? "text-red" : "text-green"} />
        {alarms > 0 ? (
          <>
            <span className="tnum">{alarms}</span> alarm{alarms > 1 ? "s" : ""}
          </>
        ) : (
          "systems OK"
        )}
      </Link>

      {/* Spend today */}
      <Link href="/cost" className={`${chip} border-border bg-bg-inset/50 text-text-secondary hover:text-text-primary`}>
        <DollarSign size={11} className="text-green" />
        <span className="tnum">{spend != null ? `$${spend.toFixed(4)}` : "—"}</span> today
      </Link>

      {/* Command palette hint */}
      <button
        onClick={() => window.dispatchEvent(new CustomEvent("hm-cmdk"))}
        className={`${chip} ml-auto border-border bg-bg-inset/50 text-text-tertiary hover:text-text-primary hover:border-border-strong`}
        title="Command palette"
      >
        <Command size={11} />
        <span>K</span>
        <span className="hidden sm:inline text-text-tertiary">· command</span>
      </button>
    </header>
  );
}
