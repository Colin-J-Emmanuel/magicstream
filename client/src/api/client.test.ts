import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError, setSessionExpiredHandler } from './client'

function json(status: number, body: unknown = {}) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

afterEach(() => {
  vi.unstubAllGlobals()
  setSessionExpiredHandler(null)
})

describe('api', () => {
  it('refreshes once and retries when the access token has expired', async () => {
    let accessValid = false
    const fetchMock = vi.fn(async (url: string) => {
      if (url === '/api/auth/refresh') {
        accessValid = true
        return json(200)
      }
      return accessValid ? json(200, { ok: true }) : json(401, { error: 'authentication required' })
    })
    vi.stubGlobal('fetch', fetchMock)

    await expect(api('/me')).resolves.toEqual({ ok: true })
    expect(fetchMock.mock.calls.map((c) => c[0])).toEqual(['/api/me', '/api/auth/refresh', '/api/me'])
  })

  it('shares a single refresh between concurrent 401s', async () => {
    let accessValid = false
    let releaseRefresh!: () => void
    const refreshGate = new Promise<void>((resolve) => (releaseRefresh = resolve))
    const fetchMock = vi.fn(async (url: string) => {
      if (url === '/api/auth/refresh') {
        await refreshGate // hold the refresh open so all three requests pile up behind it
        accessValid = true
        return json(200)
      }
      return accessValid ? json(200, {}) : json(401, { error: 'authentication required' })
    })
    vi.stubGlobal('fetch', fetchMock)

    const all = Promise.all([api('/me'), api('/recommendations'), api('/admin/users')])
    await new Promise((resolve) => setTimeout(resolve, 10)) // let all three receive their 401
    releaseRefresh()
    await all

    const refreshes = fetchMock.mock.calls.filter((c) => c[0] === '/api/auth/refresh')
    expect(refreshes).toHaveLength(1)
  })

  it('reports an expired session, without looping, when the refresh fails', async () => {
    const onExpired = vi.fn()
    setSessionExpiredHandler(onExpired)
    const fetchMock = vi.fn(async () => json(401, { error: 'authentication required' }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(api('/me')).rejects.toBeInstanceOf(ApiError)
    expect(fetchMock).toHaveBeenCalledTimes(2) // the request, then one refresh: no loop
    expect(onExpired).toHaveBeenCalledTimes(1)
  })

  it('never refreshes on auth routes', async () => {
    const fetchMock = vi.fn(async () => json(401, { error: 'invalid email or password' }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(api('/auth/login', { method: 'POST' })).rejects.toThrow('invalid email or password')
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })
})