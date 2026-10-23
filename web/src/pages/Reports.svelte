<script lang="ts">
  import { onMount } from 'svelte'
  import { api, problemMessage } from '../lib/api'
  import type { components } from '../lib/api/schema'
  import { currentWeek, formatDuration } from '../lib/format'
  import { csvHref, sectionize, totalSeconds, type ReportRow } from '../lib/reports'
  import { session } from '../lib/session.svelte'

  type Project = components['schemas']['ProjectResponse']
  type Team = components['schemas']['TeamResponse']
  type Filters = {
    from: string
    to: string
    teamID: string
    projectID: string
    by: 'week' | 'day'
  }

  const user = session.user
  const isAdmin = user?.is_admin ?? false
  const zone = user?.timezone ?? 'UTC'
  const week = currentWeek(zone)

  let from = $state(week.from)
  let to = $state(week.to)
  let teamID = $state('')
  let projectID = $state('')
  let personID = $state('')
  let by = $state<'week' | 'day'>('week')
  // applied is what the table and the CSV link were built from, so editing a
  // filter without pressing Apply cannot change the download.
  let applied = $state<Filters>({
    from: week.from,
    to: week.to,
    teamID: '',
    projectID: '',
    by: 'week',
  })

  let rows = $state<ReportRow[]>([])
  let projects = $state<Project[]>([])
  let adminTeams = $state<Team[]>([])
  let loading = $state(true)
  let error = $state('')

  const teams = $derived(
    isAdmin
      ? adminTeams.map((team) => ({
          id: team.id,
          name: team.name,
          archived: team.archived_at !== null,
        }))
      : (user?.memberships ?? [])
          .filter((member) => member.role === 'lead')
          .map((member) => ({ id: member.team_id, name: member.team, archived: false })),
  )

  // People come from the loaded rows: /users is admin-only, and someone with no
  // hours in the range has nothing to show anyway.
  const people = $derived.by(() => {
    const seen = new Map<number, string>()
    for (const row of rows) if (!seen.has(row.user_id)) seen.set(row.user_id, row.user)
    return Array.from(seen, ([id, name]) => ({ id, name }))
  })

  const shown = $derived(personID ? rows.filter((row) => row.user_id === Number(personID)) : rows)
  const sections = $derived(sectionize(shown))
  const total = $derived(totalSeconds(shown))
  const csv = $derived(
    csvHref({
      from: applied.from,
      to: applied.to,
      teamID: applied.teamID,
      projectID: applied.projectID,
      userID: personID,
    }),
  )
  const filename = $derived(`reports-${applied.from}-to-${applied.to}.csv`)

  function rangeError(start: string, end: string): string {
    const f = Date.parse(`${start}T00:00:00Z`)
    const t = Date.parse(`${end}T00:00:00Z`)
    if (Number.isNaN(f) || Number.isNaN(t)) return 'Choose a valid start and end date.'
    if (t <= f) return 'The end date must be after the start date; the end is exclusive.'
    if (t - f > 366 * 86_400_000) return 'Choose a range of at most 366 days.'
    return ''
  }

  async function loadTeams() {
    const { data, error: problem, response } = await api.GET('/api/v1/teams')
    if (problem) error = problemMessage(problem, response)
    else adminTeams = data ?? []
  }

  async function load() {
    error = ''
    const invalid = rangeError(from, to)
    if (invalid) {
      error = invalid
      return
    }
    personID = ''
    applied = { from, to, teamID, projectID, by }
    loading = true

    const query: {
      from: string
      to: string
      by: 'week' | 'day'
      team_id?: number
      project_id?: number
    } = { from: applied.from, to: applied.to, by: applied.by }
    if (applied.teamID) query.team_id = Number(applied.teamID)
    if (applied.projectID) query.project_id = Number(applied.projectID)

    const [reportRes, projectRes] = await Promise.all([
      api.GET('/api/v1/reports', { params: { query } }),
      api.GET('/api/v1/projects', { params: { query: isAdmin ? { all: true } : {} } }),
    ])

    if (reportRes.error) {
      error = problemMessage(reportRes.error, reportRes.response)
      rows = []
    } else {
      rows = reportRes.data?.rows ?? []
    }
    if (projectRes.error) error = problemMessage(projectRes.error, projectRes.response)
    else projects = projectRes.data ?? []
    loading = false
  }

  onMount(() => {
    void load()
    if (isAdmin) void loadTeams()
  })
</script>

<h1>Reports</h1>

{#if error}
  <p class="error" role="alert">{error}</p>
{/if}

<form class="toolbar" onsubmit={(event) => { event.preventDefault(); void load() }}>
  <div class="field">
    <label for="report-from">From</label>
    <input id="report-from" type="date" bind:value={from} required />
  </div>
  <div class="field">
    <label for="report-to">To</label>
    <input id="report-to" type="date" bind:value={to} required />
  </div>
  <div class="field">
    <label for="report-team">Team</label>
    <select id="report-team" bind:value={teamID}>
      <option value="">All teams</option>
      {#each teams as team (team.id)}
        <option value={String(team.id)}>{team.name}{team.archived ? ' (archived)' : ''}</option>
      {/each}
    </select>
  </div>
  <div class="field">
    <label for="report-project">Project</label>
    <select id="report-project" bind:value={projectID}>
      <option value="">All projects</option>
      {#each projects as project (project.id)}
        <option value={String(project.id)}>
          {project.name}{project.archived_at ? ' (archived)' : ''}
        </option>
      {/each}
    </select>
  </div>
  <div class="field">
    <label for="report-person">Person</label>
    <select id="report-person" bind:value={personID}>
      <option value="">Everyone</option>
      {#each people as person (person.id)}
        <option value={String(person.id)}>{person.name}</option>
      {/each}
    </select>
  </div>
  <div class="tabs" role="group" aria-label="Group by">
    <button type="button" aria-pressed={by === 'week'} onclick={() => (by = 'week')}>Week</button>
    <button type="button" aria-pressed={by === 'day'} onclick={() => (by = 'day')}>Day</button>
  </div>
  <button class="primary" type="submit" disabled={loading}>
    {loading ? 'Loading…' : 'Apply'}
  </button>
  <p class="hint">
    The end date is exclusive. Hours are bucketed in each person's timezone; your week ({zone})
    seeds the range.
  </p>
</form>

{#if loading}
  <p>Loading report…</p>
{:else if shown.length === 0}
  <p>No hours in this range.</p>
{:else}
  <p><a href={csv} download={filename}>Download CSV</a></p>

  {#each sections as section (section.bucket)}
    <h2>{applied.by === 'week' ? 'Week' : 'Day'} {section.bucket}</h2>
    <table>
      <thead>
        <tr>
          <th scope="col">Person</th>
          <th scope="col">Project</th>
          <th scope="col">Hours</th>
        </tr>
      </thead>
      <tbody>
        {#each section.rows as row (`${row.user_id}-${row.project_id}`)}
          <tr>
            <td>{row.user}</td>
            <td>
              {row.project}
              {#if row.archived}<span class="badge">archived</span>{/if}
            </td>
            <td>{formatDuration(row.seconds)}</td>
          </tr>
        {/each}
      </tbody>
      <tfoot>
        <tr>
          <th scope="row" colspan="2">Subtotal</th>
          <td>{formatDuration(section.seconds)}</td>
        </tr>
      </tfoot>
    </table>
  {/each}

  <p>Total: <strong>{formatDuration(total)}</strong></p>
{/if}
