import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { api, ApiError, errorMessage } from '../api/client'
import type { Movie } from '../api/types'
import { RankingBulbs } from '../components/RankingBulbs'
import { Trailer } from '../components/Trailer'

export function MovieDetailPage() {
  const { imdbId = '' } = useParams()
  // A new key per movie gives each one fresh state, so a previous movie never flashes on screen.
  return <MovieDetail key={imdbId} imdbId={imdbId} />
}

function MovieDetail({ imdbId }: { imdbId: string }) {
  const [movie, setMovie] = useState<Movie | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const controller = new AbortController()
    api<Movie>(`/movies/${encodeURIComponent(imdbId)}`, { signal: controller.signal })
      .then(setMovie)
      .catch((err) => {
        if (err.name === 'AbortError') return
        const notFound = err instanceof ApiError && (err.status === 404 || err.status === 400)
        setError(notFound ? "We couldn't find that movie." : errorMessage(err))
      })
    return () => controller.abort()
  }, [imdbId])

  if (error) {
    return (
      <>
        <h1 className="page-title">Movie not found</h1>
        <p className="lede">{error}</p>
        <Link to="/">Go to all movies</Link>
      </>
    )
  }
  if (!movie) return <p className="status">Loading…</p>

  return (
    <>
      <Link to="/" className="back-link">
        All movies
      </Link>
      <h1 className="page-title">{movie.title}</h1>
      <p className="lede">{movie.genre.map((g) => g.genre_name).join(', ')}</p>
      <div className="detail">
        <Trailer youtubeId={movie.youtube_id} title={movie.title} />
        <section aria-labelledby="review-heading">
          <h2 id="review-heading" className="review-heading">
            Our review
          </h2>
          <RankingBulbs ranking={movie.ranking} size="large" />
          <p className="review-text">{movie.admin_review || 'Not reviewed yet.'}</p>
        </section>
      </div>
    </>
  )
}