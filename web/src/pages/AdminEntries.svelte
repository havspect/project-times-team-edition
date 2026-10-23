<script lang="ts">
  import { onMount } from 'svelte'
  import AdminNav from '../lib/AdminNav.svelte'
  import { api, problemMessage } from '../lib/api'
  import type { components } from '../lib/api/schema'
  import { currentWeek } from '../lib/format'
  import { session } from '../lib/session.svelte'

  type Entry = components['schemas']['EntryResponse']
  type Project = components['schemas']['ProjectResponse']

  const week = currentWeek(session.user?.timezone ?? 'UTC')

  let from = $state(week.from)
  let to = $state(week.to)
  let entries = $state<Entry[]>([])
  let projects = $state<Project[]>([])
  let loading = $state(true)
  let error = $state('')
  let tab = $state<'live' | 'deleted'>('live')
  let restoring = $state<number | null>(null)

  const live = $derived(entries.filter((entry) => !entry.deleted_at))
  const deleted = $derived(entries.filter((entry) => entry.deleted_at))
  const shown = $derived(tab === 'live' ? live : deleted)

  const dateTime = new Intl.DateTimeFormat(undefined, { dateStyle: 'short', timeStyle: 'short' })

  function at(value: string): string {
    return dateTime.format(new Date(value))
  }

  function projectName(id: number): string {
    return projects.find((project) => project.id === id)?.name ?? `project ${id}`
  }

  async function load() {
    error = ''
    const [entryRes, projectRes] = await Promise.all([
      api.GET('/api/v1/entries', {
        params: { query: { include_deleted: true, from, to } },
      }),
      api.GET('/api/v1/projects', { params: { query: { all: true } } }),
    ])
    if (entryRes.error) error = problemMessage(entryRes.error, entryRes.response)
    else entries = entryRes.data ?? []
    if (projectRes.error) error = problemMessage(projectRes.error, projectRes.response)
    else projects = projectRes.data ?? []
    loading = false
  }

  onMount(() => {
    void load()
  })

  async function restore(entry: Entry) {
    error = ''
    restoring = entry.id
    try {
      const { error: problem, response } = await api.POST('/api/v1/entries/{id}/restore', {
        params: { path: { id: entry.id } },
      })
      if (problem) throw new Error(problemMessage(problem, response))
      await load()
    } catch (err) {
      error = err instanceof Error ? err.message : String(err)
    } finally {
      restoring = null
    }
  }
</script>

<AdminNav />
<h1>Deleted entries</h1>

{#if error}
  <p class="error" role="alert">{error}</p>
{/if}

<form class="toolbar" onsubmit={(event) => { event.preventDefault(); void load() }}>
  <div class="field">
    <label for="entries-from">From</label>
    <input id="entries-from" type="date" bind:value={from} required />
  </div>
  <div class="field">
    <label for="entries-to">To</label>
    <input id="entries-to" type="date" bind:value={to} required />
  </div>
  <button class="primary" type="submit" disabled={loading}>
    {loading ? 'Loading…' : 'Load'}
  </button>
  <p class="hint">The end date is exclusive.</p>
</form>

<div class="tabs" role="group" aria-label="Entry state">
  <button type="button" aria-pressed={tab === 'live'} onclick={() => (tab = 'live')}>
    Live ({live.length})
  </button>
  <button type="button" aria-pressed={tab === 'deleted'} onclick={() => (tab = 'deleted')}>
    Deleted ({deleted.length})
  </button>
</div>

{#if loading}
  <p>Loading entries…</p>
{:else if shown.length === 0}
  <p>{tab === 'live' ? 'No live entries in this range.' : 'No deleted entries in this range.'}</p>
{:else}
  <table>
    <thead>
      <tr>
        <th scope="col">ID</th>
        <th scope="col">User</th>
        <th scope="col">Project</th>
        <th scope="col">Start</th>
        <th scope="col">End</th>
        <th scope="col">Note</th>
        {#if tab === 'deleted'}
          <th scope="col">Deleted</th>
          <th scope="col">Actions</th>
        {/if}
      </tr>
    </thead>
    <tbody>
      {#each shown as entry (entry.id)}
        <tr>
          <td>{entry.id}</td>
          <td>{entry.user_id}</td>
          <td>{projectName(entry.project_id)}</td>
          <td>{at(entry.start_at)}</td>
          <td>{entry.end_at ? at(entry.end_at) : 'running'}</td>
          <td>{entry.note || '—'}</td>
          {#if tab === 'deleted'}
            <td>{entry.deleted_at ? at(entry.deleted_at) : ''}</td>
            <td>
              <button
                class="ghost"
                type="button"
                onclick={() => restore(entry)}
                disabled={restoring === entry.id}
              >
                {restoring === entry.id ? 'Restoring…' : 'Restore'}
              </button>
            </td>
          {/if}
        </tr>
      {/each}
    </tbody>
  </table>
{/if}
