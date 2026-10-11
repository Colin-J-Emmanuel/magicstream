// returnPath extracts where to send the user after signing in.
// Only same-site paths are allowed: a leading "/" but not "//", because browsers treat
// "//evil.com" as a protocol-relative URL to another site. Anything else falls back to "/".
export function returnPath(state: unknown): string {
  const from = (state as { from?: unknown } | null)?.from
  if (typeof from === 'string' && from.startsWith('/') && !from.startsWith('//') && !from.startsWith('/\\')) {
    return from
  }
  return '/'
}