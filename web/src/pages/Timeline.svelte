<script lang="ts">
  import { onMount, tick } from 'svelte'
  import { api, problemMessage } from '../lib/api'
  import type { components } from '../lib/api/schema'
  import { currentWeek, formatDuration } from '../lib/format'
  import { session } from '../lib/session.svelte'
  import { FINE_SNAP, SNAP, clampRange, lanes, snap, timeToX, xToTime } from '../lib/timeline/layout'
  import {
    addDays,
    instantToWallTime,
    localDate,
    startOfDay,
    wallTimeToInstant,
  } from '../lib/timeline/time'

  type Entry = components['schemas']['EntryResponse']
  type Project = components['schemas']['ProjectResponse']

  const DAY = 86_400_000
  const HOUR = 3_600_000
  const MIN = 60_000
  const LANE = 26
  const DAY_ZOOM = 500
  const DEFAULT_DAYS = 21
  const STICKY_PROJECT = 'times.project'

  const user = session.user
  const userID = user?.id ?? 0
  const tz = user?.timezone ?? 'UTC'

  const weekday = new Intl.DateTimeFormat(undefined, { timeZone: 'UTC', weekday: 'short' })
  const month = new Intl.DateTimeFormat(undefined, { timeZone: 'UTC', month: 'short' })
  const clock = new Intl.DateTimeFormat(undefined, { timeZone: tz, hour: '2-digit', minute: '2-digit' })
  const stamp = new Intl.DateTimeFormat(undefined, { timeZone: tz, dateStyle: 'medium', timeStyle: 'short' })

  function defaultStart(): number {
    return wallTimeToInstant(tz, `${addDays(currentWeek(tz).from, -7)}T00:00`)
  }

  let now = $state(Date.now())
  let entries = $state<Entry[]>([])
  let projects = $state<Project[]>([])
  let loading = $state(true)
  let pending = $state(false)
  let error = $state('')
  let projectID = $state('')
  let selectedID = $state<number | null>(null)

  // The window is a strip of local days. Positions are linear in epoch ms, so a
  // 23- or 25-hour DST day is simply narrower or wider than its neighbours.
  let windowStart = $state(defaultStart())
  let windowDays = $state(DEFAULT_DAYS)
  let viewWidth = $state(0)
  let dayWidth = $state(0)
  let scrollEl = $state<HTMLDivElement>()

  // Panel fields are flat state: the union of a new and an edited entry would
  // make every binding narrower than Svelte's checker likes.
  let panelMode = $state<'new' | 'edit' | null>(null)
  let panelEntry = $state<Entry | null>(null)
  let panelProject = $state('')
  let panelStart = $state('')
  let panelEnd = $state('')
  let panelNote = $state('')
  let panelFirst = $state<HTMLSelectElement>()
  const panelRunning = $derived(panelEntry !== null && panelEntry.end_at === null)

  type Drag =
    | {
        mode: 'create'
        anchor: number
        start: number
        end: number
        originX: number
        moved: boolean
        pointerID: number
      }
    | {
        mode: 'move' | 'resize-start' | 'resize-end'
        id: number
        origStart: number
        origEnd: number
        start: number
        end: number
        originX: number
        moved: boolean
        running: boolean
        pointerID: number
      }

  let drag = $state<Drag | null>(null)

  const week = currentWeek(tz)
  const weekStart = wallTimeToInstant(tz, `${week.from}T00:00`)

  const msPerPx = $derived(dayWidth > 0 ? DAY / dayWidth : 0)
  const hasWidth = $derived(viewWidth > 0)
  const days = $derived.by(() => {
    if (!msPerPx) return []
    const first = localDate(tz, windowStart)
    const out: { date: string; start: number; end: number; left: number; width: number }[] = []
    for (let i = 0; i < windowDays; i++) {
      const date = addDays(first, i)
      const start = wallTimeToInstant(tz, `${date}T00:00`)
      const end = wallTimeToInstant(tz, `${addDays(date, 1)}T00:00`)
      out.push({
        date,
        start,
        end,
        left: timeToX(start, windowStart, msPerPx),
        width: (end - start) / msPerPx,
      })
    }
    return out
  })
  const windowEnd = $derived(days.length ? days[days.length - 1].end : windowStart)
  const totalWidth = $derived(msPerPx ? (windowEnd - windowStart) / msPerPx : 0)
  const today = $derived(localDate(tz, now))

  const projectByID = $derived(new Map(projects.map((project) => [project.id, project])))
  const choices = $derived(projects.filter((project) => !project.archived_at))
  const running = $derived(entries.find((entry) => entry.end_at === null) ?? null)

  function projectOf(entry: Entry): Project | undefined {
    return projectByID.get(entry.project_id)
  }

  function colorOf(entry: Entry): string {
    return projectOf(entry)?.color || '#64748b'
  }

  function nameOf(entry: Entry): string {
    return projectOf(entry)?.name ?? `Archived project #${entry.project_id}`
  }

  function describe(entry: Entry, start: number, end: number): string {
    const note = entry.note ? `, ${entry.note}` : ''
    return `${nameOf(entry)}, ${stamp.format(start)} to ${clock.format(end)} (${formatDuration(
      (end - start) / 1000,
    )})${note}`
  }

  const blocks = $derived.by(() => {
    if (!msPerPx) return []
    const visible = entries
      .map((entry) => {
        const stored = Date.parse(entry.start_at)
        const end = entry.end_at ? Date.parse(entry.end_at) : now
        const active = drag && drag.mode !== 'create' && drag.id === entry.id ? drag : null
        const start = active ? active.start : stored
        return { entry, id: entry.id, start, end: Math.max(active ? active.end : end, start + MIN) }
      })
      .filter((span) => span.end > windowStart && span.start < windowEnd)
    const laneOf = lanes(visible)
    return visible.map((span) => {
      const left = Math.max(0, timeToX(span.start, windowStart, msPerPx))
      const right = Math.min(totalWidth, timeToX(span.end, windowStart, msPerPx))
      return { ...span, lane: laneOf.get(span.entry.id) ?? 0, left, width: Math.max(2, right - left) }
    })
  })

  const depth = $derived(blocks.reduce((max, block) => Math.max(max, block.lane + 1), 1))
  const topPad = $derived(dayWidth >= DAY_ZOOM ? 22 : 8)
  const bodyHeight = $derived(Math.max(220, depth * LANE + topPad + 8))
  const nowX = $derived(
    msPerPx && now >= windowStart && now < windowEnd ? timeToX(now, windowStart, msPerPx) : null,
  )
  const draft = $derived.by(() => {
    if (!drag || drag.mode !== 'create' || !drag.moved || !msPerPx) return null
    const left = timeToX(Math.max(drag.start, windowStart), windowStart, msPerPx)
    const right = Math.min(timeToX(drag.end, windowStart, msPerPx), totalWidth)
    return { left, width: Math.max(2, right - left) }
  })
  const gridlines = $derived.by(() => {
    if (!msPerPx || dayWidth < DAY_ZOOM) return []
    const out: { left: number; hour: string }[] = []
    for (const day of days) {
      for (let hour = 1; hour < 24; hour++) {
        const at = wallTimeToInstant(tz, `${day.date}T${String(hour).padStart(2, '0')}:00`)
        if (at < day.start || at >= day.end) continue
        out.push({ left: timeToX(at, windowStart, msPerPx), hour: String(hour) })
      }
    }
    return out
  })

  let requestID = 0

  // load fetches the window with two buffer days on each side, so a block that
  // starts just outside still has its real start and lane. refresh keeps a
  // mutation's error on screen; a window change clears it.
  async function load({ keepError = false }: { keepError?: boolean } = {}) {
    if (!userID) return
    const id = ++requestID
    const from = addDays(localDate(tz, windowStart), -2)
    const to = addDays(localDate(tz, windowStart), windowDays + 2)
    const { data, error: problem, response } = await api.GET('/api/v1/entries', {
      params: { query: { user_id: userID, from, to } },
    })
    if (id !== requestID) return
    if (problem) error = problemMessage(problem, response)
    else {
      entries = data ?? []
      if (!keepError) error = ''
    }
    loading = false
  }

  async function loadProjects() {
    const { data, error: problem, response } = await api.GET('/api/v1/projects')
    if (problem) {
      error = problemMessage(problem, response)
      return
    }
    projects = data ?? []
    const remembered = Number(localStorage.getItem(STICKY_PROJECT) ?? 0)
    const live = projects.filter((project) => !project.archived_at)
    if (!live.some((project) => String(project.id) === projectID)) {
      const picked = live.some((project) => project.id === remembered) ? remembered : live[0]?.id
      projectID = picked === undefined ? '' : String(picked)
      rememberProject()
    }
  }

  function rememberProject() {
    if (projectID) localStorage.setItem(STICKY_PROJECT, projectID)
  }

  async function refresh() {
    now = Date.now()
    await load({ keepError: true })
  }

  $effect(() => {
    if (!hasWidth) return
    windowStart
    windowDays
    void load()
  })

  $effect(() => {
    if (panelMode === null) return
    void tick().then(() => panelFirst?.focus())
  })

  onMount(() => {
    const el = scrollEl
    if (!el) return

    const fit = () => {
      viewWidth = el.clientWidth
      if (viewWidth === 0) return
      const min = viewWidth / 7
      dayWidth = dayWidth === 0 ? min : Math.min(Math.max(dayWidth, min), viewWidth)
    }
    fit()
    const observer = new ResizeObserver(fit)
    observer.observe(el)

    const timer = setInterval(() => (now = Date.now()), 30_000)
    void tick().then(() => scrollTo(weekStart))
    void loadProjects()

    // Wheel is zoom only; shift+wheel keeps the browser's horizontal scroll.
    function onWheel(event: WheelEvent) {
      if (event.shiftKey || !event.deltaY) return
      event.preventDefault()
      void zoom(event.clientX, event.deltaY)
    }
    el.addEventListener('wheel', onWheel, { passive: false })

    return () => {
      observer.disconnect()
      clearInterval(timer)
      el.removeEventListener('wheel', onWheel)
    }
  })

  async function zoom(clientX: number, deltaY: number) {
    const el = scrollEl
    if (!el || !msPerPx || viewWidth === 0) return
    const cursor = Math.min(Math.max(clientX - el.getBoundingClientRect().left, 0), viewWidth)
    const at = xToTime(el.scrollLeft + cursor, windowStart, msPerPx)
    const next = Math.min(
      Math.max(dayWidth * Math.exp(-deltaY * 0.0015), viewWidth / 7),
      viewWidth,
    )
    if (Math.abs(next - dayWidth) < 0.5) return
    dayWidth = next
    await tick()
    el.scrollLeft = timeToX(at, windowStart, DAY / dayWidth) - cursor
  }

  function scrollTo(ms: number) {
    const el = scrollEl
    if (!el || !msPerPx) return
    el.scrollLeft = Math.max(0, timeToX(ms, windowStart, msPerPx) - 8)
  }

  async function goToday() {
    const start = startOfDay(tz, Date.now())
    if (start < windowStart || start >= windowEnd) {
      windowStart = defaultStart()
      windowDays = DEFAULT_DAYS
      await tick()
    }
    scrollTo(start)
  }

  function shiftWindow(days: number) {
    windowStart = wallTimeToInstant(tz, `${addDays(localDate(tz, windowStart), days)}T00:00`)
  }

  let extending = false

  // Reaching within one day of an edge grows the strip by a week on that side
  // and shifts scrollLeft by the width the new days added, so nothing jumps.
  async function onScroll() {
    const el = scrollEl
    if (!el || extending || !msPerPx) return
    if (el.scrollLeft < dayWidth) {
      extending = true
      const oldStart = windowStart
      windowStart = wallTimeToInstant(tz, `${addDays(localDate(tz, windowStart), -7)}T00:00`)
      windowDays += 7
      const added = timeToX(oldStart, windowStart, msPerPx)
      await tick()
      el.scrollLeft += added
      extending = false
    } else if (el.scrollLeft + el.clientWidth > totalWidth - dayWidth) {
      extending = true
      windowDays += 7
      await tick()
      extending = false
    }
  }

  function localX(event: PointerEvent): number {
    const el = scrollEl
    if (!el) return 0
    return event.clientX - el.getBoundingClientRect().left + el.scrollLeft - el.clientLeft
  }

  function timeAt(event: PointerEvent): number {
    const at = xToTime(localX(event), windowStart, msPerPx)
    return Math.min(Math.max(at, windowStart), windowEnd - MIN)
  }

  function capture(event: PointerEvent) {
    scrollEl?.setPointerCapture(event.pointerId)
  }

  function downCanvas(event: PointerEvent) {
    const el = scrollEl
    if (event.button !== 0 || !msPerPx || !el) return
    // A press in a native scrollbar lane belongs to the scrollbar, not a draft.
    const rect = el.getBoundingClientRect()
    if (event.clientX - rect.left >= el.clientWidth || event.clientY - rect.top >= el.clientHeight) return
    if ((event.target as HTMLElement).closest('.strip-header')) return
    capture(event)
    const anchor = snap(timeAt(event), event.altKey ? FINE_SNAP : SNAP)
    drag = {
      mode: 'create',
      anchor,
      start: anchor,
      end: anchor,
      originX: event.clientX,
      moved: false,
      pointerID: event.pointerId,
    }
  }

  function downBlock(event: PointerEvent, entry: Entry, mode: 'move' | 'resize-start' | 'resize-end') {
    if (event.button !== 0 || !msPerPx) return
    event.stopPropagation()
    capture(event)
    const start = Date.parse(entry.start_at)
    const end = entry.end_at ? Date.parse(entry.end_at) : now
    drag = {
      mode,
      id: entry.id,
      origStart: start,
      origEnd: end,
      start,
      end,
      originX: event.clientX,
      moved: false,
      // A running entry is retimed by stopping it; dragging would silently stop
      // the timer, so it stays click-only.
      running: entry.end_at === null,
      pointerID: event.pointerId,
    }
    selectedID = entry.id
  }

  function pointerMove(event: PointerEvent) {
    if (!drag || !msPerPx) return
    if (drag.mode !== 'create' && drag.running) return
    const shift = event.clientX - drag.originX
    if (Math.abs(shift) > 3) drag.moved = true
    const delta = shift * msPerPx
    const step = event.altKey ? FINE_SNAP : SNAP

    if (drag.mode === 'create') {
      const at = snap(timeAt(event), step)
      const range = clampRange(Math.min(drag.anchor, at), Math.max(drag.anchor, at), now)
      drag.start = range.start
      drag.end = range.end
      return
    }
    if (drag.mode === 'move') {
      const start = snap(drag.origStart + delta, step)
      const range = clampRange(start, start + (drag.origEnd - drag.origStart), now, { moving: true })
      drag.start = range.start
      drag.end = range.end
      return
    }
    if (drag.mode === 'resize-start') {
      const range = clampRange(snap(drag.origStart + delta, step), drag.origEnd, now)
      drag.start = range.start
      drag.end = range.end
      return
    }
    const range = clampRange(drag.origStart, snap(drag.origEnd + delta, step), now)
    drag.start = range.start
    drag.end = range.end
  }

  async function pointerUp() {
    const finished = drag
    if (!finished) return
    drag = null

    if (!finished.moved) {
      if (finished.mode === 'create') openNew(finished.anchor, finished.anchor + HOUR)
      else openEditById(finished.id)
      return
    }

    error = ''
    if (finished.mode === 'create') {
      if (finished.end - finished.start < MIN) {
        openNew(finished.anchor, finished.anchor + HOUR)
        return
      }
      const project = Number(projectID)
      if (!project) {
        error = 'Choose a project before drawing time.'
        openNew(finished.start, finished.end)
        return
      }
      pending = true
      const { data, error: problem, response } = await api.POST('/api/v1/entries', {
        body: {
          project_id: project,
          start_at: new Date(finished.start).toISOString(),
          end_at: new Date(finished.end).toISOString(),
          note: '',
        },
      })
      pending = false
      if (problem) error = problemMessage(problem, response)
      else if (data) openEdit(data)
      await refresh()
      return
    }

    pending = true
    const { data, error: problem, response } = await api.PATCH('/api/v1/entries/{id}', {
      params: { path: { id: finished.id } },
      body: {
        start_at: new Date(finished.start).toISOString(),
        end_at: new Date(finished.end).toISOString(),
      },
    })
    pending = false
    if (problem) error = problemMessage(problem, response)
    else if (data) {
      if (panelMode === 'edit' && panelEntry?.id === data.id) openEdit(data)
      else selectedID = finished.id
    }
    await refresh()
  }

  function cancelDrag() {
    if (!drag) return
    if (scrollEl?.hasPointerCapture(drag.pointerID)) scrollEl.releasePointerCapture(drag.pointerID)
    drag = null
  }

  function openNew(startMs: number, endMs: number) {
    const range = clampRange(snap(startMs, SNAP), snap(endMs, SNAP), now)
    panelMode = 'new'
    panelEntry = null
    panelProject = projectID
    panelStart = instantToWallTime(tz, range.start)
    panelEnd = instantToWallTime(tz, range.end)
    panelNote = ''
    selectedID = null
  }

  function openEdit(entry: Entry) {
    panelMode = 'edit'
    panelEntry = entry
    panelProject = String(entry.project_id)
    panelStart = instantToWallTime(tz, Date.parse(entry.start_at))
    panelEnd = entry.end_at ? instantToWallTime(tz, Date.parse(entry.end_at)) : instantToWallTime(tz, now)
    panelNote = entry.note
    selectedID = entry.id
  }

  function openEditById(id: number) {
    const entry = entries.find((candidate) => candidate.id === id)
    if (entry) openEdit(entry)
  }

  function closePanel() {
    panelMode = null
    panelEntry = null
  }

  async function savePanel() {
    if (panelMode === null || pending) return
    const project = Number(panelProject)
    if (!project) {
      error = 'Choose a project first.'
      return
    }
    error = ''
    pending = true
    try {
      if (panelMode === 'new') {
        const range = clampRange(
          wallTimeToInstant(tz, panelStart),
          wallTimeToInstant(tz, panelEnd),
          now,
        )
        const { data, error: problem, response } = await api.POST('/api/v1/entries', {
          body: {
            project_id: project,
            start_at: new Date(range.start).toISOString(),
            end_at: new Date(range.end).toISOString(),
            note: panelNote,
          },
        })
        if (problem) {
          error = problemMessage(problem, response)
          return
        }
        if (data) openEdit(data)
      } else {
        const entry = panelEntry
        if (!entry) return
        const body: components['schemas']['EntryPatchInputBody'] = { note: panelNote }
        if (project !== entry.project_id) body.project_id = project
        // Only send times that the user touched: a note edit must not round an
        // entry with seconds down to the minute.
        const changed =
          panelStart !== instantToWallTime(tz, Date.parse(entry.start_at)) ||
          (entry.end_at !== null &&
            panelEnd !== instantToWallTime(tz, Date.parse(entry.end_at)))
        if (entry.end_at && changed) {
          const range = clampRange(
            wallTimeToInstant(tz, panelStart),
            wallTimeToInstant(tz, panelEnd),
            now,
          )
          body.start_at = new Date(range.start).toISOString()
          body.end_at = new Date(range.end).toISOString()
        }
        const { data, error: problem, response } = await api.PATCH('/api/v1/entries/{id}', {
          params: { path: { id: entry.id } },
          body,
        })
        if (problem) {
          error = problemMessage(problem, response)
          return
        }
        if (data) openEdit(data)
      }
      await refresh()
    } finally {
      pending = false
    }
  }

  async function deleteEntry(id: number) {
    if (pending) return
    error = ''
    pending = true
    const { error: problem, response } = await api.DELETE('/api/v1/entries/{id}', {
      params: { path: { id } },
    })
    pending = false
    if (problem) {
      error = problemMessage(problem, response)
      return
    }
    if (panelEntry?.id === id) closePanel()
    if (selectedID === id) selectedID = null
    await refresh()
  }

  function removeSelected() {
    if (selectedID === null) return
    if (!window.confirm('Delete this entry?')) return
    void deleteEntry(selectedID)
  }

  async function startTimer() {
    const project = Number(projectID)
    if (!project || pending) return
    error = ''
    pending = true
    const { data, error: problem, response } = await api.POST('/api/v1/timer/start', {
      body: { project_id: project },
    })
    pending = false
    if (problem) error = problemMessage(problem, response)
    else if (data) selectedID = data.id
    await refresh()
  }

  async function stopTimer() {
    if (pending) return
    error = ''
    pending = true
    const { data, error: problem, response } = await api.POST('/api/v1/timer/stop', {})
    pending = false
    if (problem) error = problemMessage(problem, response)
    else if (data) {
      selectedID = data.id
      if (panelEntry?.id === data.id) openEdit(data)
    }
    await refresh()
  }

  function toggleTimer() {
    if (pending) return
    if (running) void stopTimer()
    else void startTimer()
  }

  async function deletePanelEntry() {
    if (panelEntry === null) return
    if (!window.confirm('Delete this entry?')) return
    await deleteEntry(panelEntry.id)
  }

  function onKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      if (drag) {
        event.preventDefault()
        cancelDrag()
      } else if (panelMode !== null) {
        event.preventDefault()
        closePanel()
      }
      return
    }
    const target = event.target
    // Let native Space activation work on buttons and links.
    if (target instanceof HTMLElement && target.closest('input, select, textarea, button, a')) return
    if (panelMode !== null) return

    if (event.key === ' ') {
      event.preventDefault()
      toggleTimer()
    } else if (event.key === 'n' || event.key === 'N') {
      event.preventDefault()
      const end = snap(now, SNAP)
      openNew(end - HOUR, end)
    } else if (event.key === 'Delete' || event.key === 'Backspace') {
      if (selectedID === null) return
      event.preventDefault()
      removeSelected()
    }
  }
</script>

<svelte:window onkeydown={onKeydown} />

<div class="timeline-page">
  <h1>Timeline</h1>

  {#if error}
    <p class="error" role="alert">{error}</p>
  {/if}

  <div class="toolbar">
    <div class="field">
      <label for="timeline-project">Project</label>
      <select id="timeline-project" bind:value={projectID} onchange={rememberProject} disabled={pending}>
        {#if choices.length === 0}
          <option value="">No projects</option>
        {/if}
        {#each choices as project (project.id)}
          <option value={String(project.id)}>{project.name}</option>
        {/each}
      </select>
    </div>
    <button
      class={running ? 'danger' : 'primary'}
      type="button"
      onclick={toggleTimer}
      disabled={pending || (!running && !projectID)}
    >
      {running ? 'Stop' : 'Start'}
    </button>
    {#if running}
      <span class="hint">
        {nameOf(running)} · {formatDuration((now - Date.parse(running.start_at)) / 1000)}
      </span>
    {/if}
    <div class="timeline-nav">
      <button class="ghost" type="button" aria-label="Previous week" onclick={() => shiftWindow(-7)}>
        ◀
      </button>
      <button class="ghost" type="button" onclick={goToday}>Today</button>
      <button class="ghost" type="button" aria-label="Next week" onclick={() => shiftWindow(7)}>▶</button>
    </div>
    <p class="hint">
      Drag to log · Alt for 1-minute steps · space start/stop · n new · Delete remove · Esc cancel
    </p>
  </div>

  {#if loading}
    <p class="hint">Loading timeline…</p>
  {:else if blocks.length === 0}
    <p class="hint">Drag on the timeline to log time, or press n.</p>
  {/if}

  <div class="timeline-layout" class:with-panel={panelMode !== null}>
    <div
      class="timeline-scroll"
      role="group"
      aria-label="Timeline"
      bind:this={scrollEl}
      onscroll={onScroll}
      onpointerdown={downCanvas}
      onpointermove={pointerMove}
      onpointerup={pointerUp}
      onpointercancel={cancelDrag}
    >
      {#if msPerPx}
        <div class="strip" style:width={`${totalWidth}px`}>
          <div class="strip-header">
            {#each days as day (day.date)}
              <div
                class="day-head"
                class:today={day.date === today}
                style:left={`${day.left}px`}
                style:width={`${day.width}px`}
              >
                <span class="weekday">{weekday.format(new Date(`${day.date}T00:00:00Z`))}</span>
                <span class="daynum">{Number(day.date.slice(8, 10))}</span>
                <span class="month">{month.format(new Date(`${day.date}T00:00:00Z`))}</span>
              </div>
            {/each}
          </div>

          <div class="strip-body" style:height={`${bodyHeight}px`}>
            {#each days as day (day.date)}
              <div
                class="day-band"
                class:today={day.date === today}
                style:left={`${day.left}px`}
                style:width={`${day.width}px`}
              ></div>
            {/each}

            {#each gridlines as line, index (index)}
              <div class="hour-line" style:left={`${line.left}px`}>
                <span>{line.hour}</span>
              </div>
            {/each}

            {#each blocks as block (block.entry.id)}
              <div
                class="entry-block"
                class:selected={selectedID === block.entry.id}
                class:running={!block.entry.end_at}
                role="button"
                tabindex="0"
                aria-label={describe(block.entry, block.start, block.end)}
                title={describe(block.entry, block.start, block.end)}
                style:left={`${block.left}px`}
                style:width={`${block.width}px`}
                style:top={`${block.lane * LANE + topPad}px`}
                style:background={colorOf(block.entry)}
                onpointerdown={(event) => downBlock(event, block.entry, 'move')}
                onkeydown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault()
                    openEdit(block.entry)
                  }
                }}
              >
                <span
                  class="resize left"
                  aria-hidden="true"
                  onpointerdown={(event) => downBlock(event, block.entry, 'resize-start')}
                ></span>
                {#if block.width > 48}
                  <span class="block-text">
                    {nameOf(block.entry)}{block.entry.note ? ` · ${block.entry.note}` : ''}
                  </span>
                {/if}
                <span
                  class="resize right"
                  aria-hidden="true"
                  onpointerdown={(event) => downBlock(event, block.entry, 'resize-end')}
                ></span>
              </div>
            {/each}

            {#if draft}
              <div
                class="draft"
                style:left={`${draft.left}px`}
                style:width={`${draft.width}px`}
                style:top={`${topPad}px`}
              ></div>
            {/if}

            {#if nowX !== null}
              <div class="now-line" style:left={`${nowX}px`}>
                <span>{clock.format(now)}</span>
              </div>
            {/if}
          </div>
        </div>
      {/if}
    </div>

    {#if panelMode !== null}
      <aside class="entry-panel" aria-label={panelMode === 'new' ? 'New entry' : 'Edit entry'}>
        <h2>{panelMode === 'new' ? 'New entry' : 'Edit entry'}</h2>
        <form onsubmit={(event) => { event.preventDefault(); void savePanel() }}>
          <div class="field">
            <label for="panel-project">Project</label>
            <select id="panel-project" bind:this={panelFirst} bind:value={panelProject} disabled={pending}>
              {#if panelEntry && !projectByID.has(panelEntry.project_id)}
                <option value={String(panelEntry.project_id)}>Archived project</option>
              {/if}
              {#each choices as project (project.id)}
                <option value={String(project.id)}>{project.name}</option>
              {/each}
            </select>
          </div>
          <div class="field">
            <label for="panel-start">Start</label>
            <input
              id="panel-start"
              type="datetime-local"
              step="60"
              bind:value={panelStart}
              disabled={pending || panelRunning}
            />
          </div>
          {#if !panelRunning}
            <div class="field">
              <label for="panel-end">End</label>
              <input
                id="panel-end"
                type="datetime-local"
                step="60"
                bind:value={panelEnd}
                disabled={pending}
              />
            </div>
          {/if}
          <div class="field">
            <label for="panel-note">Note</label>
            <textarea id="panel-note" rows="3" bind:value={panelNote} disabled={pending}></textarea>
          </div>
          <div class="row-actions">
            <button class="primary" type="submit" disabled={pending}>
              {pending ? 'Saving…' : 'Save'}
            </button>
            {#if panelMode === 'edit'}
              <button class="ghost" type="button" onclick={stopTimer} disabled={pending || !panelRunning}>
                Stop now
              </button>
              <button class="danger" type="button" onclick={deletePanelEntry} disabled={pending}>
                Delete
              </button>
            {/if}
            <button class="ghost" type="button" onclick={closePanel} disabled={pending}>Cancel</button>
          </div>
          {#if panelRunning}
            <p class="hint">Running since {panelStart.replace('T', ' ')}. Stop it to change the start.</p>
          {/if}
        </form>
      </aside>
    {/if}
  </div>
</div>
