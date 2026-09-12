import assert from 'node:assert/strict';
import test from 'node:test';
import { createWorker, parseRoute, selectNode } from './worker.mjs';

const ref = number => ({ object: `objects/${String(number).padStart(64, '0')}.json`, contentType: 'application/json' });
const release = 'a'.repeat(40);
const document = {
  index: { html: ref(1), json: ref(2) }, definitions: ref(3), resources: { Root: 'root', Alias: 'child' },
  nodes: {
    root: { html: ref(4), json: ref(5), edges: { '/properties/spec': 'child', '/properties/a~1b': 'child' } },
    child: { html: ref(6), json: ref(7), edges: { '/items': 'root' }, oneOf: [{ title: '', node: 'root' }] },
  },
};
const manifest = {
  format: 1, release, default: '/kubernetes/1', catalog: ref(8), documents: { 'kubernetes/1': ref(9) }, files: { '/assets/site.js': { ...ref(10), contentType: 'text/javascript' } },
  errors: { '400': { html: ref(11), json: ref(12) }, '404': { html: ref(13), json: ref(14) } },
};
manifest.files['/context-guard.js'] = { ...ref(17), contentType: 'text/javascript' };

function fixture() {
  const calls = [];
  let clock = 0;
  let current = { format: 1, release, manifest: ref(15).object };
  const manifests = new Map([[ref(15).object, manifest], [`releases/${release}.json`, manifest]]);
  let fail = '';
  const worker = createWorker(async (address, options) => {
    const object = new URL(address).pathname.split('/').slice(2).join('/');
    calls.push({ object, options });
    assert.equal(new URL(address).origin, 'https://storage.googleapis.com');
    if (object === fail) return new Response('Unavailable', { status: 503 });
    const body = object === 'current.json' ? current : manifests.has(object) ? manifests.get(object) : object === ref(9).object ? document : { object };
    return Response.json(body, { headers: { 'CF-Cache-Status': 'HIT' } });
  }, () => clock);
  return {
    calls,
    request: (route, init) => worker.fetch(new Request(`https://www.manifests.io${route}`, init), { STORAGE_BUCKET: 'test-bucket' }),
    advance: ms => { clock += ms; },
    release: (value, mismatch = false) => {
      current = { ...current, release: value, manifest: ref(16).object };
      manifests.set(ref(16).object, { ...manifest, release: mismatch ? release : value });
      manifests.set(`releases/${value}.json`, { ...manifest, release: value });
    },
    fail: value => { fail = value; },
  };
}

test('semantic selectors follow a finite graph, including aliases and cycles', () => {
  const select = path => selectNode(document, parseRoute(new URL(path, 'https://site.invalid')));
  assert.equal(select('/kubernetes/1/Root?pointer=/properties/spec/items'), document.nodes.root);
  assert.equal(select('/kubernetes/1/Alias'), document.nodes.child);
  assert.equal(select('/kubernetes/1/Root?pointer=/properties/a~1b'), document.nodes.child);
  assert.equal(select('/kubernetes/1/Root?key=spec'), document.nodes.root);
  assert.throws(() => select('/kubernetes/1/Root?pointer=/properties/a~2b'), { status: 400 });
  assert.throws(() => select('/kubernetes/1/Root?pointer=/properties/missing'), { status: 404 });
  assert.throws(() => select('/kubernetes/1/Root?pointer=' + '/properties/spec/items'.repeat(44)), { status: 400 });
});

test('traversal and tracking parameters never affect object selection', async () => {
  const f = fixture();
  const a = await f.request('/kubernetes/1/Root?path=First&trail=invalid&utm_source=a');
  const b = await f.request('/kubernetes/1/Root?linked=Second&trail=null&utm_source=b');
  assert.equal(await a.text(), await b.text());
  assert.equal(a.headers.get('ETag'), b.headers.get('ETag'));
  assert.equal(f.calls.filter(call => call.object === 'current.json').length, 1);
  assert(f.calls.every(call => !JSON.stringify(call.options).includes('trail')));
});

test('unknown scanner URLs share a single static 404 object', async () => {
  const f = fixture();
  for (const route of ['/api/uploads/apimap', '/api/random', '/kubernetes/1/missing', '/assets/file.js.map', '/__proto__/1/Root']) {
    const response = await f.request(route);
    assert.equal(response.status, 404, route);
    assert.equal(response.headers.get('Cache-Control'), 'public, max-age=0, s-maxage=604800, must-revalidate');
  }
  assert(f.calls.every(call => call.object === 'current.json' || /^objects\/[a-f0-9]{64}\.json$/.test(call.object)));
});

test('methods and oversize URLs are rejected without bucket reads', async () => {
  const f = fixture();
  assert.equal((await f.request('/api/catalog', { method: 'POST' })).status, 405);
  assert.equal((await f.request('/' + 'x'.repeat(8192))).status, 414);
  assert.equal(f.calls.length, 0);
});

test('malformed selectors stay 400 and never select another resource', async () => {
  const f = fixture();
  for (const route of ['/api/page?item=kubernetes&item=flux&version=1', '/kubernetes/1/Root?pointer=%ZZ', '/api/definitions?item=kubernetes&version=1&resource=Root', '/kubernetes/1/Root?oneOf=string', '/kubernetes/1/Root?pointer=/oneOf/no']) {
    assert.equal((await f.request(route)).status, 400, route);
  }
});

test('HEAD, conditionals, redirects, and immutable assets preserve HTTP semantics', async () => {
  const f = fixture();
  const root = await f.request('/');
  assert.equal(root.status, 307);
  assert.equal(root.headers.get('Location'), '/kubernetes/1');
  const get = await f.request('/kubernetes/1/Root');
  const head = await f.request('/kubernetes/1/Root', { method: 'HEAD' });
  assert.equal(await head.text(), '');
  assert.equal(head.headers.get('ETag'), get.headers.get('ETag'));
  const before = f.calls.length;
  const conditional = await f.request('/kubernetes/1/Root', { headers: { 'If-None-Match': 'W/' + get.headers.get('ETag') } });
  assert.equal(conditional.status, 304);
  assert.equal(f.calls.length, before);
  assert.equal((await f.request('/assets/site.js')).headers.get('Cache-Control'), 'public, max-age=31536000, immutable');
});

test('only the release pointer expires; requests never mix release metadata', async () => {
  const f = fixture();
  assert.equal((await f.request('/kubernetes/1/Root')).headers.get('X-Manifests-Release'), release);
  f.release('b'.repeat(40));
  f.advance(61000);
  assert.equal((await f.request('/kubernetes/1/Root')).headers.get('X-Manifests-Release'), 'b'.repeat(40));
  assert.equal(f.calls.filter(call => call.object === 'current.json').length, 2);
  assert.equal(f.calls.filter(call => call.object === ref(9).object).length, 1);
});

test('missing published objects fail closed with no Cloud Run fallback', async () => {
  const f = fixture();
  f.fail(ref(4).object);
  const response = await f.request('/kubernetes/1/Root');
  assert.equal(response.status, 503);
  assert.equal(response.headers.get('Cache-Control'), 'no-store');
});

test('a mislabeled release pointer cannot serve content from another release', async () => {
  const f = fixture();
  f.release('b'.repeat(40), true);
  assert.equal((await f.request('/kubernetes/1/Root')).status, 503);
});

test('encoded API routes preserve JSON errors', async () => {
  const f = fixture();
  for (const route of ['/%61pi/page?item=kubernetes&version=1&resource=missing', '/api%2Fpage?item=kubernetes&version=1&resource=missing']) {
    const response = await f.request(route);
    assert.equal(response.status, 404);
    assert.equal((await response.json()).object, manifest.errors['404'].json.object);
  }
  const invalid = await f.request('/%61pi/page?item=kubernetes');
  assert.equal(invalid.status, 400);
  assert.equal((await invalid.json()).object, manifest.errors['400'].json.object);
});

test('old HTML can fetch its release assets after the current release changes', async () => {
  const f = fixture();
  f.release('b'.repeat(40));
  const old = await f.request(`/releases/${release}/assets/site.js`);
  assert.equal(old.status, 200);
  assert.equal(old.headers.get('X-Manifests-Release'), release);
  assert.equal((await old.json()).object, ref(10).object);
  assert.equal(f.calls.filter(call => call.object === 'current.json').length, 0);
  assert.equal((await f.request(`/releases/${release}/assets/site.js.map`)).status, 404);
  assert.equal((await f.request(`/releases/${release}/context-guard.js`)).status, 200);
});
