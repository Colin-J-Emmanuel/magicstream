import { ApiError } from '../api/client'

// reviewErrorMessage turns a failed save into something an editor can act on.
// Every failure says the review wasn't saved, so nobody assumes it was.
export function reviewErrorMessage(err: unknown): string {
  if (!(err instanceof ApiError)) {
    return "Couldn't reach the server. Your review wasn't saved. Check your connection and try again."
  }
  switch (err.status) {
    case 400:
      return `Your review wasn't saved: ${err.message}.`
    case 401:
      return "Your session has ended, so your review wasn't saved. Log in again to save it."
    case 403:
      return "Your review wasn't saved: your account doesn't have permission to edit reviews."
    case 404:
      return "This movie no longer exists, so your review wasn't saved."
    case 502:
      return "The ranking service returned an answer we couldn't use, so your review wasn't saved. Rewording it usually helps."
    case 503:
      return "The ranking service is busy or unavailable, so your review wasn't saved. Try again in a minute."
    case 504:
      return "Ranking took too long, so your review wasn't saved. Try again."
    default:
      return "Something went wrong on our side, so your review wasn't saved."
  }
}