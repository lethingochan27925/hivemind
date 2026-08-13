"use client";

import { useLive } from "@/lib/use-live";
import { api } from "@/lib/api";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Cloud } from "lucide-react";
import { PASTEL } from "@/lib/palette";

const BAR = [PASTEL.blue, PASTEL.teal, PASTEL.purple, PASTEL.orange, PASTEL.green, PASTEL.yellow, PASTEL.red];

/**
 * CloudCostPanel - what the whole platform costs, not just the model calls.
 * Reads AWS Cost Explorer (month-to-date, grouped by service) through the
 * control plane, which caches for hours because each CE query is itself billed.
 */
export function CloudCostPanel({ bedrockToday }: { bedrockToday?: number }) {
  const { data, error } = useLive(api.getCloudCost, 30 * 60 * 1000);

  const rows = data?.by_service ?? [];
  const max = Math.max(1, ...rows.map((r) => r.usd));

  return (
    <Panel
      title="Cloud infrastructure spend"
      subtitle="AWS Cost Explorer - month-to-date, every service this platform runs on"
      actions={
        data?.available ? (
          <Badge variant="blue" dot>
            ${data.total_usd.toFixed(2)} MTD
          </Badge>
        ) : null
      }
    >
      {error && <div className="text-[13px] text-red mb-2">{error}</div>}

      {!data ? (
        <div className="h-24 hm-skeleton" />
      ) : !data.available ? (
        <EmptyState
          message="Cost Explorer not available"
          hint={
            data.note ||
            "Enable Cost Explorer in the AWS billing console (it can take ~24h to populate) - the control plane picks it up automatically."
          }
        />
      ) : rows.length === 0 ? (
        <EmptyState message="No billed usage yet this month" hint="Free-tier usage shows as $0.00." />
      ) : (
        <>
          <div className="flex items-baseline gap-3 mb-3">
            <span className="tnum text-2xl font-bold text-text-primary">${data.total_usd.toFixed(2)}</span>
            <span className="text-[13px] text-text-tertiary">
              AWS, {data.period_start} → today
            </span>
            {bedrockToday != null && (
              <span className="text-[13px] text-text-tertiary ml-auto">
                agent tokens today:{" "}
                <span className="tnum text-text-secondary">${bedrockToday.toFixed(4)}</span>
              </span>
            )}
          </div>

          <ul className="flex flex-col gap-2">
            {rows.slice(0, 10).map((r, i) => (
              <li key={r.service} className="flex items-center gap-3 text-[13px]">
                <span className="w-44 shrink-0 text-text-secondary truncate" title={r.service}>
                  {r.service}
                </span>
                <div className="flex-1 h-4 rounded bg-bg-inset overflow-hidden">
                  <div
                    className="h-full rounded transition-[width] duration-500"
                    style={{ width: `${(r.usd / max) * 100}%`, background: BAR[i % BAR.length], opacity: 0.85 }}
                  />
                </div>
                <span className="tnum w-20 text-right text-text-primary font-semibold">
                  ${r.usd.toFixed(r.usd < 1 ? 4 : 2)}
                </span>
              </li>
            ))}
          </ul>

          <div className="mt-3 pt-2.5 border-t border-border flex items-center gap-2 text-[12px] text-text-tertiary">
            <Cloud size={13} />
            Cached for {Math.round(data.cache_minutes / 60)}h - Cost Explorer bills per query.
            {data.refreshed_at && <span className="ml-auto tnum">refreshed {new Date(data.refreshed_at).toLocaleString()}</span>}
          </div>
        </>
      )}
    </Panel>
  );
}
