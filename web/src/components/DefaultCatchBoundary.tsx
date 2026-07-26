import {
  ErrorComponent,
  Link,
  rootRouteId,
  useMatch,
  useRouter,
} from '@tanstack/react-router'
import type { ErrorComponentProps } from '@tanstack/react-router'

/**
 * Router-wide error boundary.
 *
 * The "retry" affordance is `router.invalidate()` rather than a page
 * reload: a transient origin failure usually just needs the loader run
 * again, and reloading would throw away the rest of the hydrated cache
 * to fix one route.
 */
export function DefaultCatchBoundary({ error }: ErrorComponentProps) {
  const router = useRouter()

  // Whether this boundary IS the root one determines where "Home" can
  // send you — from inside the root boundary there's no mounted router
  // tree left to navigate within, so it has to be a hard load.
  const isRoot = useMatch({
    strict: false,
    select: (state) => state.id === rootRouteId,
  })

  return (
    <main className="container">
      <h1>Something went wrong</h1>
      <ErrorComponent error={error} />
      <p>
        <button type="button" onClick={() => void router.invalidate()}>
          Try again
        </button>{' '}
        {isRoot ? (
          <a href="/">Back to the song list</a>
        ) : (
          <Link to="/">Back to the song list</Link>
        )}
      </p>
    </main>
  )
}
