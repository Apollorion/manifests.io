import { build } from 'esbuild';
import { createRequire } from 'node:module';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const root = fileURLToPath(new URL('../', import.meta.url));

// Avoid initializing React's streaming APIs in the synchronous Go runtime.
const renderer = resolve(dirname(require.resolve('react-dom/server.browser')), 'cjs/react-dom-server-legacy.browser.production.js');
await build({
  absWorkingDir: root,
  entryPoints: ['src/entry-runtime.ts'],
  outfile: 'dist-render/renderer.js',
  bundle: true,
  format: 'iife',
  globalName: 'ManifestsRenderer',
  platform: 'browser',
  target: 'es2015',
  jsx: 'automatic',
  define: { 'process.env.NODE_ENV': '"production"', global: 'globalThis' },
  alias: { 'react-dom/server': renderer },
  minify: true,
});
