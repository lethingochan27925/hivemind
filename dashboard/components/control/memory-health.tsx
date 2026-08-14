"use client";

import { useState } from "react";
import { api, MemoryStats } from "@/lib/api";
import { useT } from "@/lib/i18n";
import { ArchiveRestore, Loader2, BrainCircuit, AlertTriangle } from "lucide-react";

/**
 * MemoryHealth - how much of what the fleet has learned is actually reachable.
 *
 * Episodic memory is the product's core claim, and it can silently go missing:
 * the A/B experiment archives every memory for its cold phase, and one aborted
 * run left the fleet operating on 15 of 115 memories for a day — escalation
 * jumped to 87% and nothing on the dashboard said why. This bar makes that
 * state impossible to miss, and the restore button makes it a one-click fix.
 *
 * Shown only when something is actually archived; a healthy brain needs no
 * banner.
 */
export function MemoryHealth({
  stats,
  onRestored,
}: {
  stats?: MemoryStats;
  onRestored?: () => void;
}) {
  const t = useT();
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<string | null>(null);

  if (!stats) return null;
  const active = stats.active_cases ?? 0;
  const archived = stats.archived_cases ?? 0;
  const total = active + archived;
  if (total === 0 || archived === 0) return null;

  const archivedPct = Math.round((archived / total) * 100);
  // Less than half the knowledge reachable = the fleet is deciding half-blind.
  const degraded = archived > active;

  const restoreAll = async () => {
    setBusy(true);
    setMsg(null);
    try {
      // One atomic job on a current API...
      await api.memoryJob("unarchive_all");
      setMsg(t("Restored {n} memories to the vector index").replace("{n}", String(archived)));
    } catch {
      // ...or memory-by-memory against an older deployment that predates the job.
      try {
        const rows = (await api.getMemories({ limit: 500, archived: true })) ?? [];
        let n = 0;
        for (const m of rows) {
          try {
            await api.memoryAction("unarchive", m.id);
            n++;
          } catch {
            // keep going; the count reports what actually succeeded
          }
        }
        setMsg(t("Restored {n} memories to the vector index").replace("{n}", String(n)));
      } catch (e) {
        setMsg(`${t("Failed")}: ${(e as Error).message}`);
      }
    } finally {
      setBusy(false);
      onRestored?.();
    }
  };

  return (
    <div
      className={`rounded-lg border px-4 py-3 ${
        degraded ? "border-yellow/40 bg-yellow/8" : "border-border bg-bg-panel/70"
      }`}
    >
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <span className={degraded ? "text-yellow" : "text-text-tertiary"}>
          {degraded ? <AlertTriangle size={17} /> : <BrainCircuit size={17} />}
        </span>
        <div className="min-w-0 flex-1">
          <div className={`text-[14px] font-semibold ${degraded ? "text-yellow" : "text-text-primary"}`}>
            {degraded
              ? t("{p}% of learned knowledge is archived and unreachable").replace(
                  "{p}",
                  String(archivedPct)
                )
              : t("Memory health")}
          </div>
          <div className="text-[13px] text-text-secondary mt-0.5">
            {msg ??
              t("{a} memories answer recall queries · {z} sit archived outside the vector index")
                .replace("{a}", String(active))
                .replace("{z}", String(archived))}
          </div>
          {/* Active vs archived, as one bar */}
          <div className="mt-2 h-2 rounded-full bg-bg-inset overflow-hidden flex">
            <span className="h-full bg-blue" style={{ width: `${(active / total) * 100}%` }} />
            <span
              className={`h-full ${degraded ? "bg-yellow/70" : "bg-border-strong"}`}
              style={{ width: `${(archived / total) * 100}%` }}
            />
          </div>
        </div>
        <button
          onClick={restoreAll}
          disabled={busy}
          className={`shrink-0 flex items-center gap-2 px-3.5 py-2 rounded-md text-[14px] font-semibold border transition-colors disabled:opacity-40 ${
            degraded
              ? "bg-yellow/12 text-yellow border-yellow/30 hover:bg-yellow/20"
              : "bg-blue/12 text-blue border-blue/30 hover:bg-blue/20"
          }`}
        >
          {busy ? <Loader2 size={14} className="animate-spin" /> : <ArchiveRestore size={14} />}
          {t("Restore all archived")}
        </button>
      </div>
    </div>
  );
}
