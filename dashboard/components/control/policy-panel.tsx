"use client";

import { useEffect, useState } from "react";
import { api, Policy } from "@/lib/api";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { SlidersHorizontal, Save, Loader2, RotateCcw } from "lucide-react";

/**
 * PolicyPanel - the operating parameters of the fleet, editable while it runs.
 * The agent re-reads these on every task, so widening the auto-decide bands or
 * switching the model-failure behaviour takes effect within seconds - no deploy.
 */
export function PolicyPanel() {
  const [p, setP] = useState<Policy | null>(null);
  const [draft, setDraft] = useState<Policy | null>(null);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<string | null>(null);
  const [who, setWho] = useState("");

  useEffect(() => {
    api
      .getPolicy()
      .then((v) => {
        setP(v);
        setDraft(v);
      })
      .catch((e) => setMsg((e as Error).message));
  }, []);

  if (!draft || !p) {
    return (
      <Panel title="Agent policy" subtitle="operating parameters, applied live">
        <div className="h-16 hm-skeleton" />
        {msg && <div className="text-[13px] text-red mt-2">{msg}</div>}
      </Panel>
    );
  }

  const dirty = JSON.stringify(draft) !== JSON.stringify(p);
  const set = <K extends keyof Policy>(k: K, v: Policy[K]) => setDraft({ ...draft, [k]: v });

  const save = async () => {
    setBusy(true);
    setMsg(null);
    try {
      const saved = await api.setPolicy({ ...draft, updated_by: who.trim() });
      setP(saved);
      setDraft(saved);
      setMsg("Policy applied - agents pick it up within seconds.");
    } catch (e) {
      setMsg((e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  // Investigate band = the slice of traffic the agent actually reasons about.
  const bandPct = Math.max(0, (draft.risk_high - draft.risk_low) * 100);

  return (
    <Panel
      title="Agent policy"
      subtitle="how the fleet decides - editable while it runs"
      defaultCollapsed
      actions={
        <div className="flex items-center gap-2">
          {dirty && <Badge variant="yellow">unsaved</Badge>}
          <SlidersHorizontal size={15} className="text-text-tertiary" />
        </div>
      }
    >
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-x-8 gap-y-4">
        <Slider
          label="Auto-approve below risk"
          hint="anything under this is cleared without an agent call"
          value={draft.risk_low}
          min={0}
          max={0.5}
          step={0.001}
          onChange={(v) => set("risk_low", Math.min(v, draft.risk_high - 0.001))}
          display={draft.risk_low.toFixed(3)}
          tone="green"
        />
        <Slider
          label="Auto-block above risk"
          hint="anything over this is blocked without an agent call"
          value={draft.risk_high}
          min={0.5}
          max={1}
          step={0.001}
          onChange={(v) => set("risk_high", Math.max(v, draft.risk_low + 0.001))}
          display={draft.risk_high.toFixed(3)}
          tone="red"
        />
        <Slider
          label="Workers per dispatch"
          hint="fan-out of the fleet on each dispatch cycle"
          value={draft.dispatch_batch}
          min={1}
          max={50}
          step={1}
          onChange={(v) => set("dispatch_batch", Math.round(v))}
          display={`${draft.dispatch_batch}`}
          tone="blue"
        />
        <div className="flex flex-col gap-1.5">
          <span className="text-[13px] font-semibold text-text-primary">On model failure</span>
          <span className="text-[12px] text-text-tertiary">
            what to do when Bedrock is unavailable and the verdict would come from the rule-based
            fallback
          </span>
          <div className="flex gap-2 mt-1">
            {(["escalate", "requeue"] as const).map((v) => (
              <button
                key={v}
                onClick={() => set("fallback_action", v)}
                className={`px-3 py-1.5 rounded-md border text-[13px] font-semibold transition-colors ${
                  draft.fallback_action === v
                    ? "border-blue/40 bg-blue/12 text-blue"
                    : "border-border text-text-secondary hover:text-text-primary"
                }`}
              >
                {v === "escalate" ? "Send to human review" : "Retry later (re-queue)"}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div className="mt-4 pt-3 border-t border-border flex flex-wrap items-center gap-3">
        <span className="text-[13px] text-text-tertiary">
          Agent investigates the middle{" "}
          <span className="tnum text-text-primary font-semibold">{bandPct.toFixed(1)}%</span> risk
          band
        </span>
        <input
          value={who}
          onChange={(e) => setWho(e.target.value)}
          placeholder="your name (audit)"
          className="w-40 bg-bg-inset border border-border rounded-md px-2.5 py-1.5 text-[13px] text-text-primary outline-none focus:border-blue ml-auto"
        />
        <button
          onClick={() => setDraft(p)}
          disabled={!dirty || busy}
          className="flex items-center gap-1.5 px-3 py-2 rounded-md border border-border text-[13px] text-text-secondary hover:text-text-primary disabled:opacity-40 transition-colors"
        >
          <RotateCcw size={13} />
          Reset
        </button>
        <button
          onClick={save}
          disabled={!dirty || busy}
          className="flex items-center gap-1.5 px-4 py-2 rounded-md bg-blue/12 text-blue border border-blue/30 text-[13px] font-semibold hover:bg-blue/20 disabled:opacity-40 transition-colors"
        >
          {busy ? <Loader2 size={13} className="animate-spin" /> : <Save size={13} />}
          Apply policy
        </button>
      </div>

      {msg && <div className="mt-2 text-[13px] text-text-secondary">{msg}</div>}
      {p.updated_by && (
        <div className="mt-1 text-[12px] text-text-tertiary">
          last changed by {p.updated_by}
          {p.updated_at ? ` · ${new Date(p.updated_at).toLocaleString()}` : ""}
        </div>
      )}
    </Panel>
  );
}

function Slider({
  label,
  hint,
  value,
  min,
  max,
  step,
  onChange,
  display,
  tone,
}: {
  label: string;
  hint: string;
  value: number;
  min: number;
  max: number;
  step: number;
  onChange: (v: number) => void;
  display: string;
  tone: "green" | "red" | "blue";
}) {
  const accent = tone === "green" ? "accent-[#57bd8d]" : tone === "red" ? "accent-[#ec7d90]" : "accent-[#7d9bf0]";
  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-baseline justify-between gap-2">
        <span className="text-[13px] font-semibold text-text-primary">{label}</span>
        <span className="tnum text-[14px] font-semibold text-text-primary">{display}</span>
      </div>
      <span className="text-[12px] text-text-tertiary">{hint}</span>
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className={`w-full mt-1 ${accent}`}
      />
    </div>
  );
}
