"use client";

import { AuditStep } from "@/lib/api";
import {
  Brain,
  Database,
  Cpu,
  ShieldAlert,
  ShieldCheck,
  Flag,
  Zap,
  RotateCcw,
  User,
  CircleDot,
} from "lucide-react";
import { ReactNode } from "react";

/**
 * DecisionTrace - the X-ray of one verdict. Renders the append-only audit trail
 * as an anatomy of the decision: which memories were recalled (with real vector
 * similarity), what the model was told, what it cost, and how the case ended -
 * including crash/resume events, so resilience shows up in the same timeline.
 */

interface StepMeta {
  icon: ReactNode;
  tone: string; // border + text tone classes
  label: string;
}

function metaFor(action: string): StepMeta {
  switch (action) {
    case "task_claimed":
      return { icon: <CircleDot size={13} />, tone: "border-border-strong text-text-secondary", label: "Task claimed" };
    case "mcp_query":
      return { icon: <Database size={13} />, tone: "border-teal/60 text-teal", label: "Customer context (MCP)" };
    case "memory_recall":
      return { icon: <Brain size={13} />, tone: "border-blue/60 text-blue", label: "Episodic memory recall" };
    case "bedrock_reasoning":
      return { icon: <Cpu size={13} />, tone: "border-purple/60 text-purple", label: "Bedrock reasoning" };
    case "verdict_fraud":
      return { icon: <ShieldAlert size={13} />, tone: "border-red/60 text-red", label: "Verdict: FRAUD" };
    case "verdict_legit":
      return { icon: <ShieldCheck size={13} />, tone: "border-green/60 text-green", label: "Verdict: LEGIT" };
    case "verdict_escalate":
      return { icon: <Flag size={13} />, tone: "border-yellow/60 text-yellow", label: "Verdict: ESCALATE" };
    case "auto_approve":
      return { icon: <ShieldCheck size={13} />, tone: "border-green/60 text-green", label: "Auto-approved" };
    case "auto_block":
      return { icon: <ShieldAlert size={13} />, tone: "border-red/60 text-red", label: "Auto-blocked" };
    case "task_requeued":
      return { icon: <Zap size={13} />, tone: "border-yellow/60 text-yellow", label: "Crash detected - re-queued" };
    case "task_resumed":
      return { icon: <RotateCcw size={13} />, tone: "border-orange/60 text-orange", label: "Resumed from checkpoint" };
    case "task_failed":
      return { icon: <Zap size={13} />, tone: "border-red/60 text-red", label: "Task failed" };
    case "human_reviewed":
      return { icon: <User size={13} />, tone: "border-blue/60 text-blue", label: "Human review" };
    default:
      return { icon: <CircleDot size={13} />, tone: "border-border-strong text-text-secondary", label: action.replace(/_/g, " ") };
  }
}

const pct = (s: number) => `${Math.max(0, Math.min(100, Math.round(s * 100)))}%`;

function shortModel(m?: string | null) {
  if (!m) return null;
  if (m.includes("haiku")) return "claude-haiku";
  if (m.includes("sonnet")) return "claude-sonnet";
  return m.split(":")[0];
}

export function DecisionTrace({ steps }: { steps: AuditStep[] }) {
  return (
    <ol className="relative space-y-0">
      {steps.map((step, i) => {
        const m = metaFor(step.action);
        const model = step.action === "bedrock_reasoning" ? shortModel(step.bedrock_model) : null;
        const isLast = i === steps.length - 1;
        return (
          <li key={i} className="relative flex gap-3 pb-4 last:pb-0">
            {/* Rail */}
            {!isLast && (
              <span className="absolute left-[13px] top-7 bottom-0 w-px bg-border" aria-hidden />
            )}
            {/* Icon bubble */}
            <span
              className={`relative z-[1] mt-0.5 w-[27px] h-[27px] shrink-0 rounded-full border bg-bg-panel flex items-center justify-center ${m.tone}`}
            >
              {m.icon}
            </span>

            <div className="min-w-0 flex-1">
              <div className="flex items-baseline justify-between gap-2">
                <span className="text-[14px] font-semibold text-text-primary">{m.label}</span>
                <span className="text-[11px] text-text-tertiary tnum shrink-0">
                  {new Date(step.created_at).toLocaleTimeString()}
                </span>
              </div>

              {/* Memory recall: real vector similarities */}
              {step.action === "memory_recall" && (
                <div className="mt-1 flex flex-wrap items-center gap-1.5">
                  <span className="text-[12px] text-text-secondary tnum">
                    {step.memory_hits ?? 0} similar case{(step.memory_hits ?? 0) === 1 ? "" : "s"}
                  </span>
                  {(step.similarity_scores ?? []).map((s, j) => (
                    <span
                      key={j}
                      className="tnum text-[11px] px-1.5 py-0.5 rounded border border-blue/30 bg-blue/10 text-blue"
                      title={`vector similarity of recalled case #${j + 1}`}
                    >
                      {pct(s)}
                    </span>
                  ))}
                  {(step.memory_hits ?? 0) === 0 && (
                    <span className="text-[11px] text-text-tertiary">
                      cold case - nothing similar in memory yet
                    </span>
                  )}
                </div>
              )}

              {step.reasoning && (
                <div className="mt-1 text-[13px] text-text-secondary leading-relaxed">
                  {step.reasoning}
                </div>
              )}

              {/* Cost / model line */}
              {(model || step.tokens_in != null || step.latency_ms != null) && (
                <div className="mt-1 flex flex-wrap gap-1.5">
                  {model && (
                    <span className="text-[11px] px-1.5 py-0.5 rounded border border-purple/30 bg-purple/10 text-purple">
                      {model}
                    </span>
                  )}
                  {step.tokens_in != null && (
                    <span className="tnum text-[11px] px-1.5 py-0.5 rounded border border-border text-text-tertiary">
                      {step.tokens_in.toLocaleString()} in / {(step.tokens_out ?? 0).toLocaleString()} out tok
                    </span>
                  )}
                  {step.latency_ms != null && (
                    <span className="tnum text-[11px] px-1.5 py-0.5 rounded border border-border text-text-tertiary">
                      {step.latency_ms.toLocaleString()} ms
                    </span>
                  )}
                </div>
              )}
            </div>
          </li>
        );
      })}
    </ol>
  );
}
