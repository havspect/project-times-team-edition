// A history router small enough to read: path state plus push/replace and the
// back button. Query strings stay on the URL and are read by the page that needs
// them; the path is what selects a screen.

function currentPath(): string {
  return location.pathname.replace(/\/+$/, '') || '/'
}

let path = $state(currentPath())

export const router = {
  get path(): string {
    return path
  },
  navigate(to: string, options: { replace?: boolean } = {}): void {
    if (to === path) return
    if (options.replace) history.replaceState(null, '', to)
    else history.pushState(null, '', to)
    path = currentPath()
  },
}

window.addEventListener('popstate', () => {
  path = currentPath()
})
