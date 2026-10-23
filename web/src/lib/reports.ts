// Report rows as the API returns them, narrowed to the fields the page reads.
export type ReportRow = {
  bucket: string
  user_id: number
  user: string
  project_id: number
  project: string
  archived: boolean
  seconds: number
}

export type ReportSection = { bucket: string; rows: ReportRow[]; seconds: number }

// sectionize groups rows by bucket, preserving the API's order, and sums each
// bucket. The API already sorts by bucket, so equal buckets arrive adjacent.
export function sectionize(rows: ReportRow[]): ReportSection[] {
  const sections: ReportSection[] = []
  for (const row of rows) {
    const last = sections[sections.length - 1]
    if (last && last.bucket === row.bucket) {
      last.rows.push(row)
      last.seconds += row.seconds
    } else {
      sections.push({ bucket: row.bucket, rows: [row], seconds: row.seconds })
    }
  }
  return sections
}

export function totalSeconds(rows: ReportRow[]): number {
  return rows.reduce((sum, row) => sum + row.seconds, 0)
}

export type CsvFilters = {
  from: string
  to: string
  teamID: string
  projectID: string
  userID: string
}

// csvHref is the download URL for the filters the table was loaded with; an
// empty id means "all", and the API ignores the bucket because CSV is per day.
export function csvHref({ from, to, teamID, projectID, userID }: CsvFilters): string {
  const query = new URLSearchParams({ from, to })
  if (teamID) query.set('team_id', teamID)
  if (projectID) query.set('project_id', projectID)
  if (userID) query.set('user_id', userID)
  return `/api/v1/reports.csv?${query}`
}
