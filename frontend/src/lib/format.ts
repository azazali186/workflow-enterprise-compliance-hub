/** Formatting helpers — safe against null/undefined/invalid values. */

const dateFmt = new Intl.DateTimeFormat("en-US", {
  month: "short",
  day: "numeric",
  year: "numeric",
});

const dateTimeFmt = new Intl.DateTimeFormat("en-US", {
  month: "short",
  day: "numeric",
  year: "numeric",
  hour: "numeric",
  minute: "2-digit",
});

const timeFmt = new Intl.DateTimeFormat("en-US", {
  hour: "numeric",
  minute: "2-digit",
});

/** "Aug 8, 2026" — or "—" when missing/invalid. */
export function formatDate(iso?: string | null): string {
  const d = parse(iso);
  return d ? dateFmt.format(d) : "—";
}

/** "Aug 8, 2026, 3:42 PM" — or "—" when missing/invalid. */
export function formatDateTime(iso?: string | null): string {
  const d = parse(iso);
  return d ? dateTimeFmt.format(d) : "—";
}

/** "3:42 PM" for today, "Aug 8" otherwise. */
export function smartDateTime(iso?: string | null): string {
  const d = parse(iso);
  if (!d) return "—";
  const now = new Date();
  const sameDay = d.toDateString() === now.toDateString();
  return sameDay ? timeFmt.format(d) : dateFmt.format(d);
}

/** "2h ago", "3d ago", "just now" — from the backend's RFC3339 timestamps. */
export function relativeTime(iso?: string | null): string {
  const d = parse(iso);
  if (!d) return "—";
  const diff = Date.now() - d.getTime();
  const sec = Math.round(diff / 1000);
  if (sec < 45) return "just now";
  const min = Math.round(sec / 60);
  if (min < 60) return `${min}m ago`;
  const hr = Math.round(min / 60);
  if (hr < 24) return `${hr}h ago`;
  const day = Math.round(hr / 24);
  if (day < 30) return `${day}d ago`;
  return dateFmt.format(d);
}

/** Compact id for tables: "3f9a…c2b1". */
export function shortId(id?: string | null): string {
  if (!id || id.length < 12) return id ?? "—";
  return `${id.slice(0, 4)}…${id.slice(-4)}`;
}

function parse(iso?: string | null): Date | null {
  if (!iso) return null;
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? null : d;
}
