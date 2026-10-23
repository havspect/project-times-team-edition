// formatDuration renders a span of seconds the way timesheets read it, rounded
// to the minute: 27000 -> "7h 30m", 300 -> "5m".
export function formatDuration(seconds: number): string {
  const minutes = Math.round(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const rest = minutes % 60
  if (hours === 0) return `${rest}m`
  return `${hours}h ${rest}m`
}
