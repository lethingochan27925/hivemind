"use client";

import Link from "next/link";
import { useState, useSyncExternalStore } from "react";
import { usePathname } from "next/navigation";
import { useT } from "@/lib/i18n";
import {
  LayoutGrid,
  ClipboardCheck,
  Brain,
  Activity,
  ServerCog,
  DollarSign,
  Database,
  Network,
  GitBranch,
  GraduationCap,
} from "lucide-react";

// Platform-style grouped navigation: what you DO, what you WATCH, what you RUN ON.
const sections = [
  {
    label: "Operate",
    items: [
      { href: "/", label: "Mission Control", icon: LayoutGrid },
      { href: "/reviews", label: "Review Queue", icon: ClipboardCheck },
      { href: "/training", label: "Training Lab", icon: GraduationCap },
    ],
  },
  {
    label: "Observe",
    items: [
      { href: "/transactions", label: "Transactions", icon: Activity },
      { href: "/memory", label: "Fleet & Memory", icon: Brain },
      { href: "/cost", label: "Cost", icon: DollarSign },
    ],
  },
  {
    label: "Platform",
    items: [
      { href: "/pipeline", label: "Pipeline", icon: GitBranch },
      { href: "/database", label: "Database", icon: Database },
      { href: "/architecture", label: "Architecture", icon: Network },
      { href: "/infrastructure", label: "Infrastructure", icon: ServerCog },
    ],
  },
];

// Be rong nav luu trong localStorage - doc bang useSyncExternalStore (external
// store), khong phai setState trong effect.
const DEFAULT_W = 240;

function subscribeWidth(onChange: () => void) {
  window.addEventListener("hm-sidebar-resize", onChange);
  return () => window.removeEventListener("hm-sidebar-resize", onChange);
}

function readWidth(): number {
  try {
    const saved = Number(localStorage.getItem("hm-sidebar-w"));
    if (saved >= 180 && saved <= 420) return saved;
  } catch {}
  return DEFAULT_W;
}

function writeWidth(w: number) {
  try {
    localStorage.setItem("hm-sidebar-w", String(w));
  } catch {}
  window.dispatchEvent(new Event("hm-sidebar-resize"));
}

// Below this the sidebar at its saved width (180-420px) can take over half a
// phone-size viewport with no way to shrink it, since the resizable width was
// only ever meant to be adjusted by dragging. Narrow viewports instead get a
// fixed icon-only rail - no new open/close state to plumb through the
// topbar, and every link stays one tap away instead of being hidden behind a
// drawer trigger that would not exist without that extra state.
const NARROW_QUERY = "(max-width: 767px)";
const RAIL_W = 60;

function subscribeNarrow(onChange: () => void) {
  const mq = window.matchMedia(NARROW_QUERY);
  mq.addEventListener("change", onChange);
  return () => mq.removeEventListener("change", onChange);
}
function readNarrow(): boolean {
  return window.matchMedia(NARROW_QUERY).matches;
}

export function Sidebar() {
  const pathname = usePathname();
  const t = useT();

  const stored = useSyncExternalStore(subscribeWidth, readWidth, () => DEFAULT_W);
  const narrow = useSyncExternalStore(subscribeNarrow, readNarrow, () => false);
  const [dragWidth, setDragWidth] = useState<number | null>(null);
  const sidebarWidth = narrow ? RAIL_W : dragWidth ?? stored;

  // Handler duoc tao trong chinh luc keo: khong con phu thuoc vong giua onMove
  // va onUp, va tu don dep khi tha chuot.
  const onDown = (e: React.PointerEvent) => {
    e.preventDefault();
    const startX = e.clientX;
    const startW = sidebarWidth;
    let latest = startW;

    const move = (ev: PointerEvent) => {
      latest = Math.max(180, Math.min(420, startW + (ev.clientX - startX)));
      setDragWidth(latest);
    };
    const up = () => {
      document.removeEventListener("pointermove", move);
      document.removeEventListener("pointerup", up);
      writeWidth(latest);
      setDragWidth(null);
    };
    document.addEventListener("pointermove", move);
    document.addEventListener("pointerup", up);
  };

  const dragging = dragWidth !== null;

  // Every element the rail (narrow) drops relative to the full sidebar
  // (wide) lives here, computed once and named for what it is, instead of
  // as inline `{!narrow && (...)}` guards scattered through the JSX below -
  // one place to scan for "what does narrow mode hide" instead of five, and
  // one place a future wide-only addition is expected to join instead of
  // inventing its own ad-hoc guard.
  const resizeGrip = !narrow && (
    <div
      onPointerDown={onDown}
      onDoubleClick={() => writeWidth(DEFAULT_W)}
      title={t("Drag to resize · double-click to reset")}
      className={`absolute top-0 right-0 w-1.5 h-full cursor-ew-resize touch-none transition-colors ${
        dragging ? "bg-blue/60" : "hover:bg-blue/30"
      }`}
    />
  );
  const brandSubtitle = !narrow && (
    <div className="leading-tight">
      <div className="text-[17px] font-bold text-text-primary tracking-tight">HiveMind</div>
      <div className="text-[10px] uppercase tracking-[0.16em] text-text-tertiary">
        {t("Control Platform")}
      </div>
    </div>
  );
  const footer = !narrow && (
    <div className="px-5 py-4 border-t border-border text-[12px] text-text-tertiary leading-relaxed">
      <div className="flex items-center gap-2">
        <span className="w-2 h-2 rounded-full bg-green" />
        CockroachDB · Bedrock · Lambda
      </div>
      <div className="mt-1 opacity-70">{t("Agentic memory control plane")}</div>
    </div>
  );

  return (
    <aside
      style={{ width: sidebarWidth }}
      className="relative shrink-0 border-r border-border bg-bg-panel/60 flex flex-col"
    >
      {/* Kéo cạnh phải để đổi bề rộng - vô hiệu khi ở rail hẹp */}
      {resizeGrip}
      <div className={`h-16 flex items-center gap-3 border-b border-border ${narrow ? "justify-center px-2" : "px-5"}`}>
        <span className="w-8 h-8 rounded-lg bg-blue/15 border border-blue/30 flex items-center justify-center shrink-0">
          <span className="w-2.5 h-2.5 rounded-full bg-blue hm-pulse" />
        </span>
        {brandSubtitle}
      </div>

      <nav className="flex-1 py-2 overflow-y-auto">
        {sections.map((section) => (
          <div key={section.label} className="mb-1">
            {!narrow && (
              <div className="px-5 pt-3 pb-1 text-[10px] uppercase tracking-[0.16em] text-text-tertiary font-bold">
                {t(section.label)}
              </div>
            )}
            {section.items.map((item) => {
              const isActive =
                item.href === "/" ? pathname === "/" : pathname.startsWith(item.href);
              const Icon = item.icon;
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  title={narrow ? t(item.label) : undefined}
                  aria-label={narrow ? t(item.label) : undefined}
                  className={`relative flex items-center gap-3 py-2 text-[14px] transition-colors ${
                    narrow ? "justify-center px-2" : "px-5"
                  } ${
                    isActive
                      ? "text-text-primary bg-bg-panel-hover font-semibold"
                      : "text-text-secondary hover:text-text-primary hover:bg-bg-panel-hover/50"
                  }`}
                >
                  {isActive && (
                    <span className="absolute left-0 top-1.5 bottom-1.5 w-[3px] rounded-full bg-blue" />
                  )}
                  <Icon size={17} strokeWidth={1.8} />
                  {!narrow && t(item.label)}
                </Link>
              );
            })}
          </div>
        ))}
      </nav>

      {footer}
    </aside>
  );
}
