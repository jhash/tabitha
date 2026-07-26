import { fileURLToPath } from 'node:url'

import { defineConfig } from 'vitest/config'

/**
 * Separate from vite.config.ts on purpose.
 *
 * The app config loads `@cloudflare/vite-plugin`, which would run these
 * specs inside workerd. The suites here cover pure porting logic — chord
 * grouping, transposition, header dedup — with no binding, no `fetch`,
 * and no Worker API in sight, so plain Node is both correct and much
 * faster. Anything that genuinely needs a Worker (the edge cache) belongs
 * in a `@cloudflare/vitest-pool-workers` suite instead, not here.
 */
export default defineConfig({
  resolve: {
    alias: { '~': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
})
