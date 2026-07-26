import type { QueryClient } from '@tanstack/react-query'
import {
  HeadContent,
  Scripts,
  createRootRouteWithContext,
} from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { DefaultCatchBoundary } from '~/components/DefaultCatchBoundary'
import { NotFound } from '~/components/NotFound'
import { SiteHeader } from '~/components/SiteHeader'
import appCss from '~/styles/app.css?url'
import resetCss from '~/styles/reset.css?url'

/**
 * Port of internal/web/layout.go's `page()`.
 *
 * Two things the Go version did by hand now come for free and are
 * deliberately not reimplemented:
 *
 *   - Asset cache-busting. `internal/web/assets.go` hashed each static
 *     file at startup and appended `?v=`; Vite fingerprints filenames at
 *     build time, which is the same guarantee with no runtime work.
 *   - hx-boost. htmx boosted plain links into partial navigations; the
 *     router does that natively, so the attribute has no successor here.
 */

export interface RouterContext {
  queryClient: QueryClient
}

export const Route = createRootRouteWithContext<RouterContext>()({
  head: () => ({
    meta: [
      { charSet: 'utf-8' },
      { name: 'viewport', content: 'width=device-width, initial-scale=1' },
      { name: 'theme-color', content: '#7e14ff' },
      { name: 'apple-mobile-web-app-capable', content: 'yes' },
      { property: 'og:type', content: 'website' },
    ],
    links: [
      { rel: 'stylesheet', href: resetCss },
      { rel: 'stylesheet', href: appCss },
      { rel: 'manifest', href: '/manifest.webmanifest' },
      { rel: 'apple-touch-icon', href: '/icons/apple-touch-icon.png' },
      { rel: 'icon', href: '/icons/icon-512.png', type: 'image/png' },
      // Matches the Go layout's font preload — self-hosted, no CDN, no
      // FOUT. crossOrigin is required even same-origin for fonts, or the
      // preload is discarded and fetched a second time.
      {
        rel: 'preload',
        href: '/fonts/Lora-Variable.woff2',
        as: 'font',
        type: 'font/woff2',
        crossOrigin: 'anonymous',
      },
    ],
  }),
  errorComponent: DefaultCatchBoundary,
  notFoundComponent: NotFound,
  shellComponent: RootDocument,
})

/**
 * The shell is rendered once per document and deliberately sits outside
 * the router's error boundary — if a route throws, the boundary swaps out
 * `<Outlet />` while `<html>`, the stylesheets, and `<Scripts />` survive,
 * so an error page is still styled and still hydrates.
 */
function RootDocument({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <head>
        {/* Emits everything from every matched route's `head()` — titles,
            meta, links — in route order, so a child can override a parent. */}
        <HeadContent />
      </head>
      <body>
        <SiteHeader />
        {children}
        {/* Must be the last thing in <body>: it carries the dehydrated
            QueryClient state, and hydration fails if the markup above it
            isn't complete when it runs. */}
        <Scripts />
      </body>
    </html>
  )
}
