"use client";

import { useCallback, useEffect, useState } from "react";
import { api, MemoryRow } from "@/lib/api";
import { Panel } from "@/components/ui/panel";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { Pin, PinOff, Archive, ArchiveRestore, Trash2, Loader2, TimerReset, Layers } from "lucide-react";

/**
 * MemoryAdmin - operate the episodic memory itself: pin a case so decay can
 * never forget it, archive a bad memory out of the vector index, delete it
 * outright, or run the decay / bulk-archive jobs on demand. This is the
 * memory layer as a managed resource, not a read-only chart.
 */
export function MemoryAdmin() {
  const [rows, setRows] = useState<MemoryRow[]>([]);
  const [showArchived, setShowArchived] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(
    (archived: boolean) => {
      api
        .getMemories({ limit: 60, archived })
        .then((r) => setRows(r ?? []))
        .catch((e) => setMsg((e as Error).message))
        .finally(() => setLoading(false));
    },
    []
  );

  useEffect(() => {
    load(showArchived);
  }, [showArchived, load]);

  const act = async (key: string, fn: () => Promise<unknown>, done: string) => {
    setBusy(key);
    setMsg(null);
    try {
      await fn();
      setMsg(done);
      load(showArchived);
    } catch (e) {
      setMsg((e as Error).message);
    } finally {
      setBusy(null);
    }
  };

  const iconBtn =
    "p-1.5 rounded border border-border text-text-tertiary hover:text-text-primary hover:border-border-strong disabled:opacity-40 transition-colors";

  return (
    <Panel
      title="Memory administration"
      subtitle="pin, archive, forget - operate what the fleet remembers"
      actions={
        <div className="flex items-center gap-2">
          <button
            onClick={() => setShowArchived(!showArchived)}
            className={`px-2.5 py-1 rounded-md border text-[12px] transition-colors ${
              showArchived
                ? "border-blue/40 bg-blue/10 text-blue"
                : "border-border text-text-secondary hover:text-text-primary"
            }`}
          >
            {showArchived ? "showing archived" : "live only"}
          </button>
          <button
            disabled={busy !== null}
            onClick={() => act("decay", () => api.memoryJob("decay"), "Decay job invoked - salience ages down, forgotten cases archive.")}
            className="flex items-center gap-1.5 px-2.5 py-1 rounded-md border border-border text-[12px] text-text-secondary hover:text-text-primary disabled:opacity-40 transition-colors"
          >
            {busy === "decay" ? <Loader2 size={12} className="animate-spin" /> : <TimerReset size={12} />}
            Run decay
          </button>
          <button
            disabled={busy !== null}
            onClick={() => {
              if (!confirm("Archive every live memory with salience below 0.3? They drop out of recall immediately.")) return;
              act("archive_below", () => api.memoryJob("archive_below", 0.3), "Archived every memory below salience 0.3.");
            }}
            className="flex items-center gap-1.5 px-2.5 py-1 rounded-md border border-border text-[12px] text-text-secondary hover:text-text-primary disabled:opacity-40 transition-colors"
          >
            {busy === "archive_below" ? <Loader2 size={12} className="animate-spin" /> : <Layers size={12} />}
            Archive &lt; 0.3
          </button>
        </div>
      }
      bodyClassName="p-0"
    >
      {loading ? (
        <div className="p-3 space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-6 hm-skeleton" />
          ))}
        </div>
      ) : rows.length === 0 ? (
        <EmptyState
          message="No memories yet"
          hint="Each closed case is summarised, embedded and consolidated into this table."
        />
      ) : (
        <div className="overflow-x-auto max-h-[52vh]">
          <table className="w-full text-[13px]">
            <thead className="sticky top-0 bg-bg-panel">
              <tr className="text-left text-text-tertiary border-b border-border">
                <th className="py-2.5 px-4 font-normal">Case summary</th>
                <th className="py-2.5 px-3 font-normal">Pattern</th>
                <th className="py-2.5 px-3 font-normal text-right">Salience</th>
                <th className="py-2.5 px-3 font-normal text-right">Recalls</th>
                <th className="py-2.5 px-3 font-normal text-right">Merges</th>
                <th className="py-2.5 px-3 font-normal text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((m) => (
                <tr key={m.id} className={`border-b border-border/40 ${m.archived ? "opacity-55" : ""}`}>
                  <td className="py-2 px-4 text-text-secondary max-w-[420px] truncate" title={m.summary}>
                    {m.summary}
                  </td>
                  <td className="py-2 px-3">
                    <Badge variant={m.verdict === "fraud" ? "red" : m.verdict === "legit" ? "green" : "yellow"}>
                      {m.pattern_type ?? m.verdict}
                    </Badge>
                  </td>
                  <td className="py-2 px-3 tnum text-right text-text-primary font-semibold">
                    {m.salience.toFixed(2)}
                  </td>
                  <td className="py-2 px-3 tnum text-right text-text-secondary">{m.recall_count}</td>
                  <td className="py-2 px-3 tnum text-right text-text-secondary">{m.merge_count}</td>
                  <td className="py-2 px-3">
                    <div className="flex items-center justify-end gap-1.5">
                      {m.salience >= 2 ? (
                        <button
                          disabled={busy !== null}
                          onClick={() => act(`unpin-${m.id}`, () => api.memoryAction("unpin", m.id), "Unpinned.")}
                          className={iconBtn}
                          title="Unpin (salience back to 1.0)"
                          aria-label="Unpin (salience back to 1.0)"
                        >
                          <PinOff size={13} />
                        </button>
                      ) : (
                        <button
                          disabled={busy !== null}
                          onClick={() => act(`pin-${m.id}`, () => api.memoryAction("pin", m.id), "Pinned - decay can no longer forget it.")}
                          className={iconBtn}
                          title="Pin (max salience)"
                          aria-label="Pin (max salience)"
                        >
                          <Pin size={13} />
                        </button>
                      )}
                      {m.archived ? (
                        <button
                          disabled={busy !== null}
                          onClick={() => act(`un-${m.id}`, () => api.memoryAction("unarchive", m.id), "Restored to the vector index.")}
                          className={iconBtn}
                          title="Restore into the vector index"
                          aria-label="Restore into the vector index"
                        >
                          <ArchiveRestore size={13} />
                        </button>
                      ) : (
                        <button
                          disabled={busy !== null}
                          onClick={() => act(`ar-${m.id}`, () => api.memoryAction("archive", m.id), "Archived - excluded from recall.")}
                          className={iconBtn}
                          title="Archive (excluded from the partial vector index)"
                          aria-label="Archive (excluded from the partial vector index)"
                        >
                          <Archive size={13} />
                        </button>
                      )}
                      <button
                        disabled={busy !== null}
                        onClick={() => {
                          if (confirm("Delete this memory permanently?")) {
                            act(`del-${m.id}`, () => api.memoryAction("delete", m.id), "Memory deleted.");
                          }
                        }}
                        className={`${iconBtn} hover:text-red`}
                        title="Delete permanently"
                        aria-label="Delete permanently"
                      >
                        <Trash2 size={13} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {msg && <div className="px-4 py-2.5 border-t border-border text-[13px] text-text-secondary">{msg}</div>}
    </Panel>
  );
}
