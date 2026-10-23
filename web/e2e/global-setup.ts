import { execFileSync, spawn, type ChildProcess } from 'node:child_process'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

const server = process.env.TSTAMPS_SERVER ?? fileURLToPath(new URL('../../tstamps-server', import.meta.url))
const statePath = fileURLToPath(new URL('./.state.json', import.meta.url))
const port = Number(process.env.TSTAMPS_E2E_PORT ?? 8098)
const baseURL = `http://127.0.0.1:${port}`

// Bootstraps a throwaway workspace with the real server binary and leaves it
// running for the spec. The returned function stops it after the last test.
export default async function globalSetup() {
  const dir = mkdtempSync(join(tmpdir(), 'tstamps-e2e-'))
  const db = join(dir, 'tstamps.db')

  let child: ChildProcess | undefined
  try {
    const output = execFileSync(
      server,
      ['admin', 'create', '--db', db, '--email', 'admin@example.com', '--name', 'Ada Admin'],
      { encoding: 'utf8' },
    )
    const password = /initial password: (\S+)/.exec(output)?.[1]
    if (!password) throw new Error(`admin create printed no password: ${output}`)

    child = spawn(
      server,
      ['serve', '--addr', `127.0.0.1:${port}`, '--db', db, '--log-level', 'warn'],
      { stdio: 'ignore' },
    )
    await waitForHealth(`${baseURL}/healthz`)
    writeFileSync(statePath, JSON.stringify({ port, password }))

    return async () => {
      child?.kill('SIGTERM')
      await new Promise((resolve) => child?.once('exit', resolve) ?? resolve(undefined))
      rmSync(dir, { recursive: true, force: true })
      rmSync(statePath, { force: true })
    }
  } catch (err) {
    child?.kill('SIGTERM')
    rmSync(dir, { recursive: true, force: true })
    throw err
  }
}

async function waitForHealth(url: string) {
  const deadline = Date.now() + 15_000
  while (Date.now() < deadline) {
    try {
      if ((await fetch(url)).ok) return
    } catch {
      // Not listening yet.
    }
    await new Promise((resolve) => setTimeout(resolve, 100))
  }
  throw new Error(`server did not answer ${url}`)
}
