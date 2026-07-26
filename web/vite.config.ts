import { fileURLToPath } from 'node:url'

import { cloudflare } from '@cloudflare/vite-plugin'
import { tanstackStart } from '@tanstack/react-start/plugin/vite'
import viteReact from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// tabitha's TanStack Start frontend, compiled for Cloudflare Workers.
//
// Plugin order matters: `cloudflare()` must come first so it owns the
// `ssr` environment before Start configures it, and `viteReact()` must
// come last — `tanstackStart()` injects its own Babel/route-generation
// transforms that need to run before React's Fast Refresh transform.
export default defineConfig({
  resolve: {
    alias: {
      // Must mirror tsconfig.json's `paths`. tsconfig only teaches the
      // typechecker; without this the bundler can't resolve `~/…` and SSR
      // fails at request time with ERR_MODULE_NOT_FOUND.
      '~': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  plugins: [
    // viteEnvironment.name = 'ssr' compiles the server half against
    // workerd rather than Node, so a stray `node:fs` import fails the
    // build here instead of at deploy time.
    cloudflare({ viteEnvironment: { name: 'ssr' } }),
    tanstackStart(),
    viteReact(),
  ],
})
