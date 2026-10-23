<script lang="ts">
  import type { HTMLTextareaAttributes } from 'svelte/elements'

  let {
    label,
    id,
    value = $bindable(''),
    hint = '',
    ...rest
  }: Omit<HTMLTextareaAttributes, 'value'> & {
    label: string
    id: string
    value?: string
    hint?: string
  } = $props()
</script>

<div class="field">
  <label for={id}>{label}</label>
  <textarea {id} bind:value {...rest}></textarea>
  {#if hint}<span class="hint">{hint}</span>{/if}
</div>
