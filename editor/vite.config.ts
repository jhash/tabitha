import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Build-time-only tool: the compiled bundle is copied straight into Go's
// static/ directory and served by the existing file server + content-hash
// asset-versioning system (internal/web/assets.go) — fixed filenames here,
// no Node.js needed at runtime in production.
export default defineConfig({
  plugins: [react()],
  server: {
    // `npm run dev` proxies API calls to the real Go server (`air`/`go run
    // . serve` on :8080) so the standalone dev page can load/save real data.
    proxy: {
      '/songs': 'http://localhost:8080',
    },
  },
  build: {
    outDir: '../static',
    emptyOutDir: false,
    rollupOptions: {
      input: 'src/main.tsx',
      output: {
        entryFileNames: 'js/editor.js',
        chunkFileNames: 'js/editor-[name].js',
        assetFileNames: (assetInfo) =>
          assetInfo.names?.[0]?.endsWith('.css') ? 'css/editor.css' : 'assets/[name][extname]',
      },
    },
  },
})
