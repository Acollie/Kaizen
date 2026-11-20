import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

// vite.config.ts runs in Node; declare the one Node global it uses rather than pulling
// Node's types into the browser code's type environment
declare const process: { env: Record<string, string | undefined> };

// The build is embedded in the Go binary (pkg/reviewui, //go:embed static) and served
// under /static/. File names are stable (no content hashes) so rebuilding produces a
// small, reviewable diff; the server sends Cache-Control: no-cache for these assets.
export default defineConfig({
  plugins: [react()],
  base: '/static/',
  build: {
    outDir: '../../pkg/reviewui/static',
    emptyOutDir: true,
    target: 'es2022',
    sourcemap: false,
    chunkSizeWarningLimit: 800,
    rollupOptions: {
      output: {
        entryFileNames: 'assets/app.js',
        chunkFileNames: 'assets/[name].js',
        assetFileNames: 'assets/[name][extname]',
      },
    },
  },
  server: {
    // `npm run dev` proxies the API to a running `kaizen review` / `kaizen review-demo`
    // (set KAIZEN_URL=http://127.0.0.1:PORT and open the dev page with the same ?t= token)
    proxy: { '/api': process.env.KAIZEN_URL ?? 'http://127.0.0.1:7799' },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    include: ['src/**/*.test.{ts,tsx}'],
  },
});
