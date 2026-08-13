"use client";

import { api } from "@/lib/api";
import { useLive } from "@/lib/use-live";
import { Panel } from "@/components/ui/panel";
import { Stat } from "@/components/ui/stat";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import { PASTEL } from "@/lib/palette";
import { BudgetPanel } from "@/components/control/budget-panel";
import { CloudCostPanel } from "@/components/control/cloud-cost-panel";
import { BarList } from "@/components/charts/bar-list";
import { Coins, DollarSign } from "lucide-react";

export default function CostPage() {
  const { data, error, lastUpdated } = useLive(api.getCost, 10000);
  const agents = data?.by_agent ?? [];

  return (
    <div>
      <PageHeader
        title="Cost"
        description="Bedrock token usage and estimated spend, per agent - the fleet runs on cents"
        lastUpdated={lastUpdated}
        error={error}
      />

      <div className="p-6 space-y-5 hm-enter">
        <BudgetPanel />
        <CloudCostPanel bedrockToday={data?.estimated_cost_usd_today} />

        <div className="grid grid-cols-2 border border-border rounded-md divide-x divide-border bg-bg-panel/40">
          <Stat
            label="Tokens today"
            value={data?.total_tokens_today != null ? data.total_tokens_today.toLocaleString() : "-"}
            color="blue"
            icon={<Coins size={12} />}
          />
          <Stat
            label="Estimated spend today"
            value={
              data?.estimated_cost_usd_today != null
                ? `$${data.estimated_cost_usd_today.toFixed(4)}`
                : "-"
            }
            color="green"
            icon={<DollarSign size={12} />}
          />
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          <Panel title="Spend by agent" subtitle="estimated USD, Claude Haiku pricing">
            {agents.length > 0 ? (
              <BarList
                rows={agents.map((a) => ({
                  label: a.agent_id.slice(0, 16),
                  value: Math.round(a.estimated_cost_usd * 10000) / 10000,
                }))}
                color={PASTEL.green}
              />
            ) : (
              <EmptyState message="No token usage recorded today" />
            )}
          </Panel>

          <Panel title="Token breakdown" bodyClassName="p-0">
            {agents.length > 0 ? (
              <div className="overflow-x-auto">
                <table className="w-full text-[14px]">
                  <thead>
                    <tr className="text-left text-text-tertiary border-b border-border bg-bg-inset/40">
                      <th className="py-2.5 px-4 font-normal">Agent</th>
                      <th className="py-2.5 px-4 font-normal text-right">Tokens in</th>
                      <th className="py-2.5 px-4 font-normal text-right">Tokens out</th>
                      <th className="py-2.5 px-4 font-normal text-right">Cost</th>
                    </tr>
                  </thead>
                  <tbody>
                    {agents.map((a) => (
                      <tr key={a.agent_id} className="border-b border-border/40">
                        <td className="py-2.5 px-4 tnum text-text-secondary">{a.agent_id.slice(0, 16)}</td>
                        <td className="py-2.5 px-4 tnum text-right text-text-primary">
                          {a.total_tokens_in.toLocaleString()}
                        </td>
                        <td className="py-2.5 px-4 tnum text-right text-text-primary">
                          {a.total_tokens_out.toLocaleString()}
                        </td>
                        <td className="py-2.5 px-4 tnum text-right text-green">
                          ${a.estimated_cost_usd.toFixed(4)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <EmptyState message="No agents billed today" />
            )}
          </Panel>
        </div>
      </div>
    </div>
  );
}
