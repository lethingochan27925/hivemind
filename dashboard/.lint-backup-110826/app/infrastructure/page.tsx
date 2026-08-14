"use client";

import { useEffect, useMemo, useState } from "react";
import { api, LambdaInfo, ResourceInfo } from "@/lib/api";
import { useLive } from "@/lib/use-live";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import {
  Zap,
  AlertTriangle,
  Check,
  Loader2,
  RotateCcw,
  Globe2,
  Plus,
  X,
  ExternalLink,
  Undo2,
  Play,
  Pause,
  ChevronRight,
} from "lucide-react";

const REGION = process.env.NEXT_PUBLIC_AWS_REGION || "ap-southeast-1";

const alarmTone = (s: string) =>
  s === "OK" ? "green" : s === "ALARM" ? "red" : s === "INSUFFICIENT_DATA" ? "yellow" : "default";

const lambdaStateTone = (s: string) => (s === "Active" ? "green" : s === "Pending" ? "yellow" : "red");

const SCHEDULED = ["agent-worker", "dispatcher", "reaper", "salience-decay"];

// Nhan doc duoc + link console cho tung dich vu AWS trong inventory.
const SERVICE_LABEL: Record<string, string> = {
  lambda: "Lambda functions",
  s3: "S3 buckets",
  cloudfront: "CloudFront",
  events: "EventBridge rules",
  cloudwatch: "CloudWatch alarms",
  logs: "Log groups",
  ssm: "SSM parameters",
  ecr: "ECR repositories",
  sns: "SNS topics",
  dynamodb: "DynamoDB tables",
  iam: "IAM roles",
};

function consoleUrl(service: string, name: string): string {
  const r = REGION;
  switch (service) {
    case "lambda":
      return `https://console.aws.amazon.com/lambda/home?region=${r}#/functions/${name}`;
    case "s3":
      return `https://s3.console.aws.amazon.com/s3/buckets/${name}`;
    case "cloudfront":
      return `https://console.aws.amazon.com/cloudfront/v4/home#/distributions`;
    case "events":
      return `https://console.aws.amazon.com/events/home?region=${r}#/eventbus/default/rules/${name}`;
    case "cloudwatch":
      return `https://console.aws.amazon.com/cloudwatch/home?region=${r}#alarmsV2:alarm/${name}`;
    case "logs":
      return `https://console.aws.amazon.com/cloudwatch/home?region=${r}#logsV2:log-groups`;
    case "ssm":
      return `https://console.aws.amazon.com/systems-manager/parameters?region=${r}`;
    case "ecr":
      return `https://console.aws.amazon.com/ecr/repositories?region=${r}`;
    case "sns":
      return `https://console.aws.amazon.com/sns/v3/home?region=${r}#/topics`;
    case "dynamodb":
      return `https://console.aws.amazon.com/dynamodbv2/home?region=${r}#tables`;
    case "iam":
      return `https://console.aws.amazon.com/iam/home#/roles/${name}`;
    default:
      return `https://console.aws.amazon.com/console/home?region=${r}`;
  }
}

interface ChaosRun {
  taskId: string;
  startedAt: number;
}

export default function InfrastructurePage() {
  const { data, error, lastUpdated } = useLive(api.getInfrastructure, 5000);
  const lambdas = useLive(api.getLambdas, 15000);
  const fleet = useLive(api.getFleetStatus, 10000);
  const resources = useLive(api.getResources, 60000);

  const [simResult, setSimResult] = useState<string | null>(null);
  const [simulating, setSimulating] = useState(false);
  const [chaos, setChaos] = useState<ChaosRun | null>(null);
  const [node, setNode] = useState<LambdaInfo | null>(null);
  const [svcOpen, setSvcOpen] = useState<string | null>(null);

  const services = data?.services ?? [];
  const okCount = services.filter((s) => s.alarm_state === "OK").length;
  const alarmOf = (svc: string) => services.find((s) => s.service === svc)?.alarm_state ?? "UNKNOWN";
  const scheduleOf = (svc: string) => fleet.data?.schedules.find((s) => s.service === svc)?.state;

  // Deep-link tu Architecture map: ?node=<lambda> mo node detail, ?res=<service>
  // mo dung nhom tai nguyen trong inventory.
  useEffect(() => {
    const q = new URLSearchParams(window.location.search);
    const wantNode = q.get("node");
    const wantRes = q.get("res");
    if (wantRes && !svcOpen) setSvcOpen(wantRes);
    if (wantNode && !node && lambdas.data) {
      const found = lambdas.data.find((l) => l.service === wantNode);
      if (found) setNode(found);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lambdas.data]);

  const simulate = async () => {
    setSimulating(true);
    setSimResult(null);
    try {
      const res = await api.simulateCrash();
      setChaos({ taskId: res.task_id, startedAt: Date.now() });
    } catch (e) {
      setChaos(null);
      setSimResult(`No in-flight task to crash right now - feed cases first. (${(e as Error).message})`);
    } finally {
      setSimulating(false);
    }
  };

  const chaosEvents = (data?.incidents ?? []).filter((i) => i.task_id === chaos?.taskId);
  const requeuedAt = chaosEvents.find((e) => e.action === "task_requeued")?.timestamp;
  const resumedAt = chaosEvents.find((e) => e.action === "task_resumed")?.timestamp;
  const secsFromKill = (ts?: string) =>
    chaos && ts ? Math.max(0, Math.round((new Date(ts).getTime() - chaos.startedAt) / 1000)) : null;

  // Inventory: gom moi tai nguyen Terraform da tag, theo dich vu.
  const grouped = useMemo(() => {
    const g: Record<string, ResourceInfo[]> = {};
    for (const r of resources.data ?? []) (g[r.service] ??= []).push(r);
    return g;
  }, [resources.data]);
  const svcKeys = Object.keys(grouped).sort((a, b) => grouped[b].length - grouped[a].length);
  const totalResources = (resources.data ?? []).length;

  return (
    <div>
      <PageHeader
        title="Infrastructure"
        description="Every deployed resource - health, configuration, and direct control"
        lastUpdated={lastUpdated}
        error={error}
        actions={
          <div className="flex items-center gap-2">
            {totalResources > 0 && (
              <Badge variant="blue" dot>
                {totalResources} resources
              </Badge>
            )}
            {services.length > 0 && (
              <Badge variant={okCount === services.length ? "green" : "yellow"} dot>
                {okCount}/{services.length} healthy
              </Badge>
            )}
          </div>
        }
      />

      <div className="p-6 space-y-5 hm-enter">
        {/* ---- Fleet: health + config + schedule + actions in ONE table ---- */}
        <Panel
          title="Compute fleet"
          subtitle="Lambda functions - alarm state, live config, schedule and controls"
          bodyClassName="p-0"
        >
          {!lambdas.data ? (
            <div className="p-3 space-y-2">
              {Array.from({ length: 6 }).map((_, i) => (
                <div key={i} className="h-6 hm-skeleton" />
              ))}
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-[14px]">
                <thead>
                  <tr className="text-left text-text-tertiary border-b border-border bg-bg-inset/40">
                    <th className="py-2.5 px-4 font-normal">Function</th>
                    <th className="py-2.5 px-3 font-normal">State</th>
                    <th className="py-2.5 px-3 font-normal">Alarm</th>
                    <th className="py-2.5 px-3 font-normal">Schedule</th>
                    <th className="py-2.5 px-3 font-normal text-right">Ver</th>
                    <th className="py-2.5 px-3 font-normal text-right">Memory</th>
                    <th className="py-2.5 px-3 font-normal text-right">Timeout</th>
                    <th className="py-2.5 px-4 font-normal text-right">Control</th>
                  </tr>
                </thead>
                <tbody>
                  {lambdas.data.map((l) => {
                    const sched = scheduleOf(l.service);
                    const selected = node?.service === l.service;
                    return (
                      <tr
                        key={l.service}
                        onClick={() => setNode(selected ? null : l)}
                        tabIndex={0}
                        onKeyDown={(e) => e.key === "Enter" && setNode(l)}
                        className={`cursor-pointer border-b border-border/40 outline-none ${
                          selected ? "bg-blue/10" : "hover:bg-bg-panel-hover/60 focus:bg-bg-panel-hover/60"
                        }`}
                      >
                        <td className="py-2.5 px-4 text-text-primary font-medium">{l.service}</td>
                        <td className="py-2.5 px-3">
                          <Badge variant={lambdaStateTone(l.state)} dot>
                            {l.state}
                          </Badge>
                        </td>
                        <td className="py-2.5 px-3">
                          <Badge variant={alarmTone(alarmOf(l.service))} dot>
                            {alarmOf(l.service)}
                          </Badge>
                        </td>
                        <td className="py-2.5 px-3">
                          {SCHEDULED.includes(l.service) ? (
                            <Badge variant={sched === "ENABLED" ? "green" : "default"}>
                              {sched ?? "…"}
                            </Badge>
                          ) : (
                            <span className="text-[12px] text-text-tertiary">on demand</span>
                          )}
                        </td>
                        <td className="py-2.5 px-3 tnum text-right text-text-secondary">{l.version}</td>
                        <td className="py-2.5 px-3 tnum text-right text-text-primary">{l.memory_mb} MB</td>
                        <td className="py-2.5 px-3 tnum text-right text-text-secondary">{l.timeout_sec}s</td>
                        <td className="py-2.5 px-4 text-right">
                          <span className="inline-flex items-center gap-1 text-[12px] text-text-tertiary">
                            open <ChevronRight size={12} />
                          </span>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </Panel>

        {node && (
          <NodeDetail
            node={node}
            alarm={alarmOf(node.service)}
            schedule={scheduleOf(node.service)}
            onClose={() => setNode(null)}
          />
        )}

        {/* ---- Every other AWS resource, grouped and clickable ---- */}
        <Panel
          title="Infrastructure inventory"
          subtitle="every resource Terraform tagged Project=hivemind - click a service to expand"
        >
          {svcKeys.length === 0 ? (
            <EmptyState message="Loading inventory…" />
          ) : (
            <div className="space-y-2">
              <div className="flex flex-wrap gap-2">
                {svcKeys.map((svc) => (
                  <button
                    key={svc}
                    onClick={() => setSvcOpen(svcOpen === svc ? null : svc)}
                    className={`flex items-center gap-2 px-3 py-2 rounded-lg border text-[13px] transition-colors ${
                      svcOpen === svc
                        ? "border-blue/40 bg-blue/10 text-blue"
                        : "border-border bg-bg-inset/40 text-text-secondary hover:text-text-primary hover:border-border-strong"
                    }`}
                  >
                    <span className="font-semibold">{SERVICE_LABEL[svc] ?? svc}</span>
                    <span className="tnum text-text-tertiary">{grouped[svc].length}</span>
                  </button>
                ))}
              </div>

              {svcOpen && grouped[svcOpen] && (
                <div className="mt-3 rounded-lg border border-border overflow-hidden">
                  <div className="flex items-center justify-between px-3.5 h-10 border-b border-border bg-bg-inset/60">
                    <span className="text-[13px] font-semibold text-text-primary">
                      {SERVICE_LABEL[svcOpen] ?? svcOpen}
                    </span>
                    <button onClick={() => setSvcOpen(null)} className="text-text-tertiary hover:text-text-primary">
                      <X size={14} />
                    </button>
                  </div>
                  <ul className="max-h-72 overflow-y-auto divide-y divide-border/50">
                    {grouped[svcOpen].map((r) => (
                      <li key={r.arn} className="flex items-center gap-3 px-3.5 py-2">
                        <span className="tnum text-[13px] text-text-secondary truncate flex-1" title={r.arn}>
                          {r.name}
                        </span>
                        {svcOpen === "lambda" && (
                          <button
                            onClick={() => {
                              const l = lambdas.data?.find((x) => r.name.endsWith(x.service));
                              if (l) setNode(l);
                            }}
                            className="text-[12px] text-blue hover:underline"
                          >
                            control
                          </button>
                        )}
                        <a
                          href={consoleUrl(svcOpen, r.name)}
                          target="_blank"
                          rel="noreferrer"
                          className="text-text-tertiary hover:text-text-primary"
                          title="Open in AWS console"
                        >
                          <ExternalLink size={13} />
                        </a>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          )}
        </Panel>

        <RegionsPanel />

        {/* ---- Chaos test ---- */}
        <Panel title="Chaos test">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div className="flex items-start gap-2 text-[14px] text-text-secondary max-w-2xl">
              <AlertTriangle size={15} className="text-yellow shrink-0 mt-0.5" />
              <span>
                Backdates a running task&apos;s heartbeat to simulate an agent crash. The Heartbeat
                Reaper detects the stale lease and re-queues the task - a fresh agent resumes from
                the last checkpoint in CockroachDB.
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
            <div className="mt-3 text-[13px] text-text-secondary border-t border-border pt-2">{simResult}</div>
          )}

          {chaos && (
            <div className="mt-3 border-t border-border pt-3">
              <div className="text-[13px] text-text-tertiary mb-2">
                Recovery of task <span className="tnum text-text-secondary">{chaos.taskId.slice(0, 8)}</span> -
                watched live from the audit log
              </div>
              <ol className="flex flex-col sm:flex-row gap-2 sm:gap-0 sm:items-stretch">
                <TrackStage done icon={<Zap size={13} />} label="Agent killed" detail="heartbeat backdated" tone="yellow" />
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
                  Crash absorbed in {secsFromKill(resumedAt)}s. Durable working memory made the failure a
                  non-event.
                </div>
              )}
            </div>
          )}
        </Panel>

        {/* ---- Incidents ---- */}
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
                    {inc.action === "task_resumed" ? "Task resumed from checkpoint" : "Task re-queued after crash"} -{" "}
                    <span className="tnum">{inc.service}</span>
                    {inc.task_id && <span className="tnum text-text-tertiary"> · {inc.task_id.slice(0, 8)}</span>}
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

// --- Node drill-down ---------------------------------------------------------

function NodeDetail({
  node,
  alarm,
  schedule,
  onClose,
}: {
  node: LambdaInfo;
  alarm: string;
  schedule?: string;
  onClose: () => void;
}) {
  const [busy, setBusy] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const scheduled = SCHEDULED.includes(node.service);

  const act = async (key: string, fn: () => Promise<unknown>, done: string) => {
    setBusy(key);
    setMsg(null);
    try {
      await fn();
      setMsg(done);
    } catch (e) {
      setMsg((e as Error).message);
    } finally {
      setBusy(null);
    }
  };

  const btn =
    "flex items-center gap-1.5 px-3 py-2 rounded-md text-[13px] font-semibold border disabled:opacity-40 transition-colors";

  return (
    <Panel
      title={`Node · ${node.service}`}
      subtitle="live configuration and direct actions on this component"
      actions={
        <button onClick={onClose} className="text-text-tertiary hover:text-text-primary" title="Close">
          <X size={15} />
        </button>
      }
    >
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3 text-[13px]">
        <KV label="State" value={node.state} tone={lambdaStateTone(node.state)} />
        <KV label="Alarm" value={alarm} tone={alarmTone(alarm)} />
        <KV label="Version" value={`v${node.version}`} />
        <KV label="Memory" value={`${node.memory_mb} MB`} />
        <KV label="Timeout" value={`${node.timeout_sec}s`} />
        <KV
          label="Schedule"
          value={schedule ?? (scheduled ? "…" : "on demand")}
          tone={schedule === "ENABLED" ? "green" : "default"}
        />
      </div>

      <div className="flex flex-wrap items-center gap-2 mt-4 pt-3 border-t border-border">
        {scheduled && (
          <>
            <button
              disabled={busy !== null}
              onClick={() =>
                act(
                  "schedule",
                  () => api.scheduleOne(node.service, schedule === "ENABLED" ? "disable" : "enable"),
                  schedule === "ENABLED" ? "Schedule disabled." : "Schedule enabled."
                )
              }
              className={`${btn} ${
                schedule === "ENABLED"
                  ? "bg-yellow/12 text-yellow border-yellow/30 hover:bg-yellow/20"
                  : "bg-green/12 text-green border-green/30 hover:bg-green/20"
              }`}
            >
              {busy === "schedule" ? (
                <Loader2 size={13} className="animate-spin" />
              ) : schedule === "ENABLED" ? (
                <Pause size={13} />
              ) : (
                <Play size={13} />
              )}
              {schedule === "ENABLED" ? "Disable schedule" : "Enable schedule"}
            </button>
            <button
              disabled={busy !== null}
              onClick={() => act("invoke", () => api.invokeService(node.service), `Invoked ${node.service}.`)}
              className={`${btn} bg-blue/12 text-blue border-blue/30 hover:bg-blue/20`}
            >
              {busy === "invoke" ? <Loader2 size={13} className="animate-spin" /> : <Zap size={13} />}
              Run now
            </button>
          </>
        )}
        <button
          disabled={busy !== null}
          onClick={() => {
            if (confirm(`Roll ${node.service} back to its previous published version?`)) {
              act("rollback", () => api.rollbackService(node.service), `${node.service} rolled back.`);
            }
          }}
          className={`${btn} border-border text-text-secondary hover:text-text-primary`}
        >
          {busy === "rollback" ? <Loader2 size={13} className="animate-spin" /> : <Undo2 size={13} />}
          Rollback version
        </button>
        <a
          href={`https://console.aws.amazon.com/cloudwatch/home?region=${REGION}#logsV2:log-groups/log-group/${encodeURIComponent(
            `/aws/lambda/hivemind-dev-${node.service}`
          )}`}
          target="_blank"
          rel="noreferrer"
          className={`${btn} border-border text-text-secondary hover:text-text-primary`}
        >
          <ExternalLink size={13} />
          Logs
        </a>
      </div>

      {msg && <div className="mt-3 text-[13px] text-text-secondary border-t border-border pt-2.5">{msg}</div>}
    </Panel>
  );
}

function KV({ label, value, tone }: { label: string; value: string; tone?: string }) {
  const cls =
    tone === "green" ? "text-green" : tone === "red" ? "text-red" : tone === "yellow" ? "text-yellow" : "text-text-primary";
  return (
    <div className="rounded-lg border border-border bg-bg-inset/40 px-3 py-2">
      <div className="text-[11px] uppercase tracking-wide text-text-tertiary font-semibold">{label}</div>
      <div className={`tnum text-[14px] font-semibold ${cls}`}>{value}</div>
    </div>
  );
}

// --- Multi-region control ----------------------------------------------------

function RegionsPanel() {
  const { data, error } = useLive(api.getRegions, 20000);
  const [region, setRegion] = useState("");
  const [busy, setBusy] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);

  const run = async (key: string, fn: () => Promise<unknown>, done: string) => {
    setBusy(key);
    setMsg(null);
    try {
      await fn();
      setMsg(done);
    } catch (e) {
      setMsg(`${(e as Error).message}`);
    } finally {
      setBusy(null);
    }
  };

  const regions = data?.regions ?? [];

  return (
    <Panel
      title="Multi-region"
      subtitle="CockroachDB database regions - survive a region failure with RPO = 0"
      actions={
        data ? (
          <Badge variant={data.multi_region ? "green" : "default"} dot>
            {data.multi_region
              ? `${regions.length} region${regions.length > 1 ? "s" : ""}${
                  data.survival_goal ? ` · ${data.survival_goal}` : ""
                }`
              : "single-region"}
          </Badge>
        ) : null
      }
    >
      {error && <div className="text-[13px] text-red mb-2">{error}</div>}

      <div className="flex flex-wrap items-center gap-2">
        <Globe2 size={15} className="text-text-tertiary" />
        {regions.length === 0 ? (
          <span className="text-[13px] text-text-tertiary">
            Database has no region configuration yet - add the primary region to begin.
          </span>
        ) : (
          regions.map((r) => (
            <span
              key={r.region}
              className={`flex items-center gap-1.5 px-2.5 py-1.5 rounded-md border text-[13px] tnum ${
                r.primary ? "border-blue/40 bg-blue/10 text-blue" : "border-border bg-bg-inset/50 text-text-secondary"
              }`}
            >
              {r.region}
              {r.primary && <span className="text-[10px] font-bold uppercase">primary</span>}
              {!r.primary && (
                <button
                  disabled={busy !== null}
                  onClick={() => run(`drop-${r.region}`, () => api.alterRegion("drop", r.region), `Dropped ${r.region}.`)}
                  className="text-text-tertiary hover:text-red disabled:opacity-40"
                  title={`Drop ${r.region}`}
                >
                  {busy === `drop-${r.region}` ? <Loader2 size={12} className="animate-spin" /> : <X size={12} />}
                </button>
              )}
            </span>
          ))
        )}
      </div>

      <div className="flex flex-wrap items-center gap-2 mt-3 pt-3 border-t border-border">
        <input
          value={region}
          onChange={(e) => setRegion(e.target.value)}
          placeholder="aws-ap-southeast-2"
          spellCheck={false}
          className="w-52 bg-bg-inset border border-border rounded-md px-2.5 py-2 text-[13px] tnum text-text-primary outline-none focus:border-blue"
        />
        <button
          disabled={busy !== null || !region.trim()}
          onClick={() =>
            run(
              "add",
              () =>
                regions.length === 0
                  ? api.alterRegion("set_primary", region.trim())
                  : api.alterRegion("add", region.trim()),
              regions.length === 0 ? `Primary region set: ${region.trim()}.` : `Added region ${region.trim()}.`
            )
          }
          className="flex items-center gap-1.5 px-3 py-2 rounded-md bg-blue/12 text-blue border border-blue/30 text-[13px] font-semibold hover:bg-blue/20 disabled:opacity-40 transition-colors"
        >
          {busy === "add" ? <Loader2 size={13} className="animate-spin" /> : <Plus size={13} />}
          {regions.length === 0 ? "Set primary region" : "Add region"}
        </button>
        {regions.length > 1 && (
          <button
            disabled={busy !== null}
            onClick={() => run("survive", () => api.alterRegion("survive_region"), "Survival goal: REGION failure.")}
            className="flex items-center gap-1.5 px-3 py-2 rounded-md bg-green/12 text-green border border-green/30 text-[13px] font-semibold hover:bg-green/20 disabled:opacity-40 transition-colors"
          >
            {busy === "survive" ? <Loader2 size={13} className="animate-spin" /> : <Check size={13} />}
            Survive region failure
          </button>
        )}
        <span className="text-[12px] text-text-tertiary ml-auto">
          Cluster must have the region provisioned first - see <span className="tnum">scripts/multi-region.sh</span>.
        </span>
      </div>

      {msg && <div className="mt-3 text-[13px] text-text-secondary border-t border-border pt-2.5">{msg}</div>}
    </Panel>
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
    <span aria-hidden className={`self-center px-1.5 text-[13px] ${done ? "text-green" : "text-text-tertiary"}`}>
      →
    </span>
  );
}
