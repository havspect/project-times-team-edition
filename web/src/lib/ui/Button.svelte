<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { HTMLButtonAttributes } from 'svelte/elements'

  let {
    variant = 'primary',
    active,
    type = 'button',
    class: className = '',
    children,
    ...rest
  }: HTMLButtonAttributes & {
    variant?: 'primary' | 'secondary' | 'danger'
    active?: boolean
    children: Snippet
  } = $props()
</script>

<button class="{variant} {className}" {type} aria-pressed={active} {...rest}>
  {@render children()}
</button>
