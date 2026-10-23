<script lang="ts">
  import Link from './Link.svelte'
  import { router } from './router.svelte'
  import { session } from './session.svelte'

  const tabs = [
    { href: '/admin/users', label: 'Users' },
    { href: '/admin/teams', label: 'Teams' },
    { href: '/admin/projects', label: 'Projects' },
    { href: '/admin/entries', label: 'Deleted entries' },
  ]
</script>

{#if session.user?.is_admin}
  <nav class="tabs" aria-label="Admin">
    {#each tabs as tab (tab.href)}
      <Link href={tab.href} current={router.path === tab.href}>{tab.label}</Link>
    {/each}
  </nav>
{/if}
