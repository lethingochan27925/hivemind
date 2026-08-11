"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { useLive } from "@/lib/use-live";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import { Zap, AlertTriangle, Check, Loader2, RotateCcw } from "lucide-react";

const alarmTone = (s: string) =>
  s === "OK" ? "green" : s === "ALARM" ? "red" : s === "INSUFFICIENT_DATA" ? "yellow" : "default";

const lambdaStateTone = (s: string) => (s === "Active" ? "green" : s === "Pending" ? "yellow" : "red");

interface ChaosRun {
  taskId: string;
  startedAt: number; // epoch ms when the kill was fired
}

export default function InfrastructurePage() {
  const { data, error, lastUpdated } = useLive(api.getInfrastructure, 5000);
  const lambdas = useLive(api.getLambdas, 20000);
  const [simResult, setSimResult] = useState<string | null>(null);
  const [simulating, setSimulating] = useState(false);
  const [chaos, setChaos] = useState<ChaosRun | null>(null);

  const simulate = async () => {
    setSimulating(true);
    setSimResult(null);
    try {
      const res = await api.simulateCrash();
      setChaos({ taskId: res.task_id, startedAt: Date.now() });
      setSimResult(null);
    } catch (e) {
      setChaos(null);
      setSimResult(
        `No in-flight task to crash right now - feed cases on Mission Control first. (${(e as Error).message})`
      );
    } finally {
      setSimulating(false);
    }
  };

  const services = data?.services ?? [];
  const okCount = services.filter((s) => s.alarm_state === "OK").length;

  // Recovery tracker: match this chaos run's task against the live incident feed.
  const chaosEvents = (data?.incidents ?? []).filter((i) => i.task_id === chaos?.taskId);
  const requeuedAt = chaosEvents.find((e) => e.action === "task_requeued")?.timestamp;
  const resumedAt = chaosEvents.find((e) => e.action === "task_resumed")?.timestamp;
  const secsFromKill = (ts?: string) =>
    chaos && ts ? Math.max(0, Math.round((new Date(ts).getTime() - chaos.startedAt) / 1000)) : null;

  return (
    <div>
      <PageHeader
        title="Infrastructure"
        description="Service health and resilience - how the fleet recovers from failure"
        lastUpdated={lastUpdated}
        error={error}
        actions={
          services.length > 0 ? (
            <Badge variant={okCount === services.length ? "green" : "yellow"} dot>
              {okCount}/{services.length} healthy
            </Badge>
          ) : null
        }
      />

      <div className="p-6 space-y-5 hm-enter">
        <Panel title="Chaos test">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div className="flex items-start gap-2 text-[14px] text-text-secondary max-w-2xl">
              <AlertTriangle size={15} className="text-yellow shrink-0 mt-0.5" />
              <span>
                Backdates a running task&apos;s heartbeat to simulate an agent crash. The Heartbeat
                Reaper detects the stale lease and re-queues the task on its next cycle - a fresh
                agent resumes from the last checkpoint in CockroachDB. Watch{" "}
                <span className="text-text-primary">Live fleet</span> on Mission Control.
              </span>
            </div>
            <button
              onClick={simulate}
              disabled={simulating}
              className="flex items-center gap-1.5 bg-yellow/12 text-yellow border border-yellow/30 rounded-md px-3 py-2 text-[14px] font-medium disabled:opacity-40 hover:bg-yellow/20 transition-colors shrink-0"
            >
              <Zap size={13} />
              {simulating ? "Simulating…" : "Simulate agent crash"}
            </button>
          </div>
          {simResult && (
            <div className="mt-3 text-[13px] text-text-secondary border-t border-border pt-2">
              {simResult}
            </div>
          )}

          {/* Live recovery tracker - crash -> reaper re-queue -> resume from checkpoint */}
          {chaos && (
            <div className="mt-3 border-t border-border pt-3">
              <div className="text-[13px] text-text-tertiary mb-2">
                Recovery of task <span className="tnum text-text-secondary">{chaos.taskId.slice(0, 8)}</span> - watched
                live from the audit log
              </div>
              <ol className="flex flex-col sm:flex-row gap-2 sm:gap-0 sm:items-stretch">
                <TrackStage
                  done
                  icon={<Zap size={13} />}
                  label="Agent killed"
                  detail="heartbeat backdated"
                  tone="yellow"
                />
                <StageArrow done={!!requeuedAt} />
                <TrackStage
                  done={!!requeuedAt}
                  icon={requeuedAt ? <Check size={13} /> : <Loader2 size={13} className="animate-spin" />}
                  label="Reaper re-queued"
                  detail={requeuedAt ? `+${secsFromKill(requeuedAt)}s after kill` : "waiting for 30s sweep…"}
                  tone={requeuedAt ? "green" : "default"}
                />
                <StageArrow done={!!resumedAt} />
                <TrackStage
                  done={!!resumedAt}
                  icon={resumedAt ? <RotateCcw size={13} /> : <Loader2 size={13} className="animate-spin" />}
                  label="Resumed from checkpoint"
                  detail={
                    resumedAt
                      ? `+${secsFromKill(resumedAt)}s - scratchpad read, no work lost`
                      : requeuedAt
                        ? "waiting for a worker to claim…"
                        : "…"
                  }
                  tone={resumedAt ? "green" : "default"}
                />
              </ol>
              {resumedAt && (
                <div className="mt-2 text-[13px] text-green">
                  Crash absorbed in {secsFromKill(resumedAt)}s. Durable working memory made the failure a non-event.
                </div>
              )}
            </div>
          )}
        </Panel>

        <Panel title="Service health" subtitle="CloudWatch error alarms" bodyClassName="p-0">
          {services.length > 0 ? (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 divide-y sm:divide-y-0 divide-border">
              {services.map((s, i) => (
                <div
                  key={s.service}
                  className={`flex items-center justify-between px-3 py-2.5 border-border ${
                    i % 3 !== 2 ? "lg:border-r" : ""
                  } ${i % 2 !== 1 ? "sm:border-r lg:border-r" : ""} border-b`}
                >
                  <span className="text-[14px] text-text-secondary">{s.service}</span>
                  <Badge variant={alarmTone(s.alarm_state)} dot>
                    {s.alarm_state}
                  </Badge>
                </div>
              ))}
            </div>
          ) : (
            <EmptyState message="No service data available" />
          )}
        </Panel>

        <Panel title="Nodes" subtitle="every Lambda in the fleet - live config" bodyClassName="p-0">
          {lambdas.data && lambdas.data.length > 0 ? (
            <div className="overflow-x-auto">
              <table className="w-full text-[14px]">
                <thead>
                  <tr className="text-left text-text-tertiary border-b border-border bg-bg-inset/40">
                    <th className="py-2.5 px-4 font-normal">Function</th>
                    <th className="py-2.5 px-4 font-normal">State</th>
                    <th className="py-2.5 px-4 font-normal text-right">Version</th>
                    <th className="py-2.5 px-4 font-normal text-right">Memory</th>
                    <th className="py-2.5 px-4 font-normal text-right">Timeout</th>
                    <th className="py-2.5 px-4 font-normal">Package</th>
                  </tr>
                </thead>
                <tbody>
                  {lambdas.data.map((l) => (
                    <tr key={l.service} className="border-b border-border/40">
                      <td className="py-2.5 px-4 text-text-secondary">{l.service}</td>
                      <td className="py-2.5 px-4">
                        <Badge variant={lambdaStateTone(l.state)} dot>
                          {l.state}
                        </Badge>
                      </td>
                      <td className="py-2.5 px-4 tnum text-right text-text-secondary">{l.version}</td>
                      <td className="py-2.5 px-4 tnum text-right text-text-primary">{l.memory_mb} MB</td>
                      <td className="py-2.5 px-4 tnum text-right text-text-secondary">{l.timeout_sec}s</td>
                      <td className="py-2.5 px-4 text-text-tertiary">{l.runtime}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <EmptyState message="Loading node configuration…" />
          )}
        </Panel>

        <Panel title="Incident timeline" subtitle="crash re-queues and checkpoint resumes, last 24h">
          {data?.incidents && data.incidents.length > 0 ? (
            <ol className="space-y-2.5">
              {data.incidents.map((inc, i) => (
                <li
                  key={i}
                  className={`border-l-2 pl-3 py-0.5 ${
                    inc.action === "task_resumed" ? "border-green/50" : "border-yellow/50"
                  }`}
                >
                  <div className="text-[14px] text-text-primary">
                    {inc.action === "task_resumed"
                      ? "Task resumed from checkpoint"
                      : "Task re-queued after crash"}{" "}
                    - <span className="tnum">{inc.service}</span>
                    {inc.task_id && (
                      <span className="tnum text-text-tertiary"> · {inc.task_id.slice(0, 8)}</span>
                    )}
                  </div>
                  {inc.description && (
                    <div className="text-[13px] text-text-secondary mt-0.5">{inc.description}</div>
                  )}
                  <div className="text-[12px] text-text-tertiary mt-0.5 tnum">
                    {new Date(inc.timestamp).toLocaleString()}
                  </div>
                </li>
              ))}
            </ol>
          ) : (
            <EmptyState
              message="No incidents in the last 24 hours"
              hint="When the reaper rescues a crashed agent's task, it appears here with an audit record."
            />
          )}
        </Panel>
      </div>
    </div>
  );
}

// --- Chaos recovery tracker pieces ------------------------------------------

function TrackStage({
  done,
  icon,
  label,
  detail,
  tone,
}: {
  done: boolean;
  icon: React.ReactNode;
  label: string;
  detail: string;
  tone: "yellow" | "green" | "default";
}) {
  const toneCls =
    tone === "green"
      ? "border-green/40 text-green"
      : tone === "yellow"
        ? "border-yellow/40 text-yellow"
        : "border-border text-text-tertiary";
  return (
    <li
      className={`flex-1 rounded-lg border bg-bg-inset/40 px-3 py-2 flex items-start gap-2 ${
        done ? toneCls : "border-border text-text-tertiary"
      }`}
    >
      <span className="mt-0.5 shrink-0">{icon}</span>
      <span className="min-w-0">
        <span className="block text-[13px] font-semibold text-text-primary">{label}</span>
        <span className="block text-[12px] tnum">{detail}</span>
      </span>
    </li>
  );
}

function StageArrow({ done }: { done: boolean }) {
  return (
    <span
      aria-hidden
      className={`self-center px-1.5 text-[13px] ${done ? "text-green" : "text-text-tertiary"}`}
    >
      →
    </span>
  );
}
