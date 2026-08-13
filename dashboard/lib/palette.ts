/**
 * palette.ts - the one place chart series colors live.
 * Mid-tone pastels chosen to stay legible on BOTH the light and dark themes
 * (recharts renders SVG attributes, which cannot resolve CSS variables, so
 * series colors are concrete hexes; structural chart chrome uses alpha grays).
 */
export const PASTEL = {
  blue: "#7d9bf0",
  green: "#57bd8d",
  yellow: "#dfae53",
  red: "#ec7d90",
  purple: "#a98ee6",
  orange: "#eda06b",
  teal: "#56c4ba",
  gray: "#9298ad",
} as const;

/** Structural chart chrome that works on any background. */
export const CHART_CHROME = {
  grid: "rgba(128, 134, 160, 0.18)",
  axis: "#8d92aa",
  cursor: "rgba(128, 134, 160, 0.35)",
  empty: "rgba(128, 134, 160, 0.25)",
} as const;
