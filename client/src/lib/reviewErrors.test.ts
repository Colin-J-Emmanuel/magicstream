import { describe, expect, it } from 'vitest'
import { ApiError } from '../api/client'
import { reviewErrorMessage } from './reviewErrors'

describe('reviewErrorMessage', () => {
  it.each([
    [400, 'review must not be empty'],
    [403, "doesn't have permission"],
    [404, 'no longer exists'],
    [502, 'Rewording it usually helps'],
    [503, 'Try again in a minute'],
    [504, 'took too long'],
    [500, 'on our side'],
  ])('status %i explains what to do', (status, expected) => {
    expect(reviewErrorMessage(new ApiError(status, 'review must not be empty'))).toContain(expected)
  })

  it.each([400, 401, 403, 404, 500, 502, 503, 504])('status %i says the review was not saved', (status) => {
    expect(reviewErrorMessage(new ApiError(status, 'x'))).toContain("wasn't saved")
  })

  it('handles network failures', () => {
    expect(reviewErrorMessage(new TypeError('Failed to fetch'))).toContain("Couldn't reach the server")
  })

  it('never advises retrying a misconfiguration', () => {
    expect(reviewErrorMessage(new ApiError(500, 'x'))).not.toMatch(/try again/i)
  })
})