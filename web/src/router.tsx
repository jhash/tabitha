import { QueryClient } from '@tanstack/react-query'
import { createRouter as createTanStackRouter } from '@tanstack/react-router'
import { setupRouterSsrQueryIntegration } from '@tanstack/react-router-ssr-query'

import { DefaultCatchBoundary } from '~/components/DefaultCatchBoundary'
import { NotFound } from '~/components/NotFound'
import { routeTree } from './routeTree.gen'

/**
 * Builds the router (and its QueryClient) once per request on the server,
 * once per page load in the browser.
 *
 * Per-request construction is not optional on the server: a QueryClient
 * hoisted to module scope would live for the whole Worker isolate, which
 * outlives a single request and is shared by every concurrent one — so
 * one viewer's cached data would be served to the next. Building it here,
 * inside `getRouter`, keeps each request's cache private to that request.
 */
export function getRouter() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        /**
         * Non-zero so the client doesn't immediately refetch everything
         * the server just dehydrated into the page. Individual queries
         * override this (see lib/queries.ts); this is only the floor for
         * anything that doesn't.
         */
        staleTime: 30 * 1000,
        /**
         * Errors thrown during SSR must reach the router's error boundary
         * rather than being retried three times inside the Worker — a
         * retry loop just burns the request's CPU budget and delays the
         * error page.
         */
        retry: (failureCount) => (import.meta.env.SSR ? false : failureCount < 2),
      },
    },
  })

  const router = createTanStackRouter({
    routeTree,
    // Exposed to every loader as `context.queryClient` — the object the
    // ensureQueryData/useSuspenseQuery pattern hangs off of.
    context: { queryClient },
    /**
     * Render the pending state only if a load actually drags. Below this,
     * a fast navigation would otherwise flash a spinner and back — which
     * reads as jank, not as feedback.
     */
    defaultPendingMs: 200,
    defaultPendingMinMs: 300,
    defaultPreload: 'intent',
    // The loader's job is filling the QueryClient; React Query's own
    // staleTime decides freshness. A second TTL on the router layer would
    // just be a competing cache with different rules.
    defaultPreloadStaleTime: 0,
    defaultErrorComponent: DefaultCatchBoundary,
    defaultNotFoundComponent: NotFound,
    scrollRestoration: true,
  })

  // Dehydrates the QueryClient into the SSR payload and rehydrates it on
  // the client, so a query resolved during SSR is already populated on
  // mount instead of refetching. Without this the loaders would still
  // run server-side, but every result would be thrown away at hydration.
  setupRouterSsrQueryIntegration({ router, queryClient })

  return router
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof getRouter>
  }
}
