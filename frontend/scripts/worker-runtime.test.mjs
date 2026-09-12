import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import test from 'node:test';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';
import { convertV4MiniflareOptions, Miniflare } from 'miniflare';

const release = 'a'.repeat(40);
const html = '<!doctype html><html><head><title>Pod schema</title></head><body>Static Pod</body></html>';
const page = { item: 'kubernetes', version: '1', resource: 'Pod', title: 'Pod', rows: [] };

function fixture(t, redirectObject) {
  const objects = new Map();
  const calls = [];
  function object(value, contentType = 'application/json', compressed = true) {
    const plain = Buffer.from(typeof value === 'string' ? value : JSON.stringify(value));
    const body = compressed ? gzipSync(plain) : plain;
    const hash = createHash('sha256').update(body).digest('hex');
    const ref = { object: `objects/${hash}.json`, contentType, ...(compressed ? { contentEncoding: 'gzip' } : {}) };
    objects.set(ref.object, { body, headers: { 'Content-Type': contentType, ...(compressed ? { 'Content-Encoding': 'gzip' } : {}) } });
    return ref;
  }
  const content = { html: object(html, 'text/html; charset=utf-8'), json: object(page) };
  const errors = { html: object('Not found', 'text/html'), json: object({ error: 'Not found' }) };
  const graph = object({ index: content, definitions: content.json, resources: { Pod: 'Pod#' }, nodes: { 'Pod#': { ...content, edges: {} } } });
  const manifest = object({ format: 1, release, default: '/kubernetes/1', catalog: content.json, documents: { 'kubernetes/1': graph }, files: {}, errors: { '400': errors, '404': errors } }, 'application/json', false);
  objects.set('current.json', { body: JSON.stringify({ format: 1, release, manifest: manifest.object }), headers: { 'Content-Type': 'application/json' } });
  const runtime = new Miniflare(convertV4MiniflareOptions({
    rootPath: fileURLToPath(new URL('../../', import.meta.url)),
    cf: false,
    modules: true,
    scriptPath: fileURLToPath(new URL('../../edge/worker.mjs', import.meta.url)),
    compatibilityDate: '2026-09-11',
    bindings: { STORAGE_BUCKET: 'test-bucket' },
    // Raw handlers avoid Miniflare recompressing stored gzip bytes.
    outboundService: { node(request, response) {
      const url = new URL(request.url, `https://${request.headers.host}`);
      calls.push(url.href);
      assert.equal(url.origin, 'https://storage.googleapis.com');
      assert(url.pathname.startsWith('/test-bucket/'));
      const name = url.pathname.slice('/test-bucket/'.length);
      if (name === (redirectObject === 'content' ? content.html.object : redirectObject)) {
        response.writeHead(302, { Location: 'https://redirect.invalid/forbidden' });
        response.end();
        return;
      }
      const stored = objects.get(name);
      response.writeHead(stored ? 200 : 404, stored?.headers);
      response.end(stored?.body ?? 'Missing');
    } },
  }));
  t.after(() => runtime.dispose());
  return { calls, contentObject: content.html.object, request: (path, init) => runtime.dispatchFetch(`https://www.manifests.io${path}`, init) };
}

test('workerd decodes gzip graph metadata and streams HTML and JSON through real fetch', async t => {
  const f = fixture(t);
  const response = await f.request('/kubernetes/1/Pod');
  assert.equal(response.status, 200);
  assert.equal(response.headers.get('Content-Type'), 'text/html; charset=utf-8');
  assert.equal(response.headers.get('X-Manifests-Release'), release);
  assert.equal(response.headers.get('Cache-Control'), 'public, max-age=0, s-maxage=604800, must-revalidate');
  assert.equal(await response.text(), html);
  const api = await f.request('/api/page?item=kubernetes&version=1&resource=Pod&path=Context&trail=ignored');
  assert.equal(api.status, 200);
  assert.deepEqual(await api.json(), page);
  const head = await f.request('/kubernetes/1/Pod', { method: 'HEAD' });
  assert.equal(head.status, 200);
  assert.equal(await head.text(), '');
  assert.equal(head.headers.get('ETag'), response.headers.get('ETag'));
  const conditional = await f.request('/kubernetes/1/Pod', { headers: { 'If-None-Match': response.headers.get('ETag') } });
  assert.equal(conditional.status, 304);
  assert.equal(f.calls.filter(url => url.endsWith('/current.json')).length, 1);
});

test('workerd retains missing release metadata briefly without repeated bucket reads', async t => {
  const f = fixture(t);
  const path = `/releases/${'b'.repeat(40)}/assets/missing.js`;
  for (let i = 0; i < 2; i++) {
    const response = await f.request(path);
    assert.equal(response.status, 404);
    await response.text();
  }
  assert.equal(f.calls.length, 1);
});

for (const target of ['current.json', 'content']) {
  test(`workerd refuses ${target} origin redirects without following them`, async t => {
    const f = fixture(t, target);
    const response = await f.request('/kubernetes/1/Pod');
    assert.equal(response.status, 503);
    assert.equal(response.headers.get('Cache-Control'), 'no-store');
    await response.text();
    assert.equal(f.calls.at(-1), `https://storage.googleapis.com/test-bucket/${target === 'content' ? f.contentObject : target}`);
    assert(f.calls.every(url => new URL(url).origin === 'https://storage.googleapis.com'));
  });
}
