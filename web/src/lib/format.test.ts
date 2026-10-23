import { describe, expect, it } from 'vitest'
import { formatDuration } from './format'

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
