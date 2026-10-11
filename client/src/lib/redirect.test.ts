import { describe, expect, it } from 'vitest'
import { returnPath } from './redirect'

describe('returnPath', () => {
  it('returns a same-site path', () => {
    expect(returnPath({ from: '/recommendations' })).toBe('/recommendations')
    expect(returnPath({ from: '/movies/tt0111161?x=1' })).toBe('/movies/tt0111161?x=1')
  })

  it.each([
    ['protocol-relative URL', '//evil.com'],
    ['backslash variant', '/\\evil.com'],
    ['absolute URL', 'https://evil.com'],
    ['javascript URL', 'javascript:alert(1)'],
    ['relative path', 'recommendations'],
  ])('rejects a %s', (_, from) => {
    expect(returnPath({ from })).toBe('/')
  })

  it.each([undefined, null, {}, { from: 42 }])('falls back to / for state %j', (state) => {
    expect(returnPath(state)).toBe('/')
  })
})