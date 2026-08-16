"use client";

import Link from "next/link";
import { useFleet } from "@/lib/use-fleet";
import { useInfrastructure } from "@/lib/use-infrastructure";
import { useCost } from "@/lib/use-cost";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { LangToggle } from "@/components/layout/lang-toggle";
import { useT } from "@/lib/i18n";
import { Command, CircleDot, AlertTriangle, DollarSign } from "lucide-react";

/**
 * Topbar - the platform's always-visible system strip (Datadog/Grafana style).
 * One glance from any page: is the fleet running, how deep is the queue, is
 * anything alarming, what has today cost. Every chip navigates to its page.
 */
export function Topbar() {
  const t = useT();
  const fleet = useFleet();
  const infra = useInfrastructure();
  const cost = useCost();

  const running = fleet.data?.running ?? false;
  const tasks = fleet.data?.tasks ?? {};
  const pending = tasks["pending"] ?? 0;
  const investigating = tasks["investigating"] ?? 0;
  const alarms = (infra.data?.services ?? []).filter((s) => s.alarm_state === "ALARM").length;
  const spend = cost.data?.estimated_cost_usd_today;

  // shrink-0 on every chip: without it a narrow viewport squeezed each chip's
  // text instead of the whole row scrolling, and past a point the chips lost
  // their icon/text spacing entirely rather than just running off-screen.
  const chip =
    "flex items-center gap-1.5 px-2.5 h-7 rounded-md border text-[12px] transition-colors shrink-0";

  return (
    <header className="h-11 shrink-0 border-b border-border bg-bg-panel/70 backdrop-blur flex items-center gap-2 px-4 sticky top-0 z-40 overflow-x-auto">
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
        {running ? t("FLEET RUNNING") : t("FLEET PAUSED")}
      </Link>

      {/* Queue depth */}
      <Link href="/" className={`${chip} border-border bg-bg-inset/50 text-text-secondary hover:text-text-primary`}>
        <CircleDot size={11} className="text-blue" />
        <span className="tnum">{pending}</span> {t("pending")}
        <span className="text-text-tertiary">·</span>
        <span className="tnum">{investigating}</span> {t("active")}
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
            <span className="tnum">{alarms}</span> {alarms > 1 ? t("alarms") : t("alarm")}
          </>
        ) : (
          t("systems OK")
        )}
      </Link>

      {/* Spend today */}
      <Link href="/cost" className={`${chip} border-border bg-bg-inset/50 text-text-secondary hover:text-text-primary`}>
        <DollarSign size={11} className="text-green" />
        <span className="tnum">{spend != null ? `$${spend.toFixed(4)}` : "-"}</span> {t("today")}
      </Link>

      <span className="ml-auto" />
      <LangToggle />
      <ThemeToggle />

      {/* Command palette hint */}
      <button
        onClick={() => window.dispatchEvent(new CustomEvent("hm-cmdk"))}
        className={`${chip} border-border bg-bg-inset/50 text-text-tertiary hover:text-text-primary hover:border-border-strong`}
        title="Command palette"
      >
        <Command size={11} />
        <span>K</span>
        <span className="hidden sm:inline text-text-tertiary">· {t("command")}</span>
      </button>
    </header>
  );
}
