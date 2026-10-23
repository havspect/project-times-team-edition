import { describe, expect, it } from 'vitest'
import { addDays, instantToWallTime, localDate, startOfDay, wallTimeToInstant, zonedParts } from './time'

const HOUR = 3_600_000

describe('zonedParts', () => {
  it('reads the wall clock in the zone, not the machine zone', () => {
    // Berlin is still on CET at 00:30Z and on CEST at 01:30Z on 29 March 2026.
    expect(zonedParts(Date.parse('2026-03-29T00:30:00Z'), 'Europe/Berlin')).toMatchObject({
      year: 2026,
      month: 3,
      day: 29,
      hour: 1,
      minute: 30,
    })
    expect(zonedParts(Date.parse('2026-03-29T01:30:00Z'), 'Europe/Berlin').hour).toBe(3)
  })
})

describe('wallTimeToInstant', () => {
  it('handles a spring-forward day', () => {
    // 02:00 CET jumps to 03:00 CEST; 00:30 and 03:30 are real.
    expect(wallTimeToInstant('Europe/Berlin', '2026-03-29T00:30')).toBe(
      Date.parse('2026-03-28T23:30:00Z'),
    )
    expect(wallTimeToInstant('Europe/Berlin', '2026-03-29T03:30')).toBe(
      Date.parse('2026-03-29T01:30:00Z'),
    )
    // The skipped 02:30 resolves forward to 03:30 local.
    expect(instantToWallTime('Europe/Berlin', wallTimeToInstant('Europe/Berlin', '2026-03-29T02:30'))).toBe(
      '2026-03-29T03:30',
    )
  })

  it('handles a fall-back day', () => {
    expect(wallTimeToInstant('Europe/Berlin', '2026-10-25T01:30')).toBe(
      Date.parse('2026-10-24T23:30:00Z'),
    )
    // The second 02:30 is CET (+01:00).
    const second = wallTimeToInstant('Europe/Berlin', '2026-10-25T02:30')
    expect(second).toBe(Date.parse('2026-10-25T01:30:00Z'))
    expect(instantToWallTime('Europe/Berlin', second)).toBe('2026-10-25T02:30')
  })

  it('is UTC when the zone is UTC', () => {
    expect(wallTimeToInstant('UTC', '2026-10-23T09:15')).toBe(Date.parse('2026-10-23T09:15:00Z'))
  })

  it('round trips through instantToWallTime', () => {
    for (const wall of [
      '2026-01-15T00:00',
      '2026-03-29T03:30',
      '2026-06-01T23:59',
      '2026-10-25T01:30',
      '2026-12-31T12:00',
    ]) {
      expect(instantToWallTime('Europe/Berlin', wallTimeToInstant('Europe/Berlin', wall))).toBe(wall)
    }
  })
})

describe('localDate', () => {
  it('uses the user timezone at a midnight boundary', () => {
    const at = Date.parse('2026-10-23T22:30:00Z')
    expect(localDate('UTC', at)).toBe('2026-10-23')
    expect(localDate('Asia/Tokyo', at)).toBe('2026-10-24')
    expect(localDate('America/Los_Angeles', at)).toBe('2026-10-23')
  })
})

describe('startOfDay', () => {
  it('is local midnight', () => {
    expect(instantToWallTime('Europe/Berlin', startOfDay('Europe/Berlin', Date.parse('2026-10-23T12:00:00Z')))).toBe(
      '2026-10-23T00:00',
    )
  })

  it('makes DST days 23 and 25 hours wide', () => {
    const spring = startOfDay('Europe/Berlin', Date.parse('2026-03-29T12:00:00Z'))
    const springNext = startOfDay('Europe/Berlin', Date.parse('2026-03-30T12:00:00Z'))
    expect(springNext - spring).toBe(23 * HOUR)

    const fall = startOfDay('Europe/Berlin', Date.parse('2026-10-25T12:00:00Z'))
    const fallNext = startOfDay('Europe/Berlin', Date.parse('2026-10-26T12:00:00Z'))
    expect(fallNext - fall).toBe(25 * HOUR)
  })
})

describe('addDays', () => {
  it('crosses month and year boundaries', () => {
    expect(addDays('2026-10-25', 7)).toBe('2026-11-01')
    expect(addDays('2026-12-31', 1)).toBe('2027-01-01')
    expect(addDays('2026-03-01', -1)).toBe('2026-02-28')
  })
})
