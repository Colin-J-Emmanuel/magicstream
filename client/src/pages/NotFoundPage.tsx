import { Link } from 'react-router'

export function NotFoundPage() {
  return (
    <>
      <h1 className="page-title">Page not found</h1>
      <p className="lede">That address doesn't match any page on MagicStream.</p>
      <Link to="/">Go to all movies</Link>
    </>
  )
}