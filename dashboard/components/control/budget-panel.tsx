"use client";

import { useEffect, useState } from "react";
import { api, Policy } from "@/lib/api";
import { useLive } from "@/lib/use-live";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { ShieldAlert, Save, Loader2 } from "lucide-react";

/**
 * BudgetPanel - a real cost guardrail, not a warning label. Set a daily cap;
 * with auto-pause on, the control plane disables the fleet's schedules the
 * moment today's Bedrock spend crosses it.
 */
export function BudgetPanel() {
  const { data } = useLive(api.getBudget, 15000);
  const [policy, setPolicy] = useState<Policy | null>(null);
  const [cap, setCap] = useState<number>(5);
  const [auto, setAuto] = useState(false);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<string | null>(null);

  useEffect(() => {
    api
      .getPolicy()
      .then((p) => {
        setPolicy(p);
        setCap(p.daily_budget_usd);
        setAuto(p.auto_pause_on_budget);
      })
      .catch(() => {});
  }, []);

  const save = async () => {
    if (!policy) return;
    setBusy(true);
    setMsg(null);
    try {
      const saved = await api.setPolicy({ ...policy, daily_budget_usd: cap, auto_pause_on_budget: auto });
      setPolicy(saved);
      setMsg(auto ? "Guardrail armed - the fleet pauses itself if the cap is crossed." : "Budget saved.");
    } catch (e) {
      setMsg((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const pct = Math.min(100, data?.used_pct ?? 0);
  const tone = pct >= 100 ? "red" : pct >= 75 ? "yellow" : "green";
  const barColor = tone === "red" ? "bg-red" : tone === "yellow" ? "bg-yellow" : "bg-green";

  return (
    <Panel
      title="Budget guardrail"
      subtitle="daily Bedrock spend cap - enforced, not advisory"
      actions={
        data ? (
          <Badge variant={data.exceeded ? "red" : "green"} dot>
            {data.exceeded ? "cap exceeded" : `${pct.toFixed(0)}% of cap`}
          </Badge>
        ) : null
      }
    >
      <div className="flex items-baseline gap-2">
        <span className="tnum text-2xl font-bold text-text-primary">
          ${(data?.spend_today ?? 0).toFixed(4)}
        </span>
        <span className="text-[13px] text-text-tertiary">
          of ${(data?.daily_budget_usd ?? cap).toFixed(2)} today
        </span>
      </div>
      <div className="h-2 rounded-full bg-bg-inset overflow-hidden mt-2">
        <div className={`h-full ${barColor} transition-[width] duration-500`} style={{ width: `${pct}%` }} />
      </div>

      {data?.paused_by_rule && (
        <div className="mt-3 flex items-start gap-2 text-[13px] text-red">
          <ShieldAlert size={15} className="shrink-0 mt-0.5" />
          <span>Cap crossed - the control plane disabled the fleet schedules automatically.</span>
        </div>
      )}

      <div className="flex flex-wrap items-center gap-3 mt-4 pt-3 border-t border-border">
        <label className="flex items-center gap-2 text-[13px] text-text-secondary">
          Daily cap ($)
          <input
            type="number"
            min={0}
            max={1000}
            step={0.5}
            value={cap}
            onChange={(e) => setCap(Math.max(0, Math.min(1000, Number(e.target.value) || 0)))}
            className="w-24 bg-bg-inset border border-border rounded-md px-2.5 py-1.5 tnum text-[13px] text-text-primary outline-none focus:border-blue"
          />
        </label>
        <label className="flex items-center gap-2 text-[13px] text-text-secondary cursor-pointer">
          <input type="checkbox" checked={auto} onChange={(e) => setAuto(e.target.checked)} className="accent-[#7d9bf0]" />
          Auto-pause the fleet when exceeded
        </label>
        <button
          onClick={save}
          disabled={busy || !policy}
          className="flex items-center gap-1.5 px-3.5 py-2 rounded-md bg-blue/12 text-blue border border-blue/30 text-[13px] font-semibold hover:bg-blue/20 disabled:opacity-40 transition-colors ml-auto"
        >
          {busy ? <Loader2 size={13} className="animate-spin" /> : <Save size={13} />}
          Save guardrail
        </button>
      </div>

      {msg && <div className="mt-2 text-[13px] text-text-secondary">{msg}</div>}
    </Panel>
  );
}
