import { describe, expect, it } from 'vitest'
import { currentWeek, formatDuration } from './format'

describe('formatDuration', () => {
  it('rounds to the minute', () => {
    expect(formatDuration(0)).toBe('0m')
    expect(formatDuration(29)).toBe('0m')
    expect(formatDuration(90)).toBe('2m')
  })

  it('splits hours and minutes', () => {
    expect(formatDuration(300)).toBe('5m')
    expect(formatDuration(3600)).toBe('1h 0m')
    expect(formatDuration(27000)).toBe('7h 30m')
  })
})

describe('currentWeek', () => {
  it('starts on Monday and ends the next Monday', () => {
    // Friday 23 October 2026.
    const week = currentWeek('UTC', new Date('2026-10-23T12:00:00Z'))
    expect(week).toEqual({ from: '2026-10-19', to: '2026-10-26' })
  })

  it('uses the given timezone, not the machine zone', () => {
    // Sunday 22:30 UTC is already Monday in Tokyo.
    const week = currentWeek('Asia/Tokyo', new Date('2026-10-25T22:30:00Z'))
    expect(week).toEqual({ from: '2026-10-26', to: '2026-11-02' })
  })
})
