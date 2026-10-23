<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { HTMLSelectAttributes } from 'svelte/elements'

  let {
    label,
    id,
    value = $bindable(''),
    hint = '',
    children,
    ...rest
  }: Omit<HTMLSelectAttributes, 'value'> & {
    label: string
    id: string
    value?: string
    hint?: string
    children: Snippet
  } = $props()
</script>

<div class="field">
  <label for={id}>{label}</label>
  <select {id} bind:value {...rest}>{@render children()}</select>
  {#if hint}<span class="hint">{hint}</span>{/if}
</div>
