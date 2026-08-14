"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api, TrainingSnapshot, TrainingLogEntry, TrainingRun } from "@/lib/api";
import { Panel } from "@/components/ui/panel";
import { Stat } from "@/components/ui/stat";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import { LearningCurve } from "@/components/charts/learning-curve";
import { PASTEL } from "@/lib/palette";
import {
  Play,
  Square,
  Loader2,
  Brain,
  Database,
  Cpu,
  ShieldAlert,
  ShieldCheck,
  Flag,
  RotateCcw,
  Download,
  GraduationCap,
  Sparkles,
  Save,
  History,
} from "lucide-react";
import {
  ResponsiveContainer,
  ComposedChart,
  Line,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
} from "recharts";

/**
 * Training Lab - run the fleet over data batch by batch and watch what it learns.
 *
 * Honest framing, stated in the UI as well: this does NOT fine-tune model weights
 * (Claude Haiku on Bedrock is not trainable here). What forms is EPISODIC MEMORY -
 * each closed case is summarised, embedded, and consolidated with similar ones.
 * This page measures that process the way an experiment tracker would: a run is a
 * sequence of batches, and every batch is a measured step.
 *
 * The browser orchestrates; the server stays stateless. Each batch: feed cases ->
 * wait for the queue to drain -> take a cumulative snapshot -> diff it against the
 * previous one. Every number therefore traces back to audit_log.
 */

interface BatchResult {
  n: number;
  cases: number;
  auto: number;
  escalated: number;
  autoPct: number;
  accuracyPct: number | null;
  avgRecall: number;
  memories: number;
  absorbed: number;
  costUsd: number;
  fallbacks: number;
  seconds: number;
}

const IN_COST = 0.00025;
const OUT_COST = 0.00125;

const diff = (a: TrainingSnapshot, b: TrainingSnapshot, n: number, seconds: number): BatchResult => {
  const fraud = b.verdict_fraud - a.verdict_fraud;
  const legit = b.verdict_legit - a.verdict_legit;
  const esc = b.verdict_escalate - a.verdict_escalate;
  const auto = fraud + legit;
  const closed = auto + esc;
  const graded = b.graded_verdicts - a.graded_verdicts;
  const correct = b.correct_verdicts - a.correct_verdicts;
  const recallEvents = b.recall_events - a.recall_events;
  const recallHits = b.recall_hits_sum - a.recall_hits_sum;
  return {
    n,
    cases: closed,
    auto,
    escalated: esc,
    autoPct: closed > 0 ? (auto / closed) * 100 : 0,
    accuracyPct: graded > 0 ? (correct / graded) * 100 : null,
    avgRecall: recallEvents > 0 ? recallHits / recallEvents : 0,
    memories: b.memories_active,
    absorbed: b.raw_cases_absorbed,
    costUsd:
      ((b.tokens_in - a.tokens_in) / 1000) * IN_COST + ((b.tokens_out - a.tokens_out) / 1000) * OUT_COST,
    fallbacks: b.fallback_verdicts - a.fallback_verdicts,
    seconds,
  };
};

export default function TrainingPage() {
  const [batchSize, setBatchSize] = useState(40);
  const [batchCount, setBatchCount] = useState(4);
  const [coldStart, setColdStart] = useState(true);

  const [ingestCount, setIngestCount] = useState(60);
  const [fraudRate, setFraudRate] = useState(0.1);
  const [workers, setWorkers] = useState(20);
  const [ingesting, setIngesting] = useState(false);
  const [ingestMsg, setIngestMsg] = useState<string | null>(null);
  const [runs, setRuns] = useState<TrainingRun[]>([]);
  const [saveMsg, setSaveMsg] = useState<string | null>(null);

  const [running, setRunning] = useState(false);
  const [phase, setPhase] = useState<string>("");
  const [batches, setBatches] = useState<BatchResult[]>([]);
  const [snapshot, setSnapshot] = useState<TrainingSnapshot | null>(null);
  const [log, setLog] = useState<TrainingLogEntry[]>([]);
  const [error, setError] = useState<string | null>(null);
  const stopRef = useRef(false);

  // Live snapshot + activity feed, whether or not a run is in progress.
  useEffect(() => {
    let alive = true;
    const tick = async () => {
      if (document.hidden) return;
      try {
        const [m, l] = await Promise.all([api.trainingMetrics(), api.trainingLog(60)]);
        if (!alive) return;
        setSnapshot(m);
        setLog(l ?? []);
      } catch {
        /* transient */
      }
    };
    tick();
    const id = setInterval(tick, 4000);
    return () => {
      alive = false;
      clearInterval(id);
    };
  }, []);

  const loadRuns = useCallback(() => {
    api.listTrainingRuns().then((r) => setRuns(r ?? [])).catch(() => {});
  }, []);
  useEffect(() => {
    loadRuns();
  }, [loadRuns]);

  // Nap du lieu MOI (khac feed: feed chi tra case cu ve hang doi).
  const ingest = async () => {
    setIngesting(true);
    setIngestMsg(null);
    try {
      const r = await api.ingest(ingestCount, fraudRate);
      setIngestMsg(
        `Ingested ${r.inserted} new transactions (${r.fraud} fraud / ${r.legit} legit) in ${(r.took_ms / 1000).toFixed(1)}s — ${r.queued_now} now queued.`
      );
      await api.runDispatch().catch(() => {});
    } catch (e) {
      setIngestMsg((e as Error).message);
    } finally {
      setIngesting(false);
    }
  };

  const waitForDrain = useCallback(async (timeoutMs: number): Promise<TrainingSnapshot> => {
    const deadline = Date.now() + timeoutMs;
    let last = await api.trainingMetrics();
    while (Date.now() < deadline && !stopRef.current) {
      await new Promise((r) => setTimeout(r, 3000));
      last = await api.trainingMetrics();
      setSnapshot(last);
      setPhase(`processing — ${last.pending} pending, ${last.investigating} in flight`);
      if (last.pending === 0 && last.investigating === 0) break;
      // Requeued work does not invoke the fleet by itself; keep it fanned out.
      api.runDispatch().catch(() => {});
    }
    return last;
  }, []);

  const start = async () => {
    setRunning(true);
    setError(null);
    setBatches([]);
    stopRef.current = false;
    try {
      // Fan-out cua dispatcher quyet dinh toc do xu ly - dat truoc khi chay.
      try {
        const pol = await api.getPolicy();
        if (pol.dispatch_batch !== workers) await api.setPolicy({ ...pol, dispatch_batch: workers });
      } catch {
        /* policy khong bat buoc */
      }

      if (coldStart) {
        setPhase("archiving every memory — starting from a blank slate");
        await api.memoryJob("archive_all");
      }
      setPhase("baseline snapshot");
      let prev = await api.trainingMetrics();
      setSnapshot(prev);

      for (let i = 1; i <= batchCount && !stopRef.current; i++) {
        const t0 = Date.now();
        setPhase(`batch ${i}/${batchCount} — feeding ${batchSize} cases`);
        await api.feedStream(batchSize);
        await api.runDispatch().catch(() => {});
        const after = await waitForDrain(10 * 60 * 1000);
        const seconds = Math.round((Date.now() - t0) / 1000);
        setBatches((b) => [...b, diff(prev, after, i, seconds)]);
        prev = after;
      }
      setPhase(stopRef.current ? "stopped" : "run complete");
      setBatches((done) => {
        if (done.length > 0) void saveRun(done);
        return done;
      });
    } catch (e) {
      setError((e as Error).message);
      setPhase("failed");
    } finally {
      setRunning(false);
    }
  };

  const saveRun = async (result: BatchResult[]) => {
    try {
      const summary = {
        batches: result.length,
        cases: result.reduce((s, b) => s + b.cases, 0),
        auto_resolved: result.reduce((s, b) => s + b.auto, 0),
        escalated: result.reduce((s, b) => s + b.escalated, 0),
        cost_usd: result.reduce((s, b) => s + b.costUsd, 0),
        final_memories: result[result.length - 1]?.memories ?? 0,
        avg_recall_first: result[0]?.avgRecall ?? 0,
        avg_recall_last: result[result.length - 1]?.avgRecall ?? 0,
      };
      const r = await api.saveTrainingRun(
        `${result.length}×${batchSize}${coldStart ? " cold" : ""}`,
        { batch_size: batchSize, batches: batchCount, cold_start: coldStart, workers },
        result,
        summary
      );
      setSaveMsg(`Run saved (${r.id.slice(0, 8)}).`);
      loadRuns();
    } catch (e) {
      setSaveMsg(`Could not save the run: ${(e as Error).message}`);
    }
  };

  const stop = () => {
    stopRef.current = true;
    setPhase("stopping after this batch…");
  };

  const exportRun = () => {
    const payload = {
      exported_at: new Date().toISOString(),
      config: { batch_size: batchSize, batches: batchCount, cold_start: coldStart },
      batches,
      final_snapshot: snapshot,
    };
    const url = URL.createObjectURL(new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = `hivemind-training-run.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  const chart = batches.map((b) => ({
    batch: `#${b.n}`,
    autoPct: Math.round(b.autoPct * 10) / 10,
    recall: Math.round(b.avgRecall * 100) / 100,
    memories: b.memories,
    absorbed: b.absorbed,
    cases: b.cases,
    escalated: b.escalated,
  }));

  const curve = batches.map((b) => ({
    hour: `#${b.n}`,
    latency: Math.round(b.costUsd * 1_000_000) / 1000, // cost per batch, in millicents
    hits: Math.round(b.avgRecall * 100) / 100,
  }));

  const totalCases = batches.reduce((s, b) => s + b.cases, 0);
  const totalCost = batches.reduce((s, b) => s + b.costUsd, 0);

  return (
    <div>
      <PageHeader
        title="Training Lab"
        description="Feed data batch by batch and watch the fleet's memory form — measured, not asserted"
        error={error}
        actions={
          <Badge variant={running ? "yellow" : batches.length > 0 ? "green" : "default"} dot>
            {running ? phase || "running" : batches.length > 0 ? `${batches.length} batches` : "idle"}
          </Badge>
        }
      />

      <div className="p-6 space-y-5 hm-enter">
        {/* What this actually trains */}
        <div className="flex items-start gap-2.5 rounded-lg border border-border bg-bg-inset/40 px-4 py-3 text-[13px] text-text-secondary">
          <GraduationCap size={16} className="text-blue shrink-0 mt-0.5" />
          <span>
            This does <strong className="text-text-primary">not</strong> fine-tune model weights — Claude
            Haiku on Bedrock is not trainable here. What forms is{" "}
            <strong className="text-text-primary">episodic memory</strong>: every closed case is
            summarised, embedded, and consolidated with similar ones. Each batch below is a measured
            step, and every number traces back to the append-only audit log.
          </span>
        </div>

        {/* Live state */}
        <div className="grid grid-cols-2 md:grid-cols-5 border border-border rounded-md divide-y md:divide-y-0 md:divide-x divide-border bg-bg-panel/40">
          <Stat
            label="Active memories"
            value={snapshot?.memories_active ?? "—"}
            color="purple"
            icon={<Brain size={12} />}
          />
          <Stat
            label="Raw cases absorbed"
            value={snapshot?.raw_cases_absorbed?.toLocaleString() ?? "—"}
            color="blue"
            icon={<Database size={12} />}
          />
          <Stat
            label="In flight"
            value={snapshot ? snapshot.pending + snapshot.investigating : "—"}
            color={snapshot && snapshot.pending + snapshot.investigating > 0 ? "yellow" : "default"}
            icon={<Cpu size={12} />}
          />
          <Stat
            label="Verdicts"
            value={
              snapshot
                ? (snapshot.verdict_fraud + snapshot.verdict_legit + snapshot.verdict_escalate).toLocaleString()
                : "—"
            }
            color="green"
            icon={<ShieldCheck size={12} />}
          />
          <Stat
            label="Accuracy"
            value={
              snapshot && snapshot.graded_verdicts > 0
                ? `${((snapshot.correct_verdicts / snapshot.graded_verdicts) * 100).toFixed(1)}%`
                : "—"
            }
            color="green"
            icon={<ShieldAlert size={12} />}
          />
        </div>

        {/* Nạp dữ liệu mới - làm giàu kho case, qua đó làm giàu bộ nhớ */}
        <Panel
          title="Ingest training data"
          subtitle="generate new labelled transactions and queue them for the fleet"
          resizable={false}
          actions={<Sparkles size={15} className="text-text-tertiary" />}
        >
          <div className="flex flex-wrap items-end gap-4">
            <label className="flex flex-col gap-1 text-[13px] text-text-secondary">
              Transactions
              <input
                type="number"
                min={1}
                max={300}
                value={ingestCount}
                disabled={ingesting}
                onChange={(e) => setIngestCount(Math.max(1, Math.min(300, Number(e.target.value) || 0)))}
                className="w-28 bg-bg-inset border border-border rounded-md px-2.5 py-2 tnum text-[14px] text-text-primary outline-none focus:border-blue disabled:opacity-50"
              />
            </label>
            <label className="flex flex-col gap-1 text-[13px] text-text-secondary">
              Fraud share
              <input
                type="number"
                min={0}
                max={1}
                step={0.05}
                value={fraudRate}
                disabled={ingesting}
                onChange={(e) => setFraudRate(Math.max(0, Math.min(1, Number(e.target.value) || 0)))}
                className="w-24 bg-bg-inset border border-border rounded-md px-2.5 py-2 tnum text-[14px] text-text-primary outline-none focus:border-blue disabled:opacity-50"
              />
            </label>
            <span className="text-[12px] text-text-tertiary max-w-md pb-2">
              New cases carry a ground-truth label and land straight in the queue. They are scored
              synthetically inside the investigate band — this feeds the agent and its memory, it does
              not exercise the XGBoost scorer.
            </span>
            <button
              onClick={ingest}
              disabled={ingesting}
              className="flex items-center gap-1.5 px-4 py-2 rounded-md bg-purple/12 text-purple border border-purple/30 text-[14px] font-semibold hover:bg-purple/20 disabled:opacity-40 transition-colors ml-auto"
            >
              {ingesting ? <Loader2 size={14} className="animate-spin" /> : <Sparkles size={14} />}
              Generate &amp; ingest
            </button>
          </div>
          {ingestMsg && (
            <div className="mt-3 pt-3 border-t border-border text-[13px] text-text-secondary">{ingestMsg}</div>
          )}
        </Panel>

        {/* Đường đi của một case qua fleet, đếm từ audit trail đang chạy */}
        <Panel title="Agent pipeline" subtitle="what the fleet is doing right now, stage by stage" resizable={false}>
          <PipelineFunnel log={log} />
        </Panel>

        {/* Run configuration */}
        <Panel title="Training run" subtitle="each batch: feed cases → drain the queue → measure" resizable={false}>
          <div className="flex flex-wrap items-end gap-4">
            <label className="flex flex-col gap-1 text-[13px] text-text-secondary">
              Cases per batch
              <input
                type="number"
                min={5}
                max={500}
                value={batchSize}
                disabled={running}
                onChange={(e) => setBatchSize(Math.max(5, Math.min(500, Number(e.target.value) || 0)))}
                className="w-28 bg-bg-inset border border-border rounded-md px-2.5 py-2 tnum text-[14px] text-text-primary outline-none focus:border-blue disabled:opacity-50"
              />
            </label>
            <label className="flex flex-col gap-1 text-[13px] text-text-secondary">
              Batches
              <input
                type="number"
                min={1}
                max={20}
                value={batchCount}
                disabled={running}
                onChange={(e) => setBatchCount(Math.max(1, Math.min(20, Number(e.target.value) || 0)))}
                className="w-24 bg-bg-inset border border-border rounded-md px-2.5 py-2 tnum text-[14px] text-text-primary outline-none focus:border-blue disabled:opacity-50"
              />
            </label>
            <label className="flex flex-col gap-1 text-[13px] text-text-secondary">
              Parallel workers
              <input
                type="number"
                min={1}
                max={50}
                value={workers}
                disabled={running}
                onChange={(e) => setWorkers(Math.max(1, Math.min(50, Number(e.target.value) || 0)))}
                title="Written to the agent policy as dispatch_batch — more workers, faster batches"
                className="w-28 bg-bg-inset border border-border rounded-md px-2.5 py-2 tnum text-[14px] text-text-primary outline-none focus:border-blue disabled:opacity-50"
              />
            </label>
            <label className="flex items-center gap-2 text-[13px] text-text-secondary pb-2 cursor-pointer">
              <input
                type="checkbox"
                checked={coldStart}
                disabled={running}
                onChange={(e) => setColdStart(e.target.checked)}
                className="accent-[#7d9bf0]"
              />
              Start from a blank memory (archive everything first)
            </label>

            <div className="flex items-center gap-2 ml-auto pb-1">
              {batches.length > 0 && !running && (
                <button
                  onClick={exportRun}
                  className="flex items-center gap-1.5 px-3 py-2 rounded-md border border-border text-[13px] text-text-secondary hover:text-text-primary transition-colors"
                >
                  <Download size={14} />
                  Export run
                </button>
              )}
              {running ? (
                <button
                  onClick={stop}
                  className="flex items-center gap-1.5 px-4 py-2 rounded-md bg-red/12 text-red border border-red/30 text-[14px] font-semibold hover:bg-red/20 transition-colors"
                >
                  <Square size={14} />
                  Stop
                </button>
              ) : (
                <button
                  onClick={start}
                  className="flex items-center gap-1.5 px-4 py-2 rounded-md bg-blue/12 text-blue border border-blue/30 text-[14px] font-semibold hover:bg-blue/20 transition-colors"
                >
                  <Play size={14} />
                  Start training run
                </button>
              )}
            </div>
          </div>

          {(running || phase) && (
            <div className="mt-3 pt-3 border-t border-border flex items-center gap-2 text-[13px] text-text-secondary">
              {running && <Loader2 size={14} className="animate-spin text-blue" />}
              {phase}
              {batches.length > 0 && (
                <span className="ml-auto tnum text-text-tertiary">
                  {totalCases} cases · ${totalCost.toFixed(5)}
                </span>
              )}
            </div>
          )}
        </Panel>

        {/* Learning curves */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          <Panel title="Memory formation" subtitle="active memories and raw cases absorbed, per batch">
            {chart.length > 0 ? (
              <div className="h-[240px]">
                <ResponsiveContainer width="100%" height="100%">
                  <ComposedChart data={chart} margin={{ top: 8, right: 8, bottom: 0, left: -10 }}>
                    <CartesianGrid stroke="rgba(128,134,160,0.18)" strokeDasharray="3 3" vertical={false} />
                    <XAxis dataKey="batch" tick={{ fontSize: 11, fill: "#8d92aa" }} tickLine={false} />
                    <YAxis tick={{ fontSize: 11, fill: "#8d92aa" }} tickLine={false} axisLine={false} />
                    <Tooltip
                      contentStyle={{
                        background: "var(--color-bg-panel)",
                        border: "1px solid var(--color-border-strong)",
                        borderRadius: 8,
                        fontSize: 13,
                      }}
                    />
                    <Legend wrapperStyle={{ fontSize: 12 }} />
                    <Bar dataKey="memories" name="active memories" fill={PASTEL.purple} radius={[3, 3, 0, 0]} />
                    <Line
                      type="monotone"
                      dataKey="recall"
                      name="avg recalled / case"
                      stroke={PASTEL.blue}
                      strokeWidth={2}
                      dot={{ r: 3 }}
                    />
                  </ComposedChart>
                </ResponsiveContainer>
              </div>
            ) : (
              <div className="h-[240px]">
                <EmptyState
                  message="No run yet"
                  hint="Start a run: the first batch fills an empty memory, later batches show what recall does as it grows."
                />
              </div>
            )}
          </Panel>

          <Panel title="Decision mix" subtitle="auto-resolved vs escalated, per batch">
            {chart.length > 0 ? (
              <div className="h-[240px]">
                <ResponsiveContainer width="100%" height="100%">
                  <ComposedChart data={chart} margin={{ top: 8, right: 8, bottom: 0, left: -10 }}>
                    <CartesianGrid stroke="rgba(128,134,160,0.18)" strokeDasharray="3 3" vertical={false} />
                    <XAxis dataKey="batch" tick={{ fontSize: 11, fill: "#8d92aa" }} tickLine={false} />
                    <YAxis tick={{ fontSize: 11, fill: "#8d92aa" }} tickLine={false} axisLine={false} />
                    <Tooltip
                      contentStyle={{
                        background: "var(--color-bg-panel)",
                        border: "1px solid var(--color-border-strong)",
                        borderRadius: 8,
                        fontSize: 13,
                      }}
                    />
                    <Legend wrapperStyle={{ fontSize: 12 }} />
                    <Bar dataKey="cases" name="closed" fill={PASTEL.teal} radius={[3, 3, 0, 0]} />
                    <Bar dataKey="escalated" name="escalated" fill={PASTEL.yellow} radius={[3, 3, 0, 0]} />
                    <Line
                      type="monotone"
                      dataKey="autoPct"
                      name="auto-resolve %"
                      stroke={PASTEL.green}
                      strokeWidth={2}
                      dot={{ r: 3 }}
                    />
                  </ComposedChart>
                </ResponsiveContainer>
              </div>
            ) : (
              <div className="h-[240px]">
                <EmptyState message="No run yet" hint="Each batch reports how many cases the fleet closed on its own." />
              </div>
            )}
          </Panel>
        </div>

        {/* Per-batch table */}
        <Panel title="Batch results" subtitle="every row is a measured step of the run" bodyClassName="p-0">
          {batches.length === 0 ? (
            <EmptyState
              message="No batches recorded"
              hint="A run writes one row per batch: cases closed, auto-resolve rate, accuracy, memory growth and cost."
            />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-[13px]">
                <thead>
                  <tr className="text-left text-text-tertiary border-b border-border bg-bg-inset/40">
                    <th className="py-2.5 px-4 font-normal">Batch</th>
                    <th className="py-2.5 px-3 font-normal text-right">Closed</th>
                    <th className="py-2.5 px-3 font-normal text-right">Auto</th>
                    <th className="py-2.5 px-3 font-normal text-right">Escalated</th>
                    <th className="py-2.5 px-3 font-normal text-right">Auto-resolve</th>
                    <th className="py-2.5 px-3 font-normal text-right">Accuracy</th>
                    <th className="py-2.5 px-3 font-normal text-right">Avg recall</th>
                    <th className="py-2.5 px-3 font-normal text-right">Memories</th>
                    <th className="py-2.5 px-3 font-normal text-right">Fallbacks</th>
                    <th className="py-2.5 px-3 font-normal text-right">Cost</th>
                    <th className="py-2.5 px-4 font-normal text-right">Time</th>
                  </tr>
                </thead>
                <tbody>
                  {batches.map((b) => (
                    <tr key={b.n} className="border-b border-border/40">
                      <td className="py-2 px-4 text-text-primary font-semibold">#{b.n}</td>
                      <td className="py-2 px-3 tnum text-right text-text-secondary">{b.cases}</td>
                      <td className="py-2 px-3 tnum text-right text-green">{b.auto}</td>
                      <td className="py-2 px-3 tnum text-right text-yellow">{b.escalated}</td>
                      <td className="py-2 px-3 tnum text-right text-text-primary font-semibold">
                        {b.autoPct.toFixed(1)}%
                      </td>
                      <td className="py-2 px-3 tnum text-right text-text-secondary">
                        {b.accuracyPct == null ? "—" : `${b.accuracyPct.toFixed(1)}%`}
                      </td>
                      <td className="py-2 px-3 tnum text-right text-blue">{b.avgRecall.toFixed(2)}</td>
                      <td className="py-2 px-3 tnum text-right text-purple">{b.memories}</td>
                      <td className="py-2 px-3 tnum text-right text-text-tertiary">{b.fallbacks}</td>
                      <td className="py-2 px-3 tnum text-right text-text-secondary">${b.costUsd.toFixed(5)}</td>
                      <td className="py-2 px-4 tnum text-right text-text-tertiary">{b.seconds}s</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>

        {/* Phiên đã lưu */}
        <Panel
          title="Saved runs"
          subtitle="every completed run is stored in CockroachDB for comparison"
          bodyClassName="p-0"
          actions={<History size={15} className="text-text-tertiary" />}
        >
          {saveMsg && (
            <div className="px-4 py-2 border-b border-border text-[13px] text-text-secondary flex items-center gap-2">
              <Save size={13} />
              {saveMsg}
            </div>
          )}
          {runs.length === 0 ? (
            <EmptyState message="No saved runs yet" hint="A run is saved automatically when it finishes." />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-[13px]">
                <thead>
                  <tr className="text-left text-text-tertiary border-b border-border bg-bg-inset/40">
                    <th className="py-2.5 px-4 font-normal">When</th>
                    <th className="py-2.5 px-3 font-normal">Run</th>
                    <th className="py-2.5 px-3 font-normal text-right">Cases</th>
                    <th className="py-2.5 px-3 font-normal text-right">Auto</th>
                    <th className="py-2.5 px-3 font-normal text-right">Memories</th>
                    <th className="py-2.5 px-3 font-normal text-right">Recall first → last</th>
                    <th className="py-2.5 px-4 font-normal text-right">Cost</th>
                  </tr>
                </thead>
                <tbody>
                  {runs.map((r) => {
                    const sum = (r.summary ?? {}) as Record<string, number>;
                    return (
                      <tr key={r.id} className="border-b border-border/40">
                        <td className="py-2 px-4 tnum text-text-tertiary">
                          {new Date(r.created_at).toLocaleString()}
                        </td>
                        <td className="py-2 px-3 text-text-primary font-medium">{r.label || "—"}</td>
                        <td className="py-2 px-3 tnum text-right text-text-secondary">{sum.cases ?? 0}</td>
                        <td className="py-2 px-3 tnum text-right text-green">{sum.auto_resolved ?? 0}</td>
                        <td className="py-2 px-3 tnum text-right text-purple">{sum.final_memories ?? 0}</td>
                        <td className="py-2 px-3 tnum text-right text-blue">
                          {(sum.avg_recall_first ?? 0).toFixed(2)} → {(sum.avg_recall_last ?? 0).toFixed(2)}
                        </td>
                        <td className="py-2 px-4 tnum text-right text-text-secondary">
                          ${(sum.cost_usd ?? 0).toFixed(5)}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </Panel>

        {/* Live activity */}
        <Panel
          title="Live agent activity"
          subtitle="the append-only audit trail, newest first"
          bodyClassName="p-0"
          actions={
            <span className="tnum text-[12px] text-text-tertiary">{log.length} events</span>
          }
        >
          {log.length === 0 ? (
            <EmptyState message="No recent activity" hint="Start a run, or feed cases from Mission Control." />
          ) : (
            <ol className="max-h-[46vh] overflow-y-auto divide-y divide-border/40">
              {log.map((e, i) => (
                <li key={i} className="flex items-start gap-3 px-4 py-2 text-[13px]">
                  <span className="tnum text-[11px] text-text-tertiary shrink-0 w-16">
                    {new Date(e.at).toLocaleTimeString()}
                  </span>
                  <span className={`shrink-0 w-40 font-semibold ${actionTone(e.action)}`}>
                    {e.action.replace(/_/g, " ")}
                  </span>
                  <span className="min-w-0 flex-1 text-text-secondary truncate">
                    {e.reasoning ?? <span className="text-text-tertiary">—</span>}
                  </span>
                  <span className="shrink-0 flex items-center gap-2 tnum text-[11px] text-text-tertiary">
                    {e.memory_hits != null && e.action === "memory_recall" && (
                      <span className="text-blue">{e.memory_hits} recalled</span>
                    )}
                    {e.latency_ms != null && <span>{e.latency_ms} ms</span>}
                    {e.action === "bedrock_reasoning" && !e.bedrock_model && (
                      <span className="text-yellow">fallback</span>
                    )}
                  </span>
                </li>
              ))}
            </ol>
          )}
        </Panel>

        {/* Cost per batch, reusing the dual-axis curve */}
        {curve.length > 1 && (
          <Panel title="Cost per batch" subtitle="millicents spent vs memories recalled">
            <LearningCurve data={curve} />
          </Panel>
        )}
      </div>
    </div>
  );
}

/**
 * PipelineFunnel - the path a case takes through the fleet, counted from the live
 * audit trail: claim → recall memory → customer context → reason → verdict. It
 * makes the agent's process visible instead of leaving it a black box.
 */
function PipelineFunnel({ log }: { log: TrainingLogEntry[] }) {
  const stages = [
    {
      key: "claim",
      label: "Claimed",
      desc: "SKIP LOCKED",
      tone: PASTEL.gray,
      match: (a: string) => a === "task_claimed" || a === "task_resumed",
    },
    {
      key: "recall",
      label: "Memory recall",
      desc: "vector top-k",
      tone: PASTEL.blue,
      match: (a: string) => a === "memory_recall",
    },
    {
      key: "mcp",
      label: "Customer context",
      desc: "MCP read-only",
      tone: PASTEL.teal,
      match: (a: string) => a === "mcp_query",
    },
    {
      key: "reason",
      label: "Reasoning",
      desc: "Claude Haiku",
      tone: PASTEL.purple,
      match: (a: string) => a === "bedrock_reasoning",
    },
    {
      key: "verdict",
      label: "Verdict",
      desc: "fraud / legit / escalate",
      tone: PASTEL.green,
      match: (a: string) => a.startsWith("verdict_") || a === "auto_approve" || a === "auto_block",
    },
  ];

  const counts = stages.map((st) => log.filter((e) => st.match(e.action)).length);
  const max = Math.max(1, ...counts);

  const recalls = log.filter((e) => e.action === "memory_recall");
  const avgHits =
    recalls.length > 0 ? recalls.reduce((s, e) => s + (e.memory_hits ?? 0), 0) / recalls.length : 0;
  const reasons = log.filter((e) => e.action === "bedrock_reasoning" && e.latency_ms != null);
  const avgLatency =
    reasons.length > 0 ? reasons.reduce((s, e) => s + (e.latency_ms ?? 0), 0) / reasons.length : 0;

  return (
    <div>
      <div className="flex flex-col gap-2">
        {stages.map((st, i) => (
          <div key={st.key} className="flex items-center gap-3">
            <span className="w-36 shrink-0 text-[13px]">
              <span className="block font-semibold text-text-primary">{st.label}</span>
              <span className="block text-[11px] text-text-tertiary">{st.desc}</span>
            </span>
            <div className="flex-1 h-6 rounded bg-bg-inset overflow-hidden">
              <div
                className="h-full rounded transition-[width] duration-500 flex items-center justify-end pr-2"
                style={{
                  width: `${Math.max(4, (counts[i] / max) * 100)}%`,
                  background: st.tone,
                  opacity: 0.85,
                }}
              >
                {counts[i] > 0 && (
                  <span className="tnum text-[11px] font-semibold text-bg-primary">{counts[i]}</span>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>
      <div className="mt-3 pt-3 border-t border-border flex flex-wrap gap-x-6 gap-y-1 text-[12px] text-text-tertiary">
        <span>
          last <span className="tnum text-text-secondary">{log.length}</span> audited events
        </span>
        <span>
          avg recalled per case: <span className="tnum text-blue">{avgHits.toFixed(2)}</span>
        </span>
        <span>
          avg reasoning: <span className="tnum text-purple">{Math.round(avgLatency)} ms</span>
        </span>
      </div>
    </div>
  );
}

function actionTone(action: string): string {
  if (action.startsWith("verdict_fraud") || action === "auto_block" || action === "task_failed") return "text-red";
  if (action.startsWith("verdict_legit") || action === "auto_approve") return "text-green";
  if (action.startsWith("verdict_escalate")) return "text-yellow";
  if (action === "memory_recall") return "text-blue";
  if (action === "bedrock_reasoning") return "text-purple";
  if (action === "task_requeued" || action === "task_resumed") return "text-orange";
  if (action === "human_reviewed") return "text-teal";
  return "text-text-secondary";
}
