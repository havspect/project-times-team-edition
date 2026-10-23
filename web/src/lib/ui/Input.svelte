<script lang="ts">
  import type { HTMLInputAttributes } from 'svelte/elements'

  let {
    label,
    id,
    value = $bindable(''),
    checked = $bindable(false),
    hint = '',
    ...rest
  }: Omit<HTMLInputAttributes, 'value' | 'checked'> & {
    label: string
    id: string
    value?: string
    checked?: boolean
    hint?: string
  } = $props()

  const type = $derived(rest.type ?? 'text')
</script>

{#if type === 'checkbox'}
  <div class="field check">
    <input
      {id}
      checked={checked}
      onchange={(event) => (checked = event.currentTarget.checked)}
      {...rest}
    />
    <label for={id}>{label}</label>
    {#if hint}<span class="hint">{hint}</span>{/if}
  </div>
{:else}
  <div class="field">
    <label for={id}>{label}</label>
    <input {id} value={value} oninput={(event) => (value = event.currentTarget.value)} {...rest} />
    {#if hint}<span class="hint">{hint}</span>{/if}
  </div>
{/if}
