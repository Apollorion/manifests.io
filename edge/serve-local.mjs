import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { createHash } from 'node:crypto';
import { gunzipSync } from 'node:zlib';
import { createWorker } from './worker.mjs';

const directory = resolve(process.argv[2] || 'build/static');
const manifestBytes = await readFile(resolve(directory, 'manifest.json'));
const manifestHash = createHash('sha256').update(manifestBytes).digest('hex');
const pointer = { format: 1, release: JSON.parse(manifestBytes).release, manifest: `objects/${manifestHash}.json` };
const worker = createWorker(async address => {
  const url = new URL(address);
  if (url.origin !== 'https://storage.googleapis.com' || !url.pathname.startsWith('/local-static/')) throw new Error('Unexpected origin');
  const object = url.pathname.slice('/local-static/'.length);
  if (object === 'current.json') return Response.json(pointer);
  if (object === pointer.manifest) return new Response(manifestBytes);
  if (!/^objects\/[a-f0-9]{64}\.[a-z0-9.]+$/.test(object)) return new Response(null, { status: 404 });
  try {
    let body = await readFile(resolve(directory, object));
    if (body[0] === 0x1f && body[1] === 0x8b) body = gunzipSync(body);
    return new Response(body, { headers: { 'CF-Cache-Status': 'HIT' } });
  } catch (error) {
    if (error.code !== 'ENOENT') throw error;
    return new Response(null, { status: 404 });
  }
});

const server = createServer(async (incoming, outgoing) => {
  try {
    const request = new Request(`http://localhost${incoming.url}`, { method: incoming.method, headers: incoming.headers });
    const response = await worker.fetch(request, { STORAGE_BUCKET: 'local-static' });
    outgoing.writeHead(response.status, Object.fromEntries(response.headers));
    if (response.body) for await (const chunk of response.body) outgoing.write(chunk);
    outgoing.end();
  } catch {
    outgoing.writeHead(500);
    outgoing.end('Local static adapter failed');
  }
});
server.listen(Number(process.env.PORT || 8080), '127.0.0.1', () => console.log(`Static Worker serving ${directory} on ${server.address().port}`));
for (const signal of ['SIGTERM', 'SIGINT']) process.on(signal, () => server.close());
