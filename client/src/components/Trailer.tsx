// A YouTube video ID is exactly 11 characters from this set. Anything else is refused
// before it can become part of a URL.
const YOUTUBE_ID = /^[A-Za-z0-9_-]{11}$/

export function Trailer({ youtubeId, title }: { youtubeId: string; title: string }) {
  if (!YOUTUBE_ID.test(youtubeId)) {
    return (
      <div className="trailer-missing">
        <p>No trailer for this movie yet.</p>
      </div>
    )
  }

  return (
    <iframe
      className="trailer"
      src={`https://www.youtube-nocookie.com/embed/${youtubeId}`}
      title={`${title} trailer`}
      loading="lazy"
      allow="encrypted-media; picture-in-picture; fullscreen"
      referrerPolicy="strict-origin-when-cross-origin"
      sandbox="allow-scripts allow-same-origin allow-presentation allow-popups"
    />
  )
}