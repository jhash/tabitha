import { Link } from '@tanstack/react-router'

/**
 * `data` is unused but declared because the router passes it to a
 * `notFoundComponent`, and without a property in common TypeScript's
 * weak-type check rejects this component for that slot.
 */
export function NotFound({
  children,
}: {
  children?: React.ReactNode
  data?: unknown
}) {
  return (
    <main className="container">
      <h1>Not found</h1>
      <p className="no-content">
        {children ?? "That page doesn't exist — it may have been renamed."}
      </p>
      <p>
        <Link to="/">Back to the song list</Link>
      </p>
    </main>
  )
}
