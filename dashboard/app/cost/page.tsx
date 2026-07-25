"use client";

import { useEffect, useState } from "react";
import { api, CostData } from "@/lib/api";
import { Panel } from "@/components/ui/panel";
import { Stat } from "@/components/ui/stat";
import { PageHeader } from "@/components/ui/page-header";
import { EmptyState } from "@/components/ui/empty-state";
import { Badge } from "@/components/ui/badge";

export default function CostPage() {
  const [data, setData] = useState<CostData | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const load = () => {
      api
        .getCost()
        .then(setData)
        .catch((e) => setError(e.message));
    };
    load();
    const interval = setInterval(load, 10000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div>
      <PageHeader
        title="Cost"
        description="Bedrock token usage and estimated spend, broken down by agent"
        actions={error && <Badge variant="red">API unreachable</Badge>}
      />

      <div className="p-4 space-y-3">
        <div className="grid grid-cols-2 border border-border rounded-sm divide-x divide-border">
          <Stat
            label="Tokens used today"
            value={data?.total_tokens_today?.toLocaleString() ?? "—"}
            color="blue"
          />
          <Stat
            label="Estimated cost today"
            value={
              data?.estimated_cost_usd_today != null
                ? `$${data.estimated_cost_usd_today.toFixed(4)}`
                : "—"
            }
            color="green"
          />
        </div>

        <Panel title="Cost by agent">
          {data?.by_agent && data.by_agent.length > 0 ? (
            <table className="w-full text-xs">
              <thead>
                <tr className="text-left text-text-tertiary border-b border-border">
                  <th className="pb-1.5 font-normal">Agent</th>
                  <th className="pb-1.5 font-normal text-right">Tokens in</th>
                  <th className="pb-1.5 font-normal text-right">Tokens out</th>
                  <th className="pb-1.5 font-normal text-right">Est. cost</th>
                </tr>
              </thead>
              <tbody>
                {data.by_agent.map((a) => (
                  <tr key={a.agent_id} className="border-b border-border/50">
                    <td className="py-1.5 text-text-secondary">
                      {a.agent_id.slice(0, 16)}
                    </td>
                    <td className="py-1.5 text-right text-text-primary">
                      {a.total_tokens_in.toLocaleString()}
                    </td>
                    <td className="py-1.5 text-right text-text-primary">
                      {a.total_tokens_out.toLocaleString()}
                    </td>
                    <td className="py-1.5 text-right text-green">
                      ${a.estimated_cost_usd.toFixed(4)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <EmptyState message="No token usage recorded today" />
          )}
        </Panel>
      </div>
    </div>
  );
}
