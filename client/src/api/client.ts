export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

// errorMessage turns any thrown value into text a user can read.
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message
  return 'Could not reach the server. Please try again.'
}

let onSessionExpired: (() => void) | null = null

// setSessionExpiredHandler registers what to do when a session can't be renewed.
export function setSessionExpiredHandler(handler: (() => void) | null) {
  onSessionExpired = handler
}

// A 401 from these routes is an answer (wrong password, no session), not an expired
// access token, so it must never trigger a refresh. Refreshing on /auth/refresh would loop.
const NO_REFRESH = ['/auth/login', '/auth/register', '/auth/refresh', '/auth/logout']

let refreshInFlight: Promise<boolean> | null = null

// refreshSession runs at most one refresh at a time; concurrent callers share it.
// The refresh token is single-use, so two parallel refreshes would present the same
// token twice, trip the server's reuse detection, and revoke the session.
function refreshSession(): Promise<boolean> {
  if (!refreshInFlight) {
    refreshInFlight = fetch('/api/auth/refresh', { method: 'POST' })
      .then((res) => res.ok)
      .catch(() => false)
      .finally(() => {
        refreshInFlight = null
      })
  }
  return refreshInFlight
}

function send(path: string, init: RequestInit): Promise<Response> {
  return fetch(`/api${path}`, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...init.headers },
  })
}

// api sends a request to the Go backend and returns the parsed JSON body.
// An expired access token is renewed once, transparently, and the request retried.
// Non-2xx responses throw an ApiError carrying the status and the server's message.
export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  let res = await send(path, init)

  if (res.status === 401 && !NO_REFRESH.includes(path)) {
    if (await refreshSession()) {
      res = await send(path, init) // retry once with the new access token
    } else {
      onSessionExpired?.()
    }
  }

  if (res.status === 204) {
    return undefined as T
  }
  const body = await res.json().catch(() => null)
  if (!res.ok) {
    throw new ApiError(res.status, body?.error ?? res.statusText)
  }
  return body as T
}