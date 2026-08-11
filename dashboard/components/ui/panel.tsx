import { ReactNode } from "react";

/**
 * Panel - the base surface for every dashboard block. A bordered card with an
 * optional titled header and an optional right-aligned action slot.
 */
export function Panel({
  title,
  subtitle,
  actions,
  children,
  className = "",
  bodyClassName = "",
}: {
  title?: string;
  subtitle?: string;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  bodyClassName?: string;
}) {
  return (
    <section
      className={`border border-border rounded-lg bg-bg-panel/70 overflow-hidden ${className}`}
    >
      {title && (
        <header className="flex items-center justify-between gap-3 px-4 h-12 border-b border-border bg-bg-inset/30">
          <div className="flex items-baseline gap-2.5 min-w-0">
            <h2 className="text-[13px] font-bold uppercase tracking-wide text-text-primary truncate">
              {title}
            </h2>
            {subtitle && (
              <span className="text-[12px] text-text-tertiary truncate">{subtitle}</span>
            )}
          </div>
          {actions}
        </header>
      )}
      <div className={`p-4 ${bodyClassName}`}>{children}</div>
    </section>
  );
}
