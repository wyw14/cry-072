import vue from '@vitejs/plugin-vue'

const apiOrigin = process.env.CRY072_API_ORIGIN || 'http://localhost:8080'

export default {
  plugins: [vue()],
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    proxy: {
      '/api': { target: apiOrigin, changeOrigin: false },
      '/healthz': { target: apiOrigin, changeOrigin: false },
      '/readyz': { target: apiOrigin, changeOrigin: false },
    },
  },
  build: { outDir: 'dist', sourcemap: false, emptyOutDir: true },
  test: { environment: 'jsdom', restoreMocks: true, clearMocks: true },
}
