"use client";

import { useLive } from "@/lib/use-live";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import { ExternalLink, GitBranch, ArrowRight, Undo2, Loader2 } from "lucide-react";
import { useState } from "react";
import { api } from "@/lib/api";

/**
 * Pipeline - the delivery half of the platform. Live GitHub Actions state for
 * the whole CI/CD chain (verify -> build -> stage -> smoke -> canary) plus the
 * side lanes (security, dashboard CD, terraform). Public API, no token needed.
 */

const REPO = process.env.NEXT_PUBLIC_GITHUB_REPO || "lethingochan27925/hivemind";

interface GhRun {
  id: number;
  name: string;
  display_title: string;
  status: string; // queued | in_progress | completed
  conclusion: string | null; // success | failure | cancelled | ...
  html_url: string;
  run_number: number;
  head_branch: string;
  run_started_at: string;
  updated_at: string;
  event: string;
}

const fetchRuns = async (): Promise<GhRun[]> => {
  const res = await fetch(
    `https://api.github.com/repos/${REPO}/actions/runs?per_page=40`,
    { cache: "no-store" }
  );
  if (res.status === 403) throw new Error("GitHub API rate limit reached - retrying soon");
  if (!res.ok) throw new Error(`GitHub API ${res.status}`);
  const body = await res.json();
  return (body.workflow_runs ?? []) as GhRun[];
};

// The release chain, in order, plus independent lanes.
const CHAIN = ["CI", "Build and Push Images", "Deploy Staging", "Smoke Test", "Deploy Canary Production"];
const LANES = ["Security", "Deploy Dashboard", "Terraform Infrastructure"];

function tone(run?: GhRun): "green" | "red" | "yellow" | "default" {
  if (!run) return "default";
  if (run.status !== "completed") return "yellow";
  if (run.conclusion === "success") return "green";
  if (run.conclusion === "failure" || run.conclusion === "startup_failure") return "red";
  return "default";
}

function statusLabel(run?: GhRun): string {
  if (!run) return "no runs";
  if (run.status !== "completed") return run.status.replace("_", " ");
  return run.conclusion ?? "unknown";
}

function duration(run: GhRun): string {
  const ms = new Date(run.updated_at).getTime() - new Date(run.run_started_at).getTime();
  if (!Number.isFinite(ms) || ms <= 0) return "—";
  const s = Math.round(ms / 1000);
  return s < 60 ? `${s}s` : `${Math.floor(s / 60)}m ${s % 60}s`;
}

function ago(iso: string): string {
  const s = Math.round((Date.now() - new Date(iso).getTime()) / 1000);
  if (s < 60) return `${s}s ago`;
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
}

function StageChip({ name, run }: { name: string; run?: GhRun }) {
  const t = tone(run);
  const border =
    t === "green" ? "border-green/40" : t === "red" ? "border-red/50" : t === "yellow" ? "border-yellow/40" : "border-border";
  const body = (
    <div className={`rounded-lg border ${border} bg-bg-panel px-3 py-2 min-w-[130px]`}>
      <div className="text-[13px] font-semibold text-text-primary leading-tight">{name}</div>
      <div className="mt-1 flex items-center gap-1.5">
        <Badge variant={t} dot>
          {statusLabel(run)}
        </Badge>
        {run && <span className="tnum text-[11px] text-text-tertiary">{ago(run.updated_at)}</span>}
      </div>
    </div>
  );
  return run ? (
    <a href={run.html_url} target="_blank" rel="noreferrer" className="hover:opacity-90 transition-opacity">
      {body}
    </a>
  ) : (
    body
  );
}

export default function PipelinePage() {
  const { data, error, lastUpdated } = useLive(fetchRuns, 90000);
  const runs = data ?? [];

  const latestByWorkflow = new Map<string, GhRun>();
  for (const r of runs) {
    if (!latestByWorkflow.has(r.name)) latestByWorkflow.set(r.name, r);
  }

  const failing = [...latestByWorkflow.values()].filter((r) => tone(r) === "red").length;

  return (
    <div>
      <PageHeader
        title="Pipeline"
        description={`Live CI/CD state from GitHub Actions - ${REPO}`}
        lastUpdated={lastUpdated}
        error={error}
        actions={
          runs.length > 0 ? (
            <Badge variant={failing > 0 ? "red" : "green"} dot>
              {failing > 0 ? `${failing} workflow failing` : "all workflows green"}
            </Badge>
          ) : null
        }
      />

      <div className="p-6 space-y-5 hm-enter">
        {/* Release chain */}
        <Panel
          title="Release chain"
          subtitle="push to main -> verify -> build -> stage -> smoke -> canary (auto-rollback on alarm)"
          actions={<GitBranch size={15} className="text-text-tertiary" />}
        >
          <div className="flex flex-wrap items-center gap-2">
            {CHAIN.map((name, i) => (
              <div key={name} className="flex items-center gap-2">
                {i > 0 && <ArrowRight size={14} className="text-text-tertiary" />}
                <StageChip name={name} run={latestByWorkflow.get(name)} />
              </div>
            ))}
          </div>
          <div className="mt-4 pt-3 border-t border-border flex flex-wrap items-center gap-2">
            <span className="text-[11px] uppercase tracking-[0.14em] text-text-tertiary font-bold mr-1">
              Independent lanes
            </span>
            {LANES.map((name) => (
              <StageChip key={name} name={name} run={latestByWorkflow.get(name)} />
            ))}
          </div>
        </Panel>

        <RollbackPanel />

        {/* Recent runs */}
        <Panel title="Recent runs" subtitle="latest 15 workflow executions" bodyClassName="p-0">
          {runs.length === 0 ? (
            <EmptyState
              message={error ? "GitHub API unavailable" : "Loading pipeline history…"}
              hint={error ?? undefined}
            />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-[14px]">
                <thead>
                  <tr className="text-left text-text-tertiary border-b border-border bg-bg-inset/40">
                    <th className="py-2.5 px-4 font-normal">Workflow</th>
                    <th className="py-2.5 px-4 font-normal">Status</th>
                    <th className="py-2.5 px-4 font-normal">Commit</th>
                    <th className="py-2.5 px-4 font-normal">Branch</th>
                    <th className="py-2.5 px-4 font-normal text-right">Duration</th>
                    <th className="py-2.5 px-4 font-normal text-right">When</th>
                    <th className="py-2.5 px-4 font-normal" />
                  </tr>
                </thead>
                <tbody>
                  {runs.slice(0, 15).map((r) => (
                    <tr key={r.id} className="border-b border-border/40">
                      <td className="py-2.5 px-4 text-text-primary font-medium">{r.name}</td>
                      <td className="py-2.5 px-4">
                        <Badge variant={tone(r)} dot>
                          {statusLabel(r)}
                        </Badge>
                      </td>
                      <td className="py-2.5 px-4 text-text-secondary max-w-[320px] truncate">
                        {r.display_title}
                      </td>
                      <td className="py-2.5 px-4 tnum text-text-tertiary">{r.head_branch}</td>
                      <td className="py-2.5 px-4 tnum text-right text-text-secondary">{duration(r)}</td>
                      <td className="py-2.5 px-4 tnum text-right text-text-tertiary">{ago(r.updated_at)}</td>
                      <td className="py-2.5 px-4 text-right">
                        <a
                          href={r.html_url}
                          target="_blank"
                          rel="noreferrer"
                          className="inline-flex text-text-tertiary hover:text-text-primary"
                          title="Open on GitHub"
                        >
                          <ExternalLink size={14} />
                        </a>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>

        <div className="text-[12px] text-text-tertiary">
          Full pipeline design (canary weights, auto-rollback, OIDC): see{" "}
          <a
            href={`https://github.com/${REPO}/blob/main/docs/CICD.md`}
            target="_blank"
            rel="noreferrer"
            className="text-blue hover:underline"
          >
            docs/CICD.md
          </a>
          .
        </div>
      </div>
    </div>
  );
}

/**
 * RollbackPanel - the manual brake next to the automated canary: move a
 * function's `live` alias back to the previous published version, in one click.
 */
function RollbackPanel() {
  const SERVICES = ["agent-worker", "dispatcher", "reaper", "salience-decay", "scoring-api", "dashboard-api"];
  const [busy, setBusy] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);

  const rollback = async (svc: string) => {
    if (!confirm(`Roll ${svc} back to its previous published version?`)) return;
    setBusy(svc);
    setMsg(null);
    try {
      const r = await api.rollbackService(svc);
      setMsg(`${svc}: alias live moved v${r.from_version} → v${r.to_version}.`);
    } catch (e) {
      setMsg(`${svc}: ${(e as Error).message}`);
    } finally {
      setBusy(null);
    }
  };

  return (
    <Panel
      title="Deployment control"
      subtitle="manual rollback - move the live alias to the previous version"
      actions={<Undo2 size={15} className="text-text-tertiary" />}
    >
      <div className="flex flex-wrap gap-2">
        {SERVICES.map((svc) => (
          <button
            key={svc}
            disabled={busy !== null}
            onClick={() => rollback(svc)}
            className="flex items-center gap-1.5 px-3 py-2 rounded-md border border-border text-[13px] text-text-secondary hover:text-text-primary hover:border-border-strong disabled:opacity-40 transition-colors"
          >
            {busy === svc ? <Loader2 size={13} className="animate-spin" /> : <Undo2 size={13} />}
            {svc}
          </button>
        ))}
      </div>
      {msg && <div className="mt-3 text-[13px] text-text-secondary border-t border-border pt-2.5">{msg}</div>}
    </Panel>
  );
}
