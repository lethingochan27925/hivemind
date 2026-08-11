"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import {
  LayoutGrid,
  ClipboardCheck,
  Brain,
  Activity,
  Database,
  DollarSign,
  Network,
  ServerCog,
  GitBranch,
  Play,
  Pause,
  Zap,
  Plus,
  Search,
  Loader2,
  CornerDownLeft,
} from "lucide-react";
import { ReactNode } from "react";

/**
 * CommandPalette - Ctrl/Cmd+K, Linear/Vercel style. One keystroke away from any
 * page AND any live mutation: start/pause the fleet, dispatch, feed cases,
 * simulate a crash. The platform is fully operable from the keyboard.
 */

interface Item {
  id: string;
  group: "Go to" | "Actions";
  label: string;
  keywords: string;
  icon: ReactNode;
  run: () => Promise<string | void> | string | void;
}

export function CommandPalette() {
  const router = useRouter();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [index, setIndex] = useState(0);
  const [busy, setBusy] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  const close = useCallback(() => {
    setOpen(false);
    setQuery("");
    setIndex(0);
    setMsg(null);
  }, []);

  const items: Item[] = useMemo(() => {
    const nav = (path: string, label: string, keywords: string, icon: ReactNode): Item => ({
      id: `nav:${path}`,
      group: "Go to",
      label,
      keywords,
      icon,
      run: () => {
        router.push(path);
        close();
      },
    });
    const act = (
      id: string,
      label: string,
      keywords: string,
      icon: ReactNode,
      fn: () => Promise<string>
    ): Item => ({
      id: `act:${id}`,
      group: "Actions",
      label,
      keywords,
      icon,
      run: async () => {
        setBusy(id);
        setMsg(null);
        try {
          setMsg(await fn());
        } catch (e) {
          setMsg(`Failed: ${(e as Error).message}`);
        } finally {
          setBusy(null);
        }
      },
    });

    return [
      nav("/", "Mission Control", "overview home kpi verdict", <LayoutGrid size={15} />),
      nav("/reviews", "Review Queue", "human approve reject escalate", <ClipboardCheck size={15} />),
      nav("/memory", "Fleet & Memory", "episodic salience agents patterns", <Brain size={15} />),
      nav("/transactions", "Transactions", "audit decision trace verdict", <Activity size={15} />),
      nav("/database", "Database", "cockroachdb sql query console rows", <Database size={15} />),
      nav("/cost", "Cost", "tokens spend bedrock usd", <DollarSign size={15} />),
      nav("/pipeline", "Pipeline", "ci cd github actions deploy canary", <GitBranch size={15} />),
      nav("/architecture", "Architecture", "topology map infra resources", <Network size={15} />),
      nav("/infrastructure", "Infrastructure", "health alarms chaos nodes lambda", <ServerCog size={15} />),

      act("start", "Start fleet", "run enable schedules resume", <Play size={15} />, async () => {
        await api.setFleetState("start");
        return "Fleet started - EventBridge schedules enabled.";
      }),
      act("pause", "Pause fleet", "stop disable schedules halt", <Pause size={15} />, async () => {
        await api.setFleetState("pause");
        return "Fleet paused - schedules disabled.";
      }),
      act("dispatch", "Run dispatch cycle", "invoke workers queue", <Zap size={15} />, async () => {
        const r = await api.runDispatch();
        const d = r.dispatcher_result;
        return `Dispatch: created ${d.tasks_created}, invoked ${d.workers_invoked} workers.`;
      }),
      act("feed", "Feed 50 cases", "stream replay transactions inject", <Plus size={15} />, async () => {
        const r = await api.feedStream(50);
        return `Fed ${r.requeued} cases into the queue.`;
      }),
      act("chaos", "Simulate agent crash", "kill chaos reaper resume test", <Zap size={15} />, async () => {
        const r = await api.simulateCrash();
        router.push("/infrastructure");
        return `Killed task ${r.task_id.slice(0, 8)} - watch the recovery tracker.`;
      }),
    ];
  }, [router, close]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return items;
    return items.filter(
      (i) => i.label.toLowerCase().includes(q) || i.keywords.includes(q)
    );
  }, [items, query]);

  // Global shortcuts
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((o) => !o);
      } else if (e.key === "Escape") {
        close();
      }
    };
    const onOpen = () => setOpen(true);
    window.addEventListener("keydown", onKey);
    window.addEventListener("hm-cmdk", onOpen);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener("hm-cmdk", onOpen);
    };
  }, [close]);

  useEffect(() => {
    if (open) setTimeout(() => inputRef.current?.focus(), 10);
  }, [open]);

  useEffect(() => {
    setIndex(0);
  }, [query]);

  if (!open) return null;

  const groups: ("Go to" | "Actions")[] = ["Go to", "Actions"];
  const flat = filtered;

  return (
    <div
      className="fixed inset-0 z-50 bg-black/60 backdrop-blur-[2px] flex items-start justify-center pt-[14vh]"
      onClick={close}
      role="dialog"
      aria-modal="true"
    >
      <div
        className="w-[600px] max-w-[92vw] rounded-xl border border-border-strong bg-bg-panel shadow-2xl overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-2.5 px-4 h-12 border-b border-border">
          <Search size={15} className="text-text-tertiary" />
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown") {
                e.preventDefault();
                setIndex((i) => Math.min(i + 1, flat.length - 1));
              } else if (e.key === "ArrowUp") {
                e.preventDefault();
                setIndex((i) => Math.max(i - 1, 0));
              } else if (e.key === "Enter" && flat[index]) {
                flat[index].run();
              }
            }}
            placeholder="Go to a page or run an action…"
            className="flex-1 bg-transparent outline-none text-[15px] text-text-primary placeholder:text-text-tertiary"
          />
          <kbd className="text-[11px] text-text-tertiary border border-border rounded px-1.5 py-0.5">esc</kbd>
        </div>

        <div className="max-h-[46vh] overflow-y-auto py-1.5">
          {flat.length === 0 && (
            <div className="px-4 py-6 text-[13px] text-text-tertiary">No matches.</div>
          )}
          {groups.map((g) => {
            const group = flat.filter((i) => i.group === g);
            if (group.length === 0) return null;
            return (
              <div key={g}>
                <div className="px-4 pt-2 pb-1 text-[10px] uppercase tracking-[0.16em] text-text-tertiary font-bold">
                  {g}
                </div>
                {group.map((item) => {
                  const i = flat.indexOf(item);
                  const active = i === index;
                  return (
                    <button
                      key={item.id}
                      onClick={() => item.run()}
                      onMouseEnter={() => setIndex(i)}
                      className={`w-full flex items-center gap-3 px-4 py-2 text-left text-[14px] transition-colors ${
                        active ? "bg-blue/10 text-text-primary" : "text-text-secondary"
                      }`}
                    >
                      <span className={active ? "text-blue" : "text-text-tertiary"}>
                        {busy === item.id.replace("act:", "") ? (
                          <Loader2 size={15} className="animate-spin" />
                        ) : (
                          item.icon
                        )}
                      </span>
                      {item.label}
                      {active && (
                        <CornerDownLeft size={13} className="ml-auto text-text-tertiary" />
                      )}
                    </button>
                  );
                })}
              </div>
            );
          })}
        </div>

        {msg && (
          <div className="px-4 py-2.5 border-t border-border text-[13px] text-text-secondary">
            {msg}
          </div>
        )}
      </div>
    </div>
  );
}
