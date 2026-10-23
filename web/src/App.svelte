<script lang="ts">
  import { onMount } from 'svelte'
  import type { Component } from 'svelte'
  import { router } from './lib/router.svelte'
  import { session } from './lib/session.svelte'
  import Link from './lib/Link.svelte'
  import Login from './pages/Login.svelte'
  import NotFound from './pages/NotFound.svelte'
  import Password from './pages/Password.svelte'
  import Reports from './pages/Reports.svelte'
  import Timeline from './pages/Timeline.svelte'
  import AdminEntries from './pages/AdminEntries.svelte'
  import AdminProjects from './pages/AdminProjects.svelte'
  import AdminTeams from './pages/AdminTeams.svelte'
  import AdminUsers from './pages/AdminUsers.svelte'

  const routes: Record<string, Component> = {
    '/login': Login,
    '/password': Password,
    '/timeline': Timeline,
    '/reports': Reports,
    '/admin/users': AdminUsers,
    '/admin/teams': AdminTeams,
    '/admin/projects': AdminProjects,
    '/admin/entries': AdminEntries,
  }

  let ready = $state(false)

  onMount(async () => {
    try {
      await session.load()
    } catch {
      // Offline: show the login screen rather than guess at a session.
    }
    ready = true
  })

  const user = $derived(session.user)
  const leads = $derived(user?.memberships?.some((m) => m.role === 'lead') ?? false)
  const Page = $derived(routes[router.path] ?? NotFound)

  // One guard for the whole shell: no session -> login; a reset password ->
  // the password screen; a signed-in user on an auth screen -> the timeline.
  $effect(() => {
    if (!ready) return
    const path = router.path
    if (!user) {
      if (path !== '/login') router.navigate('/login', { replace: true })
    } else if (session.mustChangePassword) {
      if (path !== '/password') router.navigate('/password', { replace: true })
    } else if (path.startsWith('/admin') && !user.is_admin) {
      router.navigate('/timeline', { replace: true })
    } else if (path === '/reports' && !user.is_admin && !leads) {
      router.navigate('/timeline', { replace: true })
    } else if (path === '/login' || path === '/password') {
      router.navigate('/timeline', { replace: true })
    }
  })

  async function logout() {
    await session.logout()
    router.navigate('/login', { replace: true })
  }
</script>

<header>
  <Link href="/timeline" class="brand">Project Times</Link>
  {#if user}
    <nav>
      <Link href="/timeline">Timeline</Link>
      {#if user.is_admin || leads}
        <Link href="/reports">Reports</Link>
      {/if}
      {#if user.is_admin}
        <Link href="/admin/users">Admin</Link>
      {/if}
    </nav>
    <div class="who">
      <span>{user.name} · {user.timezone}</span>
      <button class="ghost" type="button" onclick={logout}>Log out</button>
    </div>
  {/if}
</header>

<main>
  {#if ready}
    <Page />
  {/if}
</main>
