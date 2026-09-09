import { build } from 'esbuild';
import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const root = fileURLToPath(new URL('../', import.meta.url));

// Avoid initializing React's streaming APIs in the synchronous Go runtime.
const renderer = resolve(dirname(require.resolve('react-dom/server.browser')), 'cjs/react-dom-server-legacy.browser.production.js');
const source = await readFile(renderer, 'utf8');
const start = source.indexOf('function renderToStringImpl(');
const end = source.indexOf('\nexports.renderToStaticMarkup', start);
let collector = source.slice(start, end);
if (start < 0 || end < 0 || (collector.match(/\bresult\b/g) || []).length !== 3) {
  throw new Error('React output collector changed; review the Goja adapter');
}
// Goja copies the growing Unicode string on every append; join chunks once.
for (const [before, after] of [
  ['result = ""', 'result = []'],
  ['null !== chunk && (result += chunk);', 'null !== chunk && result.push(chunk);'],
  ['return result;', 'return result.join("");'],
]) {
  if (collector.split(before).length !== 2) {
    throw new Error('React output collector changed; review the Goja adapter');
  }
  collector = collector.replace(before, after);
}
const adaptedRenderer = source.slice(0, start) + collector + source.slice(end);
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
  plugins: [{
    name: 'goja-react-output',
    setup(builder) {
      builder.onLoad({ filter: /react-dom-server-legacy\.browser\.production\.js$/ }, (args) => {
        if (args.path !== renderer) return;
        return { contents: adaptedRenderer, loader: 'js', resolveDir: dirname(renderer) };
      });
    },
  }],
  minify: true,
});
