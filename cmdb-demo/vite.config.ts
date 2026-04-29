import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    // First-paint budget: keep vendor + app chunks each under ~500kB. The
    // heavy libs below only show up on specific pages, so splitting them
    // out means the dashboard does not pay for them. Function form is
    // required by rolldown (vite 8+).
    rollupOptions: {
      output: {
        manualChunks(id: string) {
          if (id.includes('node_modules')) {
            // elkjs is ~1.4MB and only used by topology/3D pages. Its
            // dynamic imports already split it from the main bundle; this
            // entry just gives the chunk a stable name.
            if (id.includes('/elkjs/')) return 'elk'
            // @e965/xlsx is ~500KB and only used by Excel import/export.
            if (id.includes('/@e965/xlsx/') || id.includes('/xlsx/')) return 'xlsx'
            // Keep React core in its own chunk so it caches across deploys.
            if (
              id.includes('/react/') ||
              id.includes('/react-dom/') ||
              id.includes('/react-router-dom/') ||
              id.includes('/react-router/')
            ) {
              return 'vendor-react'
            }
          }
          return undefined
        },
      },
    },
    // elkjs is a 1.4MB graph-layout vendor lib that is already isolated
    // into its own lazy chunk (only loaded by the topology page). It can't
    // be split further. Lift the warning ceiling above its size so genuine
    // regressions in app code still trigger.
    chunkSizeWarningLimit: 1500,
  },
  server: {
    port: 5175,
    host: '0.0.0.0',
    proxy: {
      '/api/v1/ingestion': {
        target: 'http://localhost:8081',
        changeOrigin: true,
        rewrite: (path: string) => path.replace('/api/v1/ingestion', ''),
      },
      '/api/v1/ws': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        ws: true,
      },
      '/api/v1': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
})
