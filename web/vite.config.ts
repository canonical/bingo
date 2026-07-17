/// <reference types="vitest" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  // Relative asset paths so the built index.html works when served under any
  // path prefix (e.g. Traefik ingress-per-app's default path-based routing),
  // not just at the domain root. The server injects a matching <base href>
  // tag (see internal/server/server.go's indexHTMLWithBase) so these
  // relative references resolve against the app's externally visible base
  // path rather than wherever the current page happens to be nested.
  base: './',
  server: {
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/auth': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    globals: true,
    passWithNoTests: true,
    exclude: ['**/node_modules/**', '**/dist/**', 'tests/e2e/**'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
    },
  },
})
