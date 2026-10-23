import { describe, expect, it } from 'vitest'
import { csvHref, sectionize, totalSeconds } from './reports'
import type { ReportRow } from './reports'

// row builds a report row; user 1 is Ada, user 2 is Grace.
function row(bucket: string, userID: number, projectID: number, seconds: number): ReportRow {
  return {
    bucket,
    user_id: userID,
    user: userID === 1 ? 'Ada' : 'Grace',
    project_id: projectID,
    project: projectID === 1 ? 'Site' : 'App',
    archived: false,
    seconds,
  }
}

describe('sectionize', () => {
  it('groups rows by bucket in the order the API sent them', () => {
    const sections = sectionize([
      row('2026-W43', 1, 1, 3600),
      row('2026-W43', 1, 2, 1800),
      row('2026-W44', 2, 1, 60),
    ])

    expect(sections.map((section) => section.bucket)).toEqual(['2026-W43', '2026-W44'])
    expect(sections[0].rows.map((r) => r.project_id)).toEqual([1, 2])
    expect(sections[1].rows).toHaveLength(1)
  })

  it('sums a subtotal per section', () => {
    const [week] = sectionize([
      row('2026-W43', 1, 1, 3600),
      row('2026-W43', 1, 2, 1800),
      row('2026-W43', 2, 1, 900),
    ])

    expect(week.seconds).toBe(6300)
  })

  it('returns nothing for no rows', () => {
    expect(sectionize([])).toEqual([])
  })
})

describe('totalSeconds', () => {
  it('sums every row', () => {
    const rows = [row('2026-W43', 1, 1, 3600), row('2026-W43', 1, 2, 1800), row('2026-W44', 2, 1, 60)]

    expect(totalSeconds(rows)).toBe(5460)
  })

  it('is zero for no rows', () => {
    expect(totalSeconds([])).toBe(0)
  })
})

describe('csvHref', () => {
  it('always sends the range and only the chosen filters', () => {
    expect(
      csvHref({ from: '2026-10-19', to: '2026-10-26', teamID: '', projectID: '', userID: '' }),
    ).toBe('/api/v1/reports.csv?from=2026-10-19&to=2026-10-26')

    const href = csvHref({
      from: '2026-10-19',
      to: '2026-10-26',
      teamID: '2',
      projectID: '7',
      userID: '5',
    })
    const query = new URLSearchParams(href.split('?')[1])

    expect(query.get('team_id')).toBe('2')
    expect(query.get('project_id')).toBe('7')
    expect(query.get('user_id')).toBe('5')
  })
})
