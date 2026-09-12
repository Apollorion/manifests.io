import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { createHash } from 'node:crypto';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const require = createRequire(new URL('../frontend/package.json', import.meta.url));
const { convertV4MiniflareOptions, Miniflare } = require('miniflare');
const directory = resolve(process.argv[2] || 'build/static');
const manifestBytes = await readFile(resolve(directory, 'manifest.json'));
const manifestHash = createHash('sha256').update(manifestBytes).digest('hex');
const pointer = { format: 1, release: JSON.parse(manifestBytes).release, manifest: `objects/${manifestHash}.json` };
const inventory = JSON.parse(await readFile(resolve(directory, 'upload-manifest.json'), 'utf8'));
const objects = new Map(inventory.objects.map(ref => [ref.object, ref]));
const runtime = new Miniflare(convertV4MiniflareOptions({
  rootPath: fileURLToPath(new URL('../', import.meta.url)),
  modules: true,
  scriptPath: fileURLToPath(new URL('./worker.mjs', import.meta.url)),
  compatibilityDate: '2026-09-11',
  cf: false,
  host: '127.0.0.1',
  port: Number(process.env.PORT || 8080),
  bindings: { STORAGE_BUCKET: 'local-static' },
  // Raw handlers avoid Miniflare recompressing stored gzip bytes.
  outboundService: { async node(request, response) {
    function send(body, status = 200, headers = {}) {
      response.writeHead(status, headers);
      response.end(body);
    }
    const url = new URL(request.url, `https://${request.headers.host}`);
    if (url.origin !== 'https://storage.googleapis.com' || !url.pathname.startsWith('/local-static/')) throw new Error('Unexpected origin');
    const object = url.pathname.slice('/local-static/'.length);
    if (object === 'current.json') return send(JSON.stringify(pointer), 200, { 'Content-Type': 'application/json' });
    if (object === pointer.manifest || object === `releases/${pointer.release}.json`) return send(manifestBytes, 200, { 'Content-Type': 'application/json' });
    if (!/^objects\/[a-f0-9]{64}\.[a-z0-9.]+$/.test(object)) return send(null, 404);
    const ref = objects.get(object);
    if (!ref) return send(null, 404);
    try {
      const body = await readFile(resolve(directory, object));
      return send(body, 200, {
        'Content-Type': ref.contentType,
        'Content-Length': String(body.length),
        ...(ref.contentEncoding ? { 'Content-Encoding': ref.contentEncoding } : {}),
        'CF-Cache-Status': 'HIT',
      });
    } catch (error) {
      if (error.code !== 'ENOENT') throw error;
      return send(null, 404);
    }
  } },
}));

const address = await runtime.ready;
console.log(`Static Worker serving ${directory} on ${address.port}`);
for (const signal of ['SIGTERM', 'SIGINT']) process.on(signal, () => runtime.dispose());
