"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
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
} from "lucide-react";

// Platform-style grouped navigation: what you DO, what you WATCH, what you RUN ON.
const sections = [
  {
    label: "Operate",
    items: [
      { href: "/", label: "Mission Control", icon: LayoutGrid },
      { href: "/reviews", label: "Review Queue", icon: ClipboardCheck },
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

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="w-60 shrink-0 border-r border-border bg-bg-panel/60 flex flex-col">
      <div className="h-16 flex items-center gap-3 px-5 border-b border-border">
        <span className="w-8 h-8 rounded-lg bg-blue/15 border border-blue/30 flex items-center justify-center">
          <span className="w-2.5 h-2.5 rounded-full bg-blue hm-pulse" />
        </span>
        <div className="leading-tight">
          <div className="text-[17px] font-bold text-text-primary tracking-tight">HiveMind</div>
          <div className="text-[10px] uppercase tracking-[0.16em] text-text-tertiary">
            Control Platform
          </div>
        </div>
      </div>

      <nav className="flex-1 py-2 overflow-y-auto">
        {sections.map((section) => (
          <div key={section.label} className="mb-1">
            <div className="px-5 pt-3 pb-1 text-[10px] uppercase tracking-[0.16em] text-text-tertiary font-bold">
              {section.label}
            </div>
            {section.items.map((item) => {
              const isActive =
                item.href === "/" ? pathname === "/" : pathname.startsWith(item.href);
              const Icon = item.icon;
              return (
                <Link
                  key={item.href}
                  href={item.href}
                  className={`relative flex items-center gap-3 px-5 py-2 text-[14px] transition-colors ${
                    isActive
                      ? "text-text-primary bg-bg-panel-hover font-semibold"
                      : "text-text-secondary hover:text-text-primary hover:bg-bg-panel-hover/50"
                  }`}
                >
                  {isActive && (
                    <span className="absolute left-0 top-1.5 bottom-1.5 w-[3px] rounded-full bg-blue" />
                  )}
                  <Icon size={17} strokeWidth={1.8} />
                  {item.label}
                </Link>
              );
            })}
          </div>
        ))}
      </nav>

      <div className="px-5 py-4 border-t border-border text-[12px] text-text-tertiary leading-relaxed">
        <div className="flex items-center gap-2">
          <span className="w-2 h-2 rounded-full bg-green" />
          CockroachDB · Bedrock · Lambda
        </div>
        <div className="mt-1 opacity-70">Agentic memory control plane</div>
      </div>
    </aside>
  );
}
