/**
 * Format an ISO date string to a human-readable locale string.
 */
export function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString("pt-BR", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

/**
 * Format an ISO date string to include time.
 */
export function formatDateTime(iso: string): string {
  return new Date(iso).toLocaleString("pt-BR", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/**
 * Format score display: "13 — 6"
 */
export function formatScore(scoreA?: number, scoreB?: number): string | null {
  if (scoreA == null || scoreB == null) return null;
  return `${scoreA} — ${scoreB}`;
}

/**
 * Truncate a string to a max length, adding ellipsis.
 */
export function truncate(str: string, max: number): string {
  if (str.length <= max) return str;
  return str.slice(0, max) + "…";
}
