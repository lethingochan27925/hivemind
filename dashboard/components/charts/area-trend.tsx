"use client";

import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from "recharts";
import { PASTEL, CHART_CHROME } from "@/lib/palette";

/**
 * AreaTrend - a single-series filled line for time-bucketed metrics
 * (memory-recall hits, throughput). Deliberately minimal chrome; theme-safe.
 */
export function AreaTrend({
  data,
  xKey,
  yKey,
  color = PASTEL.blue,
  height = 200,
  unit = "",
}: {
  data: Record<string, unknown>[];
  xKey: string;
  yKey: string;
  color?: string;
  height?: number;
  unit?: string;
}) {
  const gradId = `grad-${yKey}`;
  return (
    <div style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart data={data} margin={{ top: 6, right: 6, bottom: 0, left: -18 }}>
          <defs>
            <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity={0.28} />
              <stop offset="100%" stopColor={color} stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid stroke={CHART_CHROME.grid} strokeDasharray="2 3" vertical={false} />
          <XAxis
            dataKey={xKey}
            stroke={CHART_CHROME.axis}
            fontSize={10}
            tickLine={false}
            axisLine={{ stroke: CHART_CHROME.grid }}
            minTickGap={24}
          />
          <YAxis
            stroke={CHART_CHROME.axis}
            fontSize={10}
            tickLine={false}
            axisLine={false}
            width={34}
            allowDecimals={false}
          />
          <Tooltip
            cursor={{ stroke: CHART_CHROME.cursor }}
            contentStyle={{
              background: "var(--color-bg-panel)",
              border: "1px solid var(--color-border-strong)",
              borderRadius: 8,
              fontSize: 11,
              padding: "6px 8px",
            }}
            labelStyle={{ color: "var(--color-text-secondary)" }}
            formatter={(v) => [`${v ?? ""}${unit}`, ""] as [string, string]}
          />
          <Area
            type="monotone"
            dataKey={yKey}
            stroke={color}
            strokeWidth={1.75}
            fill={`url(#${gradId})`}
            dot={false}
            activeDot={{ r: 3, fill: color, strokeWidth: 0 }}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
