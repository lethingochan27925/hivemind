"use client";

import { ReactNode, useCallback, useEffect, useRef, useState } from "react";
import { ChevronDown, ChevronRight, Maximize2, Minimize2, GripHorizontal } from "lucide-react";
import { useT } from "@/lib/i18n";

interface Size {
  h?: number;
  w?: number;
}

/**
 * Panel - the base surface for every dashboard block, and the one place where
 * panel behaviour lives:
 *   - title/subtitle are translated, so every page is bilingual for free;
 *   - DRAG to resize: grab the grip on the bottom edge to change height, or the
 *     bottom-right corner to change height and width. Double-click a grip to
 *     snap back to automatic size;
 *   - collapse / expand and maximize, all remembered per panel across reloads.
 *
 * The drag is implemented with pointer capture rather than CSS `resize`, because
 * the card clips its own overflow (rounded corners) and the native handle ended
 * up invisible and impossible to grab.
 */
export function Panel({
  title,
  subtitle,
  actions,
  children,
  className = "",
  bodyClassName = "",
  resizable = true,
  collapsible = true,
}: {
  title?: string;
  subtitle?: string;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  bodyClassName?: string;
  resizable?: boolean;
  collapsible?: boolean;
}) {
  const t = useT();
  const key = title ? `hm-panel:${title}` : null;

  const [collapsed, setCollapsed] = useState(false);
  const [maximized, setMaximized] = useState(false);
  const [size, setSize] = useState<Size>({});
  const [dragging, setDragging] = useState(false);

  const bodyRef = useRef<HTMLDivElement>(null);
  const sectionRef = useRef<HTMLElement>(null);
  const start = useRef({ y: 0, x: 0, h: 0, w: 0, axis: "y" as "y" | "xy" });

  // Restore collapsed + size.
  useEffect(() => {
    if (!key) return;
    try {
      const raw = localStorage.getItem(key);
      if (!raw) return;
      const saved = JSON.parse(raw) as { collapsed?: boolean; size?: Size };
      if (saved.collapsed) setCollapsed(true);
      if (saved.size) setSize(saved.size);
    } catch {}
  }, [key]);

  const persist = useCallback(
    (next: { collapsed?: boolean; size?: Size }) => {
      if (!key) return;
      try {
        const raw = localStorage.getItem(key);
        const prev = raw ? JSON.parse(raw) : {};
        localStorage.setItem(key, JSON.stringify({ ...prev, ...next }));
      } catch {}
    },
    [key]
  );

  const toggleCollapse = () => {
    const next = !collapsed;
    setCollapsed(next);
    persist({ collapsed: next });
  };

  // Esc closes the maximized overlay.
  useEffect(() => {
    if (!maximized) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setMaximized(false);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [maximized]);

  const onGripDown = (axis: "y" | "xy") => (e: React.PointerEvent<HTMLDivElement>) => {
    if (!bodyRef.current || !sectionRef.current) return;
    e.preventDefault();
    (e.target as HTMLElement).setPointerCapture(e.pointerId);
    start.current = {
      y: e.clientY,
      x: e.clientX,
      h: bodyRef.current.getBoundingClientRect().height,
      w: sectionRef.current.getBoundingClientRect().width,
      axis,
    };
    setDragging(true);
  };

  const onGripMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!dragging) return;
    const s = start.current;
    const next: Size = { h: Math.max(56, s.h + (e.clientY - s.y)) };
    if (s.axis === "xy") next.w = Math.max(280, s.w + (e.clientX - s.x));
    else if (size.w) next.w = size.w;
    setSize(next);
  };

  const onGripUp = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!dragging) return;
    (e.target as HTMLElement).releasePointerCapture(e.pointerId);
    setDragging(false);
    persist({ size });
  };

  const resetSize = () => {
    setSize({});
    persist({ size: {} });
  };

  const iconBtn =
    "flex items-center justify-center w-6 h-6 rounded text-text-tertiary hover:text-text-primary hover:bg-bg-panel-hover transition-colors";

  const header = title ? (
    <header className="flex items-center justify-between gap-3 px-4 h-12 border-b border-border bg-bg-inset/30 shrink-0">
      <div className="flex items-baseline gap-2.5 min-w-0">
        {collapsible && (
          <button
            onClick={toggleCollapse}
            className="text-text-tertiary hover:text-text-primary transition-colors self-center"
            title={collapsed ? t("Expand") : t("Collapse")}
            aria-expanded={!collapsed}
          >
            {collapsed ? <ChevronRight size={14} /> : <ChevronDown size={14} />}
          </button>
        )}
        <h2 className="text-[13px] font-bold uppercase tracking-wide text-text-primary truncate">
          {t(title)}
        </h2>
        {subtitle && <span className="text-[12px] text-text-tertiary truncate">{t(subtitle)}</span>}
      </div>
      <div className="flex items-center gap-2 shrink-0">
        {actions}
        <button
          onClick={() => setMaximized(!maximized)}
          className={iconBtn}
          title={maximized ? t("Restore size") : t("Maximize")}
        >
          {maximized ? <Minimize2 size={13} /> : <Maximize2 size={13} />}
        </button>
      </div>
    </header>
  ) : null;

  const body = (
    <div
      ref={bodyRef}
      style={maximized ? undefined : size.h ? { height: size.h } : undefined}
      className={`p-4 ${bodyClassName} ${maximized ? "overflow-auto flex-1" : size.h ? "overflow-auto" : ""}`}
    >
      {children}
    </div>
  );

  if (maximized) {
    return (
      <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-[2px] p-4 sm:p-8 flex">
        <section className="flex-1 flex flex-col border border-border-strong rounded-lg bg-bg-panel shadow-2xl overflow-hidden">
          {header}
          {body}
        </section>
      </div>
    );
  }

  return (
    <section
      ref={sectionRef}
      style={size.w ? { width: size.w, maxWidth: "100%" } : undefined}
      className={`relative border ${
        dragging ? "border-blue/60" : "border-border"
      } rounded-lg bg-bg-panel/70 overflow-hidden ${className}`}
    >
      {header}
      {!collapsed && body}

      {/* Drag grips: bottom edge = height, bottom-right corner = height + width */}
      {resizable && !collapsed && (
        <>
          <div
            onPointerDown={onGripDown("y")}
            onPointerMove={onGripMove}
            onPointerUp={onGripUp}
            onDoubleClick={resetSize}
            title={t("Drag to resize · double-click to reset")}
            className="group absolute left-0 right-6 bottom-0 h-3 flex items-center justify-center cursor-ns-resize touch-none"
          >
            <GripHorizontal
              size={14}
              className={`transition-opacity ${
                dragging ? "opacity-100 text-blue" : "opacity-30 text-text-tertiary group-hover:opacity-80"
              }`}
            />
          </div>
          <div
            onPointerDown={onGripDown("xy")}
            onPointerMove={onGripMove}
            onPointerUp={onGripUp}
            onDoubleClick={resetSize}
            title={t("Drag to resize · double-click to reset")}
            className="group absolute right-0 bottom-0 w-4 h-4 cursor-nwse-resize touch-none"
          >
            <span
              className={`absolute right-[3px] bottom-[3px] w-2 h-2 border-b-2 border-r-2 rounded-br transition-colors ${
                dragging ? "border-blue" : "border-text-tertiary/40 group-hover:border-text-tertiary"
              }`}
            />
          </div>
        </>
      )}
    </section>
  );
}
