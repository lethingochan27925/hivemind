"use client";

import { api } from "@/lib/api";
import { useLive } from "@/lib/use-live";
import { Panel } from "@/components/ui/panel";
import { Stat } from "@/components/ui/stat";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import { PASTEL } from "@/lib/palette";
import { MemoryAdmin } from "@/components/control/memory-admin";
import { BarList } from "@/components/charts/bar-list";
import { Database, Archive, Gauge, Target } from "lucide-react";

const prettyPattern = (s: string) =>
  s.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());

export default function MemoryPage() {
  const { data, error, lastUpdated } = useLive(api.getMemory, 8000);
  const stats = data?.stats;
  const impact = data?.impact;

  const withHit = impact?.avg_latency_with_hit_ms;
  const noHit = impact?.avg_latency_no_hit_ms;
  const speedup =
    withHit != null && noHit != null && withHit > 0
      ? Math.round(((noHit - withHit) / noHit) * 100)
      : null;

  return (
    <div>
      <PageHeader
        title="Fleet & Memory"
        description="Episodic memory in CockroachDB - what the fleet has learned and how it recalls it"
        lastUpdated={lastUpdated}
        error={error}
      />

      <div className="p-6 space-y-5 hm-enter">
        <MemoryAdmin />

        <div className="grid grid-cols-2 md:grid-cols-4 border border-border rounded-md divide-y md:divide-y-0 md:divide-x divide-border bg-bg-panel/40">
          <Stat
            label="Active cases"
            value={stats?.active_cases ?? "-"}
            color="blue"
            icon={<Database size={12} />}
            hint="consolidated, searchable"
          />
          <Stat
            label="Archived"
            value={stats?.archived_cases ?? "-"}
            color="default"
            icon={<Archive size={12} />}
            hint="decayed below threshold"
          />
          <Stat
            label="Avg salience"
            value={stats?.avg_salience != null ? stats.avg_salience.toFixed(2) : "-"}
            color="purple"
            icon={<Gauge size={12} />}
            hint="recall-weighted importance"
          />
          <Stat
            label="Verdict accuracy"
            value={impact?.verdict_accuracy_pct != null ? impact.verdict_accuracy_pct : "-"}
            unit="%"
            color="green"
            icon={<Target size={12} />}
            hint="vs ground-truth labels"
          />
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          <Panel title="Learned patterns" subtitle="episodic cases by fraud signature">
            {stats?.top_patterns && stats.top_patterns.length > 0 ? (
              <BarList
                rows={stats.top_patterns.map((p) => ({
                  label: p.pattern_type,
                  value: p.count,
                }))}
                color={PASTEL.purple}
                formatLabel={prettyPattern}
              />
            ) : (
              <EmptyState
                message="No patterns recorded yet"
                hint="Each investigation writes a case; similar cases (>0.92) consolidate into one pattern."
              />
            )}
          </Panel>

          <Panel
            title="Memory impact"
            subtitle="does recall make the fleet faster?"
          >
            {withHit != null && noHit != null ? (
              <div className="flex flex-col gap-5 py-1">
                <div className="grid grid-cols-2 gap-3">
                  <div className="rounded-md border border-border bg-bg-inset px-3 py-3">
                    <div className="text-[12px] uppercase tracking-wide text-text-tertiary">
                      With memory hit
                    </div>
                    <div className="tnum text-2xl font-semibold text-green mt-1">
                      {Math.round(withHit)}
                      <span className="text-sm text-text-tertiary ml-1">ms</span>
                    </div>
                  </div>
                  <div className="rounded-md border border-border bg-bg-inset px-3 py-3">
                    <div className="text-[12px] uppercase tracking-wide text-text-tertiary">
                      Cold (no hit)
                    </div>
                    <div className="tnum text-2xl font-semibold text-text-secondary mt-1">
                      {Math.round(noHit)}
                      <span className="text-sm text-text-tertiary ml-1">ms</span>
                    </div>
                  </div>
                </div>
                {speedup != null && (
                  <p className="text-[13px] text-text-secondary">
                    {speedup > 0 ? (
                      <>
                        Recalling a prior case resolves an investigation{" "}
                        <span className="text-green font-medium">{speedup}% faster</span> - memory
                        turns investigation cost into an accumulating asset.
                      </>
                    ) : (
                      <>Latency is comparable with and without recall on the current sample.</>
                    )}
                  </p>
                )}
              </div>
            ) : (
              <EmptyState
                message="Not enough data to compare"
                hint="Latency-with-hit vs cold-start is measured once the same pattern recurs."
              />
            )}
          </Panel>
        </div>

        <Panel
          title="Active agents"
          subtitle="workers seen in the last 30 minutes"
          actions={
            data?.active_agents?.length ? (
              <span className="tnum text-[13px] text-text-tertiary">
                {data.active_agents.length}
              </span>
            ) : null
          }
        >
          {data?.active_agents && data.active_agents.length > 0 ? (
            <div className="overflow-x-auto">
              <table className="w-full text-[14px]">
                <thead>
                  <tr className="text-left text-text-tertiary border-b border-border">
                    <th className="pb-2 font-normal">Agent</th>
                    <th className="pb-2 font-normal">Status</th>
                    <th className="pb-2 font-normal">Current task</th>
                    <th className="pb-2 font-normal text-right">Last activity</th>
                  </tr>
                </thead>
                <tbody>
                  {data.active_agents.map((a) => (
                    <tr key={a.agent_id} className="border-b border-border/40">
                      <td className="py-2 tnum text-text-secondary">{a.agent_id.slice(0, 18)}</td>
                      <td className="py-2">
                        <Badge variant={a.status === "active" ? "blue" : "default"} dot>
                          {a.status}
                        </Badge>
                      </td>
                      <td className="py-2 tnum text-text-tertiary">
                        {a.current_task ? a.current_task.slice(0, 8) : "-"}
                      </td>
                      <td className="py-2 tnum text-right text-text-tertiary">
                        {a.last_activity ? new Date(a.last_activity).toLocaleTimeString() : "-"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <EmptyState
              message="No agents active in the last 30 minutes"
              hint="Agents are ephemeral Lambdas - they appear here only while a stream is running."
            />
          )}
        </Panel>
      </div>
    </div>
  );
}
