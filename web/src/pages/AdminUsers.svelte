<script lang="ts">
  import { onMount } from 'svelte'
  import AdminNav from '../lib/AdminNav.svelte'
  import { api, problemMessage } from '../lib/api'
  import type { components } from '../lib/api/schema'

  type User = components['schemas']['UserResponse']
  type Notice = { who: string; password: string }

  let users = $state<User[]>([])
  let loading = $state(true)
  let error = $state('')
  let notice = $state<Notice | null>(null)
  let copied = $state(false)

  let email = $state('')
  let name = $state('')
  let timezone = $state('UTC')
  let isAdmin = $state(false)
  let createPending = $state(false)

  let editing = $state<number | null>(null)
  let editName = $state('')
  let editTimezone = $state('')
  let editAdmin = $state(false)
  let editActive = $state(false)
  let savePending = $state(false)
  let resetting = $state<number | null>(null)

  async function load() {
    const { data, error: problem, response } = await api.GET('/api/v1/users')
    if (problem) error = problemMessage(problem, response)
    else users = data ?? []
    loading = false
  }

  onMount(() => {
    void load()
  })

  async function create(event: SubmitEvent) {
    event.preventDefault()
    error = ''
    copied = false
    createPending = true
    try {
      const { data, error: problem, response } = await api.POST('/api/v1/users', {
        body: { email, name, timezone, is_admin: isAdmin },
      })
      if (problem) throw new Error(problemMessage(problem, response))
      if (data) notice = { who: email, password: data.initial_password }
      email = ''
      name = ''
      timezone = 'UTC'
      isAdmin = false
      await load()
    } catch (err) {
      error = err instanceof Error ? err.message : String(err)
    } finally {
      createPending = false
    }
  }

  function startEdit(user: User) {
    if (editing === user.id) {
      editing = null
      return
    }
    editing = user.id
    editName = user.name
    editTimezone = user.timezone
    editAdmin = user.is_admin
    editActive = user.active
  }

  async function save(event: SubmitEvent, user: User) {
    event.preventDefault()
    error = ''
    savePending = true
    try {
      const { error: problem, response } = await api.PATCH('/api/v1/users/{id}', {
        params: { path: { id: user.id } },
        body: { name: editName, timezone: editTimezone, is_admin: editAdmin, active: editActive },
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

  async function reset(user: User) {
    error = ''
    copied = false
    resetting = user.id
    try {
      const { data, error: problem, response } = await api.POST('/api/v1/users/{id}/password', {
        params: { path: { id: user.id } },
      })
      if (problem) throw new Error(problemMessage(problem, response))
      if (data) notice = { who: user.email, password: data.initial_password }
      await load()
    } catch (err) {
      error = err instanceof Error ? err.message : String(err)
    } finally {
      resetting = null
    }
  }

  async function copy() {
    if (!notice) return
    try {
      await navigator.clipboard.writeText(notice.password)
      copied = true
    } catch {
      error = 'Could not copy; select the password and copy it by hand.'
    }
  }
</script>

<AdminNav />
<h1>Users</h1>

{#if error}
  <p class="error" role="alert">{error}</p>
{/if}

{#if notice}
  <div class="success one-time" role="status">
    <span>
      Password for <strong>{notice.who}</strong>, shown once:
      <code class="secret">{notice.password}</code>
    </span>
    <span class="hint">A new or reset password ends that user's sessions and API tokens.</span>
    <button class="ghost" type="button" onclick={copy}>{copied ? 'Copied' : 'Copy'}</button>
    <button
      class="ghost"
      type="button"
      onclick={() => {
        notice = null
        copied = false
      }}>Dismiss</button
    >
  </div>
{/if}

<form class="toolbar" onsubmit={create}>
  <div class="field">
    <label for="new-email">Email</label>
    <input id="new-email" type="email" bind:value={email} required autocomplete="off" />
  </div>
  <div class="field">
    <label for="new-name">Name</label>
    <input id="new-name" bind:value={name} required />
  </div>
  <div class="field">
    <label for="new-timezone">Timezone</label>
    <input id="new-timezone" bind:value={timezone} required />
  </div>
  <div class="field check">
    <input id="new-admin" type="checkbox" bind:checked={isAdmin} />
    <label for="new-admin">Admin</label>
  </div>
  <button class="primary" type="submit" disabled={createPending}>
    {createPending ? 'Creating…' : 'Create user'}
  </button>
</form>

{#if loading}
  <p>Loading users…</p>
{:else if users.length === 0}
  <p>No users yet.</p>
{:else}
  <table>
    <thead>
      <tr>
        <th scope="col">Email</th>
        <th scope="col">Name</th>
        <th scope="col">Timezone</th>
        <th scope="col">Admin</th>
        <th scope="col">Active</th>
        <th scope="col">Password</th>
        <th scope="col">Actions</th>
      </tr>
    </thead>
    <tbody>
      {#each users as user (user.id)}
        <tr>
          <td>{user.email}</td>
          <td>{user.name}</td>
          <td>{user.timezone}</td>
          <td>{user.is_admin ? 'yes' : 'no'}</td>
          <td>{user.active ? 'yes' : 'no'}</td>
          <td>
            {#if user.must_change_password}
              <span class="badge alert">must change password</span>
            {/if}
          </td>
          <td>
            <div class="row-actions">
              <button class="ghost" type="button" onclick={() => startEdit(user)}>
                {editing === user.id ? 'Close' : 'Edit'}
              </button>
              <button
                class="ghost"
                type="button"
                onclick={() => reset(user)}
                disabled={resetting === user.id}
              >
                {resetting === user.id ? 'Resetting…' : 'Reset password'}
              </button>
            </div>
          </td>
        </tr>
        {#if editing === user.id}
          <tr>
            <td colspan="7">
              <form class="toolbar" onsubmit={(event) => save(event, user)}>
                <div class="field">
                  <label for={`edit-name-${user.id}`}>Name</label>
                  <input id={`edit-name-${user.id}`} bind:value={editName} required />
                </div>
                <div class="field">
                  <label for={`edit-timezone-${user.id}`}>Timezone</label>
                  <input id={`edit-timezone-${user.id}`} bind:value={editTimezone} required />
                </div>
                <div class="field check">
                  <input id={`edit-admin-${user.id}`} type="checkbox" bind:checked={editAdmin} />
                  <label for={`edit-admin-${user.id}`}>Admin</label>
                </div>
                <div class="field check">
                  <input id={`edit-active-${user.id}`} type="checkbox" bind:checked={editActive} />
                  <label for={`edit-active-${user.id}`}>Active</label>
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
