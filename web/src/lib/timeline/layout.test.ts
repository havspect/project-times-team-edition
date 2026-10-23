import { describe, expect, it } from 'vitest'
import { FINE_SNAP, SNAP, clampRange, lanes, snap, timeToX, xToTime } from './layout'

const MIN = 60_000
const HOUR = 60 * MIN
const DAY = 24 * HOUR
const NOW = Date.parse('2026-10-23T12:00:00Z')

describe('timeToX and xToTime', () => {
  it('round trips', () => {
    const origin = NOW
    const msPerPx = DAY / 200
    const x = timeToX(origin + 6 * HOUR, origin, msPerPx)
    expect(x).toBe(50)
    expect(xToTime(x, origin, msPerPx)).toBe(origin + 6 * HOUR)
  })
})

describe('snap', () => {
  it('rounds to the nearest quarter hour', () => {
    expect(snap(NOW + 8 * MIN, SNAP)).toBe(NOW + 15 * MIN)
    expect(snap(NOW + 7 * MIN, SNAP)).toBe(NOW)
    expect(snap(NOW + 22 * MIN, SNAP)).toBe(NOW + 15 * MIN)
  })

  it('rounds to the nearest minute when fine', () => {
    expect(snap(NOW + 29_000, FINE_SNAP)).toBe(NOW)
    expect(snap(NOW + 31_000, FINE_SNAP)).toBe(NOW + MIN)
  })

  it('can be switched off', () => {
    expect(snap(1234, SNAP, false)).toBe(1234)
  })
})

describe('clampRange', () => {
  it('ends no later than now plus the grace', () => {
    const end = NOW + 3 * HOUR
    const range = clampRange(end - HOUR, end, NOW)
    expect(range.end).toBe(NOW + 5 * MIN)
    expect(range.start).toBe(NOW + 5 * MIN - HOUR)
  })

  it('keeps a moved block its full duration', () => {
    const range = clampRange(NOW - 1 * HOUR, NOW + 4 * HOUR, NOW, { moving: true })
    expect(range.end).toBe(NOW + 5 * MIN)
    expect(range.end - range.start).toBe(5 * HOUR)
  })

  it('caps a too-long span at 24 hours', () => {
    const range = clampRange(NOW - 25 * HOUR, NOW, NOW)
    expect(range.start).toBe(NOW - 24 * HOUR)
    expect(range.end).toBe(NOW)
  })

  it('pushes the start to within 24 hours of the end', () => {
    const range = clampRange(NOW - 30 * HOUR, NOW - 2 * HOUR, NOW)
    expect(range.end).toBe(NOW - 2 * HOUR)
    expect(range.start).toBe(NOW - 26 * HOUR)
  })

  it('keeps at least one minute', () => {
    const point = clampRange(NOW, NOW, NOW)
    expect(point.end - point.start).toBe(MIN)

    const backwards = clampRange(NOW - 2 * HOUR, NOW - 3 * HOUR, NOW)
    expect(backwards.end - backwards.start).toBe(HOUR)
  })
})

describe('lanes', () => {
  it('puts non-overlapping spans in lane 0', () => {
    const map = lanes([
      { id: 1, start: NOW, end: NOW + HOUR },
      { id: 2, start: NOW + HOUR, end: NOW + 2 * HOUR },
    ])
    expect(map.get(1)).toBe(0)
    expect(map.get(2)).toBe(0)
  })

  it('stacks a three-way overlap', () => {
    const map = lanes([
      { id: 1, start: NOW, end: NOW + 3 * HOUR },
      { id: 2, start: NOW + HOUR, end: NOW + 2 * HOUR },
      { id: 3, start: NOW + 90 * MIN, end: NOW + 4 * HOUR },
    ])
    expect(map.get(1)).toBe(0)
    expect(map.get(2)).toBe(1)
    expect(map.get(3)).toBe(2)
  })

  it('reuses a lane once it is free, ordering by start then id', () => {
    const map = lanes([
      { id: 3, start: NOW + 2 * HOUR, end: NOW + 3 * HOUR },
      { id: 2, start: NOW, end: NOW + HOUR },
      { id: 1, start: NOW + HOUR, end: NOW + 2 * HOUR },
    ])
    expect(map.get(2)).toBe(0)
    expect(map.get(1)).toBe(0)
    expect(map.get(3)).toBe(0)
  })

  it('treats a running span as ending at now', () => {
    const running = { id: 1, start: NOW - HOUR, end: NOW }
    const later = { id: 2, start: NOW + MIN, end: NOW + HOUR }
    expect(lanes([running, later]).get(2)).toBe(0)
  })
})
