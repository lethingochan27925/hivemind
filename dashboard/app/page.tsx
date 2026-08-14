"use client";

import { api } from "@/lib/api";
import { useLive } from "@/lib/use-live";
import { Panel } from "@/components/ui/panel";
import { Stat } from "@/components/ui/stat";
import { Meter } from "@/components/ui/meter";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import { VerdictDonut } from "@/components/charts/verdict-donut";
import { AreaTrend } from "@/components/charts/area-trend";
import { LearningCurve } from "@/components/charts/learning-curve";
import { SystemStatus } from "@/components/control/system-status";
import { FleetControl } from "@/components/control/fleet-control";
import { PolicyPanel } from "@/components/control/policy-panel";
import { ShieldAlert, ShieldCheck, Flag, ClipboardList, Cpu } from "lucide-react";
import { PASTEL } from "@/lib/palette";

const C = { red: PASTEL.red, green: PASTEL.green, yellow: PASTEL.yellow, blue: PASTEL.blue };

/**
 * Mission Control - the page a first-time visitor lands on.
 *
 * Order is deliberate and reads top to bottom as an answer to three questions:
 *   1. what happened?      KPI row
 *   2. is anything wrong?  status banner, with the one action that fixes it
 *   3. how well did it go? impact and verdict split, then the learning curves
 * Operating controls sit below that, and policy tuning - which almost no visitor
 * touches - sits last and folded. Previously the tuning panel was third from the
 * top, so the first thing anyone saw was an empty form.
 */
export default function OverviewPage() {
  const { data, error, loading, lastUpdated } = useLive(api.getOverview, 5000);

  const verdicts = data?.verdicts_today ?? [];
  const count = (v: string) => verdicts.find((x) => x.verdict === v)?.count ?? 0;
  const fraud = count("fraud");
  const legit = count("legit");
  const escalate = count("escalate");
  const autoResolved = fraud + legit;
  const handled = fraud + legit + escalate;
  const agents = new Set(data?.live_tasks?.map((t) => t.claimed_by) ?? []).size;
  const accuracy = data?.verdict_accuracy_pct;

  // First load only. A refresh must never blank out numbers that are already on
  // screen, or the whole page flickers every five seconds.
  const pending = loading && !data;

  const trend =
    data?.memory_hits_trend?.map((p) => ({
      hour: p.hour_bucket,
      hits: Math.round(p.avg_memory_hits * 100) / 100,
    })) ?? [];

  const curve =
    data?.learning_curve?.map((p) => ({
      hour: p.hour_bucket,
      latency: Math.round(p.avg_latency_ms),
      hits: Math.round(p.avg_memory_hits * 100) / 100,
    })) ?? [];

  const chartSkeleton = (h: string) => <div className={`${h} hm-skeleton rounded`} />;

  return (
    <div>
      <PageHeader
        title="Mission Control"
        description="Fraud-investigation fleet - live verdicts, memory, and throughput"
        lastUpdated={lastUpdated}
        error={error}
      />

      <div className="p-6 space-y-5 hm-enter">
        {/* What happened today */}
        <div className="grid grid-cols-2 md:grid-cols-5 border border-border rounded-md divide-y md:divide-y-0 md:divide-x divide-border bg-bg-panel/40">
          <Stat label="Fraud blocked" value={fraud} color="red" loading={pending} icon={<ShieldAlert size={12} />} />
          <Stat label="Escalated" value={escalate} color="yellow" loading={pending} icon={<Flag size={12} />} />
          <Stat label="Cleared" value={legit} color="green" loading={pending} icon={<ShieldCheck size={12} />} />
          <Stat
            label="Pending review"
            value={data?.pending_reviews ?? "-"}
            color={(data?.pending_reviews ?? 0) > 0 ? "yellow" : "default"}
            loading={pending}
            icon={<ClipboardList size={12} />}
          />
          <Stat label="Active agents" value={agents} color="blue" loading={pending} icon={<Cpu size={12} />} />
        </div>

        {/* Is anything wrong, and what do I press */}
        <SystemStatus apiError={error} hasVerdicts={handled > 0} />

        {/* How well did it go */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
          <Panel title="Real-world impact" className="lg:col-span-2">
            <div className="grid grid-cols-1 sm:grid-cols-[auto_1fr] gap-6 items-center">
              <div className="flex flex-col gap-1">
                <div className="flex items-baseline gap-2">
                  <span className="tnum text-4xl font-semibold text-green">
                    {accuracy != null ? `${accuracy}` : "-"}
                    <span className="text-lg">%</span>
                  </span>
                  <span className="text-[14px] text-text-tertiary">verdict accuracy</span>
                </div>
                <span className="text-[13px] text-text-tertiary max-w-[36ch]">
                  Measured against PaySim ground-truth labels, over every case the agent
                  auto-decided (fraud or cleared).
                </span>
              </div>
              <div className="flex flex-col gap-3 min-w-0">
                <Meter
                  label="Agent auto-resolved"
                  value={autoResolved}
                  max={handled}
                  tone="blue"
                  display={handled ? `${autoResolved}/${handled}` : "-"}
                />
                <Meter
                  label="Escalated to human review"
                  value={escalate}
                  max={handled}
                  tone="yellow"
                  display={handled ? `${escalate}/${handled}` : "-"}
                />
                <Meter
                  label="Fraud auto-blocked"
                  value={fraud}
                  max={handled}
                  tone="red"
                  display={`${fraud}`}
                />
              </div>
            </div>
          </Panel>

          <Panel title="Verdict split">
            {pending ? (
              chartSkeleton("h-[200px]")
            ) : handled > 0 ? (
              <VerdictDonut
                data={[
                  { name: "cleared", value: legit, color: C.green },
                  { name: "escalate", value: escalate, color: C.yellow },
                  { name: "fraud", value: fraud, color: C.red },
                ]}
              />
            ) : (
              <EmptyState message="No verdicts yet" hint="Start the fleet to populate this." />
            )}
          </Panel>
        </div>

        {/* Operate */}
        <FleetControl />

        {/* The payoff of agentic memory, over time */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          <Panel
            title="Fleet learning curve"
            subtitle="as shared memory grows: recall rises, reasoning latency falls"
          >
            {pending ? (
              chartSkeleton("h-[220px]")
            ) : curve.length > 1 ? (
              <LearningCurve data={curve} />
            ) : (
              <div className="h-[220px]">
                <EmptyState
                  message="Not enough history yet"
                  hint="Run the fleet for a while - each hour plots avg memory hits vs avg Bedrock latency, the measurable payoff of agentic memory."
                />
              </div>
            )}
          </Panel>

          <Panel
            title="Memory recall"
            subtitle="avg similar cases retrieved per investigation, last 24h"
          >
            {pending ? (
              chartSkeleton("h-[150px]")
            ) : trend.length > 0 ? (
              <AreaTrend data={trend} xKey="hour" yKey="hits" color={C.blue} unit=" hits" />
            ) : (
              <div className="h-[150px]">
                <EmptyState
                  message="No recall data in the last 24h"
                  hint="Each investigation records how many past cases the vector index returned."
                />
              </div>
            )}
          </Panel>
        </div>

        <Panel
          title="Live fleet"
          subtitle="agents currently investigating"
          actions={
            data?.live_tasks?.length ? (
              <span className="tnum text-[13px] text-text-tertiary">
                {data.live_tasks.length} active
              </span>
            ) : null
          }
        >
          {pending ? (
            <div className="space-y-2">
              {Array.from({ length: 4 }).map((_, i) => (
                <div key={i} className="h-6 hm-skeleton" />
              ))}
            </div>
          ) : data?.live_tasks && data.live_tasks.length > 0 ? (
            <div className="overflow-x-auto">
              <table className="w-full text-[14px]">
                <thead>
                  <tr className="text-left text-text-tertiary border-b border-border">
                    <th className="pb-2 font-normal">Task</th>
                    <th className="pb-2 font-normal">Agent</th>
                    <th className="pb-2 font-normal text-right">Claimed</th>
                  </tr>
                </thead>
                <tbody>
                  {data.live_tasks.slice(0, 12).map((t) => (
                    <tr key={t.task_id} className="border-b border-border/40">
                      <td className="py-2 tnum text-text-secondary">{t.task_id.slice(0, 8)}</td>
                      <td className="py-2 tnum text-text-secondary">{t.claimed_by.slice(0, 16)}</td>
                      <td className="py-2 tnum text-right text-text-tertiary">
                        {new Date(t.claimed_at).toLocaleTimeString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <EmptyState message="No agents currently investigating" hint="The queue is drained - start a stream to see the fleet claim tasks concurrently." />
          )}
        </Panel>

        {/* Advanced: tuning the thresholds the fleet decides by */}
        <PolicyPanel />
      </div>
    </div>
  );
}
