<script lang="ts">
  import { onMount } from 'svelte'
  import AdminNav from '../lib/AdminNav.svelte'
  import { api, problemMessage } from '../lib/api'
  import type { components } from '../lib/api/schema'

  type Project = components['schemas']['ProjectResponse']
  type Team = components['schemas']['TeamResponse']

  const defaultColor = '#4f46e5'

  let projects = $state<Project[]>([])
  let teams = $state<Team[]>([])
  let loading = $state(true)
  let error = $state('')

  let name = $state('')
  let teamID = $state('')
  let color = $state(defaultColor)
  let createPending = $state(false)

  let editing = $state<number | null>(null)
  let editName = $state('')
  let editColor = $state(defaultColor)
  let editTeam = $state('')
  let savePending = $state(false)
  let archiving = $state<number | null>(null)
  let restoring = $state<number | null>(null)

  const activeTeams = $derived(teams.filter((team) => team.archived_at === null))

  function teamName(id: number): string {
    return teams.find((team) => team.id === id)?.name ?? `team ${id}`
  }

  function teamArchived(id: number): boolean {
    return teams.find((team) => team.id === id)?.archived_at !== null
  }

  async function load() {
    const [projectRes, teamRes] = await Promise.all([
      api.GET('/api/v1/projects', { params: { query: { all: true } } }),
      api.GET('/api/v1/teams'),
    ])
    if (projectRes.error) error = problemMessage(projectRes.error, projectRes.response)
    else projects = projectRes.data ?? []
    if (teamRes.error) error = problemMessage(teamRes.error, teamRes.response)
    else teams = teamRes.data ?? []
    loading = false
  }

  onMount(() => {
    void load()
  })

  async function create(event: SubmitEvent) {
    event.preventDefault()
    error = ''
    createPending = true
    try {
      const { error: problem, response } = await api.POST('/api/v1/projects', {
        body: { name, team_id: Number(teamID), color },
      })
      if (problem) throw new Error(problemMessage(problem, response))
      name = ''
      color = defaultColor
      await load()
    } catch (err) {
      error = err instanceof Error ? err.message : String(err)
    } finally {
      createPending = false
    }
  }

  function startEdit(project: Project) {
    if (editing === project.id) {
      editing = null
      return
    }
    editing = project.id
    editName = project.name
    editColor = project.color || defaultColor
    editTeam = String(project.team_id)
  }

  async function save(event: SubmitEvent, project: Project) {
    event.preventDefault()
    error = ''
    savePending = true
    try {
      const { error: problem, response } = await api.PATCH('/api/v1/projects/{id}', {
        params: { path: { id: project.id } },
        body: { name: editName, color: editColor, team_id: Number(editTeam) },
      })
      if (problem) throw new Error(problemMessage(problem, response))
      editing = null
      await load()
    } catch (err) {
      error = err instanceof Error ? err.message : String(err)
    } finally {
      savePending = false
    }
  }

  async function archive(project: Project) {
    if (!confirm(`Archive ${project.name}? Its hours stay in reports, and you can restore it later.`)) return
    error = ''
    archiving = project.id
    try {
      const { error: problem, response } = await api.PATCH('/api/v1/projects/{id}', {
        params: { path: { id: project.id } },
        body: { archived: true },
      })
      if (problem) throw new Error(problemMessage(problem, response))
      await load()
    } catch (err) {
      error = err instanceof Error ? err.message : String(err)
    } finally {
      archiving = null
    }
  }

  async function restore(project: Project) {
    error = ''
    restoring = project.id
    try {
      const { error: problem, response } = await api.POST('/api/v1/projects/{id}/restore', {
        params: { path: { id: project.id } },
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
<h1>Projects</h1>

{#if error}
  <p class="error" role="alert">{error}</p>
{/if}

<form class="toolbar" onsubmit={create}>
  <div class="field">
    <label for="new-project">Name</label>
    <input id="new-project" bind:value={name} required />
  </div>
  <div class="field">
    <label for="new-project-team">Team</label>
    <select id="new-project-team" bind:value={teamID} required>
      <option value="" disabled>Choose a team</option>
      {#each activeTeams as team (team.id)}
        <option value={String(team.id)}>{team.name}</option>
      {/each}
    </select>
  </div>
  <div class="field">
    <label for="new-project-color">Color</label>
    <input id="new-project-color" class="color" type="color" bind:value={color} />
  </div>
  <button class="primary" type="submit" disabled={createPending || activeTeams.length === 0}>
    {createPending ? 'Creating…' : 'Create project'}
  </button>
  {#if activeTeams.length === 0}
    <p class="hint">Create an active team first.</p>
  {/if}
</form>

{#if loading}
  <p>Loading projects…</p>
{:else if projects.length === 0}
  <p>No projects yet.</p>
{:else}
  <table>
    <thead>
      <tr>
        <th scope="col">Name</th>
        <th scope="col">Team</th>
        <th scope="col">Status</th>
        <th scope="col">Actions</th>
      </tr>
    </thead>
    <tbody>
      {#each projects as project (project.id)}
        <tr>
          <td>
            <span class="swatch" style={`background: ${project.color || 'transparent'}`} aria-hidden="true"
            ></span>
            {project.name}
          </td>
          <td>{teamName(project.team_id)}</td>
          <td>
            {#if project.archived_at}
              <span class="badge alert">archived</span>
            {/if}
          </td>
          <td>
            <div class="row-actions">
              <button class="ghost" type="button" onclick={() => startEdit(project)}>
                {editing === project.id ? 'Close' : 'Edit'}
              </button>
              {#if project.archived_at}
                <button
                  class="ghost"
                  type="button"
                  onclick={() => restore(project)}
                  disabled={restoring === project.id || teamArchived(project.team_id)}
                >
                  {restoring === project.id ? 'Restoring…' : 'Restore'}
                </button>
                {#if teamArchived(project.team_id)}
                  <span class="hint">Team archived</span>
                {/if}
              {:else}
                <button
                  class="ghost"
                  type="button"
                  onclick={() => archive(project)}
                  disabled={archiving === project.id}
                >
                  {archiving === project.id ? 'Archiving…' : 'Archive'}
                </button>
              {/if}
            </div>
          </td>
        </tr>
        {#if editing === project.id}
          <tr>
            <td colspan="4">
              <form class="toolbar" onsubmit={(event) => save(event, project)}>
                <div class="field">
                  <label for={`project-name-${project.id}`}>Name</label>
                  <input id={`project-name-${project.id}`} bind:value={editName} required />
                </div>
                <div class="field">
                  <label for={`project-team-${project.id}`}>Team</label>
                  <select id={`project-team-${project.id}`} bind:value={editTeam}>
                    {#each teams as team (team.id)}
                      <option value={String(team.id)}>
                        {team.name}{team.archived_at ? ' (archived)' : ''}
                      </option>
                    {/each}
                  </select>
                  <span class="hint">A project can only change team while it has no entries.</span>
                </div>
                <div class="field">
                  <label for={`project-color-${project.id}`}>Color</label>
                  <input
                    id={`project-color-${project.id}`}
                    class="color"
                    type="color"
                    bind:value={editColor}
                  />
                </div>
                <button class="primary" type="submit" disabled={savePending}>
                  {savePending ? 'Saving…' : 'Save'}
                </button>
                <button class="ghost" type="button" onclick={() => (editing = null)}>Cancel</button>
              </form>
            </td>
          </tr>
        {/if}
      {/each}
    </tbody>
  </table>
{/if}
