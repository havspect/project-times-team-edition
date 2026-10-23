// Pixel geometry and the range rules the API will accept. The axis is linear in
// epoch milliseconds: a DST day is 23 or 25 hours wide, not a lie of 24.

export const SNAP = 15 * 60_000
export const FINE_SNAP = 60_000

const MIN = 60_000
const GRACE = 5 * 60_000
const MAX = 24 * 60 * 60_000

export function timeToX(ms: number, originMs: number, msPerPx: number): number {
  return (ms - originMs) / msPerPx
}

export function xToTime(x: number, originMs: number, msPerPx: number): number {
  return originMs + x * msPerPx
}

export function snap(ms: number, stepMs: number, enabled = true): number {
  return enabled ? Math.round(ms / stepMs) * stepMs : ms
}

// clampRange keeps a draft inside what internal/entry.Validate accepts: a
// positive length of at most 24 hours, ending no later than now plus the
// server's 5-minute grace. moving shifts the whole span so a dragged block
// keeps its duration instead of being squashed at "now".
export function clampRange(
  start: number,
  end: number,
  now: number,
  { moving = false }: { moving?: boolean } = {},
): { start: number; end: number } {
  const limit = now + GRACE
  if (moving) {
    const duration = Math.min(Math.max(end - start, MIN), MAX)
    const hi = Math.min(end, limit)
    return { start: hi - duration, end: hi }
  }
  let lo = Math.min(start, end)
  let hi = Math.max(start, end)
  if (lo > limit) {
    // A draft wholly in the future slides back whole instead of collapsing.
    const duration = Math.min(Math.max(hi - lo, MIN), MAX)
    return { start: limit - duration, end: limit }
  }
  hi = Math.min(hi, limit)
  lo = Math.max(lo, hi - MAX)
  if (hi - lo < MIN) {
    if (lo + MIN <= limit) {
      hi = lo + MIN
    } else {
      hi = limit
      lo = limit - MIN
    }
  }
  return { start: lo, end: hi }
}

// lanes puts each span in the first lane whose previous span has ended. Input
// order does not matter: start, then id, keeps the picture stable.
export function lanes<T extends { id: number; start: number; end: number }>(
  entries: T[],
): Map<number, number> {
  const sorted = [...entries].sort((a, b) => a.start - b.start || a.id - b.id)
  const lastEnd: number[] = []
  const out = new Map<number, number>()
  for (const entry of sorted) {
    let lane = lastEnd.findIndex((end) => end <= entry.start)
    if (lane === -1) {
      lane = lastEnd.length
      lastEnd.push(entry.end)
    } else {
      lastEnd[lane] = entry.end
    }
    out.set(entry.id, lane)
  }
  return out
}
