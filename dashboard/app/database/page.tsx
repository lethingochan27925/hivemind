"use client";

import { useEffect, useState } from "react";
import { api, QueryResult } from "@/lib/api";
import { useLive } from "@/lib/use-live";
import { useQueryParam } from "@/lib/use-query-param";
import { Panel } from "@/components/ui/panel";
import { Stat } from "@/components/ui/stat";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { PageHeader } from "@/components/ui/page-header";
import { Database, Play, Loader2, Table2, Download } from "lucide-react";
import { useT } from "@/lib/i18n";

const PRESETS: { label: string; sql: string }[] = [
  { label: "Verdict breakdown", sql: "SELECT verdict, COUNT(*) AS n FROM tasks WHERE verdict IS NOT NULL GROUP BY verdict ORDER BY n DESC" },
  { label: "Learned patterns", sql: "SELECT pattern_type, COUNT(*) AS cases, SUM(merge_count) AS merges FROM case_memory WHERE archived = false GROUP BY pattern_type ORDER BY merges DESC" },
  { label: "Audit actions", sql: "SELECT action, COUNT(*) AS n FROM audit_log GROUP BY action ORDER BY n DESC LIMIT 15" },
  { label: "Task status", sql: "SELECT status, COUNT(*) AS n FROM tasks GROUP BY status ORDER BY n DESC" },
];

const TABLE_TONE: Record<string, "blue" | "green" | "purple" | "yellow"> = {
  transactions: "blue",
  tasks: "green",
  case_memory: "purple",
  audit_log: "yellow",
};

export default function DatabasePage() {
  const t = useT();
  const { data, error, lastUpdated } = useLive(api.getDbStats, 10000);
  const presetSql = useQueryParam("sql");
  const [sql, setSql] = useState(PRESETS[0].sql);
  const [result, setResult] = useState<QueryResult | null>(null);
  const [queryErr, setQueryErr] = useState<string | null>(null);
  const [running, setRunning] = useState(false);

  const run = async (q?: string) => {
    const query = q ?? sql;
    if (q) setSql(q);
    setRunning(true);
    setQueryErr(null);
    try {
      setResult(await api.runQuery(query));
    } catch (e) {
      setResult(null);
      setQueryErr((e as Error).message);
    } finally {
      setRunning(false);
    }
  };

  // Deep-link tu global search: /database?sql=<query> tu dien va chay luon.
  // Moi setState nam trong continuation cua promise, khong goi dong bo trong
  // than effect - do la cai gay cascading render.
  useEffect(() => {
    if (!presetSql) return;
    let alive = true;
    api
      .runQuery(presetSql)
      .then((r) => {
        if (!alive) return;
        setSql(presetSql);
        setResult(r);
        setQueryErr(null);
      })
      .catch((e) => {
        if (!alive) return;
        setSql(presetSql);
        setResult(null);
        setQueryErr((e as Error).message);
      });
    return () => {
      alive = false;
    };
  }, [presetSql]);

  const fmt = (v: unknown) =>
    v === null || v === undefined
      ? "-"
      : typeof v === "object"
        ? JSON.stringify(v)
        : String(v);

  return (
    <div>
      <PageHeader
        title="Database"
        description="CockroachDB - the fleet's shared memory. Inspect tables and run read-only queries."
        lastUpdated={lastUpdated}
        error={error}
        actions={
          data ? (
            <Badge variant="purple" dot>
              {data.database} · {data.total_rows.toLocaleString()} rows
            </Badge>
          ) : null
        }
      />

      <div className="p-6 space-y-5 hm-enter">
        <div className="grid grid-cols-2 md:grid-cols-4 border border-border rounded-md divide-y md:divide-y-0 md:divide-x divide-border bg-bg-panel/40">
          {(data?.tables ?? []).map((tbl) => (
            <Stat
              key={tbl.table}
              label={tbl.table}
              value={tbl.rows.toLocaleString()}
              color={TABLE_TONE[tbl.table] ?? "default"}
              icon={<Table2 size={13} />}
            />
          ))}
          {!data && <Stat label="Loading…" value="-" />}
        </div>

        <Panel
          title="Query console"
          subtitle="read-only - SELECT / SHOW / WITH"
          actions={<Database size={15} className="text-text-tertiary" />}
        >
          <div className="flex flex-wrap gap-2 mb-3">
            {PRESETS.map((p) => (
              <button
                key={p.label}
                onClick={() => run(p.sql)}
                disabled={running}
                className="px-3 py-1.5 rounded-md border border-border text-[13px] text-text-secondary hover:text-text-primary hover:border-border-strong disabled:opacity-40 transition-colors"
              >
                {t(p.label)}
              </button>
            ))}
          </div>

          <textarea
            value={sql}
            onChange={(e) => setSql(e.target.value)}
            spellCheck={false}
            rows={3}
            className="w-full bg-bg-inset border border-border rounded-lg px-3.5 py-3 text-[13px] tnum text-text-primary outline-none focus:border-blue resize-y"
          />

          <div className="flex items-center gap-3 mt-3">
            <button
              onClick={() => run()}
              disabled={running}
              className="flex items-center gap-2 px-4 py-2 rounded-md bg-blue/15 text-blue border border-blue/30 text-[14px] font-semibold hover:bg-blue/25 disabled:opacity-40 transition-colors"
            >
              {running ? <Loader2 size={15} className="animate-spin" /> : <Play size={15} />}
              {t("Run query")}
            </button>
            {result && (
              <span className="text-[13px] text-text-tertiary tnum">
                {result.row_count} {t("rows")}{result.truncated ? ` ${t("(truncated at 200)")}` : ""}
              </span>
            )}
            {result && result.rows.length > 0 && (
              <button
                onClick={() => downloadCsv(result)}
                className="flex items-center gap-1.5 px-3 py-2 rounded-md border border-border text-[13px] text-text-secondary hover:text-text-primary hover:border-border-strong transition-colors"
                title={t("Export these rows as CSV")}
              >
                <Download size={14} />
                CSV
              </button>
            )}
            <span className="ml-auto text-[12px] text-text-tertiary">
              {t("Mutating statements are rejected server-side.")}
            </span>
          </div>

          {queryErr && <div className="mt-3 text-[13px] text-red">{queryErr}</div>}

          {result && (
            <div className="mt-4 overflow-x-auto border border-border rounded-lg max-h-[52vh]">
              <table className="w-full text-[13px]">
                <thead className="sticky top-0 bg-bg-inset">
                  <tr className="text-left text-text-tertiary border-b border-border">
                    {result.columns.map((c) => (
                      <th key={c} className="py-2.5 px-3.5 font-semibold">
                        {c}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {result.rows.length === 0 ? (
                    <tr>
                      <td colSpan={result.columns.length} className="py-6">
                        <EmptyState message="Query returned no rows" />
                      </td>
                    </tr>
                  ) : (
                    result.rows.map((row, i) => (
                      <tr key={i} className="border-b border-border/40">
                        {row.map((v, j) => (
                          <td key={j} className="py-2 px-3.5 tnum text-text-secondary">
                            {fmt(v)}
                          </td>
                        ))}
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
      </div>
    </div>
  );
}

/** downloadCsv - xuat ket qua query hien tai ra file, khong can backend. */
function downloadCsv(result: QueryResult) {
  const NEEDS_QUOTE = new RegExp('[",\\n]');
  const esc = (v: unknown) => {
    const s = v === null || v === undefined ? "" : typeof v === "object" ? JSON.stringify(v) : String(v);
    return NEEDS_QUOTE.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s;
  };
  const csv = [result.columns.join(","), ...result.rows.map((r) => r.map(esc).join(","))].join("\n");
  const url = URL.createObjectURL(new Blob([csv], { type: "text/csv;charset=utf-8" }));
  const a = document.createElement("a");
  a.href = url;
  a.download = "hivemind-query.csv";
  a.click();
  URL.revokeObjectURL(url);
}
