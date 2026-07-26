import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'

import { viewerQueryOptions } from '~/lib/queries'

/**
 * Port of the site chrome in internal/web/layout.go's `page()`.
 *
 * `useQuery`, not `useSuspenseQuery`: the header renders on every route,
 * including ones that never load a viewer, and suspending here would
 * block the whole document on a query the page may not need. An
 * unresolved viewer just renders the signed-out header.
 */
export function SiteHeader() {
  const { data: viewer } = useQuery(viewerQueryOptions())

  return (
    <header className="site-header">
      <div className="site-header-inner container-wide">
        <Link className="site-title" to="/">
          tabitha
        </Link>
        <div className="site-header-right">
          {/* Populated by the offline sync script — stays hidden in an
              ordinary browser tab, shown in an installed PWA. */}
          <span id="offline-status" className="offline-status" hidden />
          {viewer?.isSuperadmin ? (
            <a className="site-admin-link" href="/admin">
              Admin
            </a>
          ) : null}
        </div>
      </div>
    </header>
  )
}
