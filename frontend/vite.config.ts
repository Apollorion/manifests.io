import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  publicDir: '../public',
  build: { manifest: true, sourcemap: true },
  server: {
    proxy: { '/api': 'http://localhost:8080' },
    watch: { ignored: ['**/dist-server/**', '**/prerender*/**'] },
  },
  test: { environment: 'jsdom', setupFiles: ['./src/test-setup.ts'], include: ['src/**/*.test.{ts,tsx}'] },
});
