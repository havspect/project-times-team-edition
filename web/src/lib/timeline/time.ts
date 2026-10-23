// Wall-clock arithmetic in a user's IANA timezone. Days, headers and edit
// fields follow that zone, not the browser's; instants stay epoch milliseconds.

export type Parts = {
  year: number
  month: number
  day: number
  hour: number
  minute: number
  second: number
}

const formatters = new Map<string, Intl.DateTimeFormat>()

function formatter(tz: string): Intl.DateTimeFormat {
  let format = formatters.get(tz)
  if (!format) {
    format = new Intl.DateTimeFormat('en-CA', {
      timeZone: tz,
      hourCycle: 'h23',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
    formatters.set(tz, format)
  }
  return format
}

export function zonedParts(ms: number, tz: string): Parts {
  const parts: Record<string, number> = {}
  for (const part of formatter(tz).formatToParts(new Date(ms))) {
    if (part.type !== 'literal') parts[part.type] = Number(part.value)
  }
  return {
    year: parts.year ?? 0,
    month: parts.month ?? 1,
    day: parts.day ?? 1,
    hour: parts.hour ?? 0,
    minute: parts.minute ?? 0,
    second: parts.second ?? 0,
  }
}

function pad(value: number): string {
  return String(value).padStart(2, '0')
}

// offsetAt is the zone's offset in ms at an instant, including DST.
function offsetAt(ms: number, tz: string): number {
  const parts = zonedParts(ms, tz)
  const wall = Date.UTC(parts.year, parts.month - 1, parts.day, parts.hour, parts.minute, parts.second)
  return wall - Math.floor(ms / 1000) * 1000
}

// wallTimeToInstant maps a 'YYYY-MM-DDTHH:mm' wall time in tz to an instant.
// The offset is measured twice, because the first guess can sit on the other
// side of a DST change. A skipped spring-forward time resolves forward.
export function wallTimeToInstant(tz: string, wall: string): number {
  const [date, clock = '00:00'] = wall.split('T')
  const [year, month, day] = date.split('-').map(Number)
  const [hour, minute] = clock.split(':').map(Number)
  const guess = Date.UTC(year, month - 1, day, hour, minute)
  const first = offsetAt(guess, tz)
  const instant = guess - first
  const second = offsetAt(instant, tz)
  return second === first ? instant : guess - second
}

// instantToWallTime is the value <input type="datetime-local"> wants.
export function instantToWallTime(tz: string, ms: number): string {
  const parts = zonedParts(ms, tz)
  return `${parts.year}-${pad(parts.month)}-${pad(parts.day)}T${pad(parts.hour)}:${pad(parts.minute)}`
}

export function localDate(tz: string, ms: number): string {
  const parts = zonedParts(ms, tz)
  return `${parts.year}-${pad(parts.month)}-${pad(parts.day)}`
}

export function startOfDay(tz: string, ms: number): number {
  return wallTimeToInstant(tz, `${localDate(tz, ms)}T00:00`)
}

// addDays is calendar arithmetic on a plain date, immune to DST.
export function addDays(dateISO: string, days: number): string {
  const date = new Date(`${dateISO}T00:00:00Z`)
  date.setUTCDate(date.getUTCDate() + days)
  return date.toISOString().slice(0, 10)
}
