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
} from "lucide-react";

const navItems = [
  { href: "/", label: "Overview", icon: LayoutGrid },
  { href: "/reviews", label: "Review Queue", icon: ClipboardCheck },
  { href: "/memory", label: "Fleet & Memory", icon: Brain },
  { href: "/transactions", label: "Transactions", icon: Activity },
  { href: "/cost", label: "Cost", icon: DollarSign },
  { href: "/infrastructure", label: "Infrastructure", icon: ServerCog },
];

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="w-52 shrink-0 border-r border-border bg-bg-panel flex flex-col">
      <div className="h-12 flex items-center px-4 border-b border-border">
        <span className="text-sm font-medium text-text-primary">HiveMind</span>
      </div>
      <nav className="flex-1 py-2">
        {navItems.map((item) => {
          const isActive = pathname === item.href;
          const Icon = item.icon;
          return (
            <Link
              key={item.href}
              href={item.href}
              className={`flex items-center gap-2 px-4 py-1.5 text-[13px] ${
                isActive
                  ? "bg-bg-panel-hover text-text-primary"
                  : "text-text-secondary hover:text-text-primary hover:bg-bg-panel-hover"
              }`}
            >
              <Icon size={15} strokeWidth={1.5} />
              {item.label}
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
