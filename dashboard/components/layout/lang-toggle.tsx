"use client";

import { useI18n } from "@/lib/i18n";

/** LangToggle - EN / VI, remembered across reloads. */
export function LangToggle() {
  const { lang, setLang } = useI18n();
  return (
    <div className="flex items-center rounded-md border border-border bg-bg-inset/50 overflow-hidden h-7">
      {(["en", "vi"] as const).map((l) => (
        <button
          key={l}
          onClick={() => setLang(l)}
          className={`px-2 h-full text-[11px] font-semibold uppercase transition-colors ${
            lang === l
              ? "bg-blue/15 text-blue"
              : "text-text-tertiary hover:text-text-primary"
          }`}
          title={l === "vi" ? "Tiếng Việt" : "English"}
        >
          {l}
        </button>
      ))}
    </div>
  );
}
