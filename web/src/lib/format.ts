const rtf = new Intl.RelativeTimeFormat("en", { numeric: "auto" });

/**
 * Human-readable relative time — "3 hours ago", "in 2 days".
 * Accepts ISO string or Date; null/undefined returns "never".
 */
export function relativeTime(input: string | Date | null | undefined): string {
  if (!input) return "never";
  const date = typeof input === "string" ? new Date(input) : input;
  const diffMs = date.getTime() - Date.now();
  const absSec = Math.abs(diffMs) / 1000;

  const units: Array<[Intl.RelativeTimeFormatUnit, number]> = [
    ["year", 365 * 24 * 60 * 60],
    ["month", 30 * 24 * 60 * 60],
    ["week", 7 * 24 * 60 * 60],
    ["day", 24 * 60 * 60],
    ["hour", 60 * 60],
    ["minute", 60],
  ];
  for (const [unit, seconds] of units) {
    if (absSec >= seconds) {
      return rtf.format(Math.round(diffMs / (seconds * 1000)), unit);
    }
  }
  return "just now";
}

const dateFmt = new Intl.DateTimeFormat("en-CA", {
  year: "numeric",
  month: "short",
  day: "numeric",
});

/**
 * YYYY-MMM-DD for article pub dates. null/undefined returns "—".
 */
export function formatDate(input: string | Date | null | undefined): string {
  if (!input) return "—";
  const date = typeof input === "string" ? new Date(input) : input;
  if (Number.isNaN(date.getTime())) return "—";
  return dateFmt.format(date);
}

/**
 * Human-readable seconds interval: 21600 → "6h", 3600 → "1h", 600 → "10m".
 */
export function formatInterval(seconds: number): string {
  if (seconds % 3600 === 0) return `${seconds / 3600}h`;
  if (seconds % 60 === 0) return `${seconds / 60}m`;
  return `${seconds}s`;
}
