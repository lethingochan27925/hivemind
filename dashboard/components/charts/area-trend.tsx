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

/**
 * AreaTrend - a single-series filled line for time-bucketed metrics
 * (memory-recall hits, throughput). Deliberately minimal chrome.
 */
export function AreaTrend({
  data,
  xKey,
  yKey,
  color = "#5c9dff",
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
          <CartesianGrid stroke="#1a1d24" strokeDasharray="2 3" vertical={false} />
          <XAxis
            dataKey={xKey}
            stroke="#4b515e"
            fontSize={10}
            tickLine={false}
            axisLine={{ stroke: "#232732" }}
            minTickGap={24}
          />
          <YAxis
            stroke="#4b515e"
            fontSize={10}
            tickLine={false}
            axisLine={false}
            width={34}
            allowDecimals={false}
          />
          <Tooltip
            cursor={{ stroke: "#313644" }}
            contentStyle={{
              background: "#121419",
              border: "1px solid #313644",
              borderRadius: 6,
              fontSize: 11,
              padding: "6px 8px",
            }}
            labelStyle={{ color: "#a3a9b5" }}
            formatter={(v) => [`${v ?? ""}${unit}`, ""] as [string, string]}
          />
          <Area
            type="monotone"
            dataKey={yKey}
            stroke={color}
            strokeWidth={1.75}
            fill={`url(#${gradId})`}
            dot={false}
            activeDot={{ r: 3, fill: color, stroke: "#0a0b0d", strokeWidth: 2 }}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
