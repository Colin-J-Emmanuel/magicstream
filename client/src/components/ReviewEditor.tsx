import { useState, type FormEvent } from 'react'
import { api } from '../api/client'
import type { Movie } from '../api/types'
import { NOT_RANKED_VALUE, rankingLabel } from '../lib/ranking'
import { reviewErrorMessage } from '../lib/reviewErrors'

const MAX_LENGTH = 2000
const SLOW_AFTER_MS = 3000

export function ReviewEditor({ movie, onSaved }: { movie: Movie; onSaved: (updated: Movie) => void }) {
  const [text, setText] = useState(movie.admin_review)
  const [saving, setSaving] = useState(false)
  const [slow, setSlow] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState<string | null>(null)

  const review = text.trim()
  const length = [...review].length // characters, matching the server's count
  const unchanged = review === movie.admin_review && movie.ranking.ranking_value !== NOT_RANKED_VALUE
  const canSave = !saving && length > 0 && length <= MAX_LENGTH && !unchanged

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!canSave) return
    setError(null)
    setSaved(null)
    setSaving(true)
    const slowTimer = setTimeout(() => setSlow(true), SLOW_AFTER_MS)
    try {
      // No abort signal on purpose: a save should finish even if the editor navigates away.
      const updated = await api<Movie>(`/admin/movies/${encodeURIComponent(movie.imdb_id)}/review`, {
        method: 'PATCH',
        body: JSON.stringify({ review }),
      })
      onSaved(updated)
      setSaved(`Saved and ranked ${rankingLabel(updated.ranking)}.`)
    } catch (err) {
      setError(reviewErrorMessage(err)) // the text stays in the box
    } finally {
      clearTimeout(slowTimer)
      setSlow(false)
      setSaving(false)
    }
  }

  return (
    <form className="form editor" onSubmit={handleSubmit} aria-busy={saving}>
      <h3 className="editor-heading">Edit review</h3>
      <label className="field">
        Review text
        <textarea
          value={text}
          maxLength={MAX_LENGTH}
          rows={6}
          onChange={(e) => {
            setText(e.target.value)
            setSaved(null)
          }}
        />
      </label>
      <p className={length > MAX_LENGTH ? 'char-count over' : 'char-count'}>
        {length} / {MAX_LENGTH}
      </p>
      <p className="form-status">Saving sends the review to the ranking model. Nothing is saved if ranking fails.</p>
      {error && (
        <p className="form-error" role="alert">
          {error}
        </p>
      )}
      <p className="form-status" role="status">
        {slow ? 'This is taking longer than usual. The ranking service may be busy; your review will be saved if it succeeds.' : saved}
      </p>
      <button type="submit" className="button" disabled={!canSave}>
        {saving ? 'Ranking…' : unchanged ? 'No changes to save' : 'Rank and save'}
      </button>
    </form>
  )
}