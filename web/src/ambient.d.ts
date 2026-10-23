// tsc does not know Svelte components, so imports of *.svelte resolve to this
// shape. svelte-check type-checks the real files.
declare module '*.svelte' {
  import type { Component } from 'svelte'

  const component: Component
  export default component
}
