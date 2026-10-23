<script lang="ts">
  import { api, problemMessage } from '../lib/api'
  import { router } from '../lib/router.svelte'
  import { session } from '../lib/session.svelte'
  import Button from '../lib/ui/Button.svelte'
  import Input from '../lib/ui/Input.svelte'

  let current = $state('')
  let next = $state('')
  let error = $state('')
  let success = $state(false)
  let pending = $state(false)

  async function submit(event: SubmitEvent) {
    event.preventDefault()
    error = ''
    if (next.length < 8) {
      error = 'The new password must be at least 8 characters.'
      return
    }
    pending = true
    try {
      const { error: problem, response } = await api.POST('/api/v1/me/password', {
        body: { current_password: current, new_password: next },
      })
      if (problem) throw new Error(problemMessage(problem, response))
      success = true
      await session.load()
      router.navigate('/timeline', { replace: true })
    } catch (err) {
      error = err instanceof Error ? err.message : String(err)
    } finally {
      pending = false
    }
  }
</script>

<form class="auth" onsubmit={submit}>
  <h1>Change password</h1>
  {#if success}
    <p class="success" role="status">Password changed.</p>
  {/if}
  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}
  <Input
    id="current"
    label="Current password"
    type="password"
    bind:value={current}
    required
    autocomplete="current-password"
  />
  <Input
    id="next"
    label="New password"
    type="password"
    bind:value={next}
    required
    minlength={8}
    autocomplete="new-password"
  />
  <Button type="submit" disabled={pending}>
    {pending ? 'Saving…' : 'Change password'}
  </Button>
</form>
