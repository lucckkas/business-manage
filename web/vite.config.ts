import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  build: { outDir: '../internal/webui/dist', emptyOutDir: true },
  server: {
    host: '127.0.0.1',
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        // Preserve the browser's Host so the backend can validate Origin.
        changeOrigin: false,
      },
    },
  },
});
