<script lang="ts">
  import { session } from '../lib/session.svelte'

  let email = $state('')
  let password = $state('')
  let error = $state('')
  let pending = $state(false)

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    error = ''
    pending = true
    try {
      await session.login(email, password)
    } catch (err) {
      error = err instanceof Error ? err.message : String(err)
    } finally {
      pending = false
    }
  }
</script>

<form class="auth" onsubmit={submit}>
  <h1>Sign in</h1>
  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}
  <label for="email">Email</label>
  <input id="email" type="email" bind:value={email} required autocomplete="username" />
  <label for="password">Password</label>
  <input id="password" type="password" bind:value={password} required autocomplete="current-password" />
  <button class="primary" type="submit" disabled={pending}>
    {pending ? 'Signing in…' : 'Sign in'}
  </button>
</form>
