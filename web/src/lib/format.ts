// formatDuration renders a span of seconds the way timesheets read it, rounded
// to the minute: 27000 -> "7h 30m", 300 -> "5m".
export function formatDuration(seconds: number): string {
  const minutes = Math.round(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const rest = minutes % 60
  if (hours === 0) return `${rest}m`
  return `${hours}h ${rest}m`
}

// currentWeek returns this week's ISO dates in timezone: from is Monday, to is
// the next Monday (the API's exclusive end). The "now" parameter is for tests.
export function currentWeek(timezone: string, now: Date = new Date()): { from: string; to: string } {
  const today = new Intl.DateTimeFormat('en-CA', {
    timeZone: timezone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(now)
  // Parse as UTC midnight so weekday and month arithmetic cannot drift with DST.
  const monday = new Date(`${today}T00:00:00Z`)
  monday.setUTCDate(monday.getUTCDate() - ((monday.getUTCDay() + 6) % 7))
  const next = new Date(monday)
  next.setUTCDate(next.getUTCDate() + 7)
  return { from: monday.toISOString().slice(0, 10), to: next.toISOString().slice(0, 10) }
}
