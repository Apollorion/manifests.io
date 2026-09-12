import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtemp, mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { test } from 'node:test';
import { gunzipSync } from 'node:zlib';
import { pathToFileURL } from 'node:url';
import { exportStatic } from './export-static.mjs';
import { StaticRenderPool } from './static-render-pool.mjs';

const template = '<html><head><!--page-head--></head><body><!--app-html--><!--page-data--></body></html>';
const release = 'a'.repeat(40);
const basePage = { item: 'example', version: '1', title: 'Example', resources: [], catalog: [], canonical: '/example/1' };
const pageRecord = (page, status = 200) => ({ kind: 'page', status, data: JSON.stringify(page).replaceAll('<', '\\u003c'), head: `<title>${page.title}</title>` });
const errorRecords = () => [400, 404].map(status => pageRecord({ ...basePage, error: 'Unavailable', canonical: '/' }, status));

function records() {
  return [
    { kind: 'root', site: 'https://docs.example', default: '/example/1', catalog: [] },
    ...errorRecords(),
    { kind: 'document', graph: { item: 'example', version: '1', resources: { A: 'A#', Alias: 'A#' }, nodes: { 'A#': { canonical: '/example/1/A', edges: { '/properties/again': 'A#' } } } } },
    pageRecord(basePage),
    pageRecord({ ...basePage, resource: 'A', title: 'A', description: '</script><script>private</script>', canonical: '/example/1/A' }),
    { kind: 'definitions', data: [{ name: 'A', resource: 'A', href: '/example/1/A' }] },
    ...errorRecords(),
    { kind: 'document-end' },
    { kind: 'file', path: '/robots.txt', contentType: 'text/plain; charset=utf-8', data: 'User-agent: *\nDisallow:\n' },
    { kind: 'complete' },
  ];
}

async function fixture(t) {
  const directory = await mkdtemp(join(tmpdir(), 'manifests-static-'));
  t.after(() => rm(directory, { recursive: true, force: true }));
  const webDir = join(directory, 'web');
  await mkdir(join(webDir, 'assets'), { recursive: true });
  await writeFile(join(webDir, 'index.html'), template);
  await writeFile(join(webDir, 'assets/main-abcd1234.js'), 'export const ready=true;');
  await writeFile(join(webDir, 'assets/main-abcd1234.js.map'), 'private source');
  await writeFile(join(webDir, 'manifest.json'), '{"name":"Manifests"}');
  return { target: join(directory, 'static'), webDir, template, release, render: page => `<main><h1>${page.title}</h1></main>` };
}

async function readObject(target, ref) {
  const stored = await readFile(join(target, ref.object));
  assert.equal(createHash('sha256').update(stored).digest('hex'), ref.object.split('/')[1].split('.')[0]);
  return (ref.contentEncoding === 'gzip' ? gunzipSync(stored) : stored).toString();
}

test('complete static releases preserve pages, graph targets, APIs, assets and safe embedded data', async t => {
  const options = await fixture(t);
  const stats = await exportStatic({ ...options, records: records() });
  assert.equal(stats.pages, 2);
  assert.equal(stats.documents, 1);
  const manifest = JSON.parse(await readFile(join(options.target, 'manifest.json')));
  assert.equal(manifest.release, release);
  assert.equal(manifest.default, '/example/1');
  const graph = JSON.parse(await readObject(options.target, manifest.documents['example/1']));
  assert.equal(graph.resources.Alias, 'A#');
  const html = await readObject(options.target, graph.nodes['A#'].html);
  assert(html.includes('<title>A</title>') && html.includes('<h1>A</h1>'));
  assert(!html.includes('<!--') && !html.includes('</script><script>private'));
  const embedded = JSON.parse(html.match(/<script id="__PAGE_DATA__" type="application\/json">(.*?)<\/script>/s)[1]);
  assert.deepEqual(embedded, JSON.parse(await readObject(options.target, graph.nodes['A#'].json)));
  assert.equal(JSON.parse(await readObject(options.target, graph.definitions))[0].name, 'A');
  assert(manifest.files['/assets/main-abcd1234.js']);
  assert(!manifest.files['/assets/main-abcd1234.js.map']);
  assert.equal(JSON.parse(await readObject(options.target, manifest.files['/manifest.json'])).name, 'Manifests');
  for (const status of [400, 404]) assert(JSON.parse(await readObject(options.target, graph.errors[status].json)).error);
  const inventory = JSON.parse(await readFile(join(options.target, 'upload-manifest.json')));
  for (const ref of inventory.objects) {
    assert.equal((await readFile(join(options.target, ref.object))).length, ref.bytes);
    await readObject(options.target, ref);
  }
  const first = await readFile(join(options.target, 'manifest.json'), 'utf8');
  await exportStatic({ ...options, records: records() });
  assert.equal(await readFile(join(options.target, 'manifest.json'), 'utf8'), first, 'Repeated exports changed content identities');
});

test('incomplete streams preserve the previous complete release', async t => {
  const options = await fixture(t);
  await exportStatic({ ...options, records: records() });
  const previous = await readFile(join(options.target, 'manifest.json'), 'utf8');
  await assert.rejects(exportStatic({ ...options, records: records().slice(0, -1) }), /before completion/);
  assert.equal(await readFile(join(options.target, 'manifest.json'), 'utf8'), previous);
});

test('dangling graph targets fail the export', async t => {
  const options = await fixture(t);
  const input = records();
  input.find(record => record.kind === 'document').graph.nodes['A#'].edges['/items'] = 'missing#';
  await assert.rejects(exportStatic({ ...options, records: input }), /Static edge target is absent/);
});

test('bounded parallel workers produce the same release as serial rendering', async t => {
  const options = await fixture(t);
  await exportStatic({ ...options, records: records() });
  const serial = await readFile(join(options.target, 'manifest.json'), 'utf8');
  const renderer = join(options.target, '..', 'missing-renderer.mjs');
  const pool = new StaticRenderPool({ template, rendererURL: pathToFileURL(renderer).href, size: 4 });
  try {
    await assert.rejects(pool.render(records()[1]));
  } finally {
    await pool.close();
  }
  const modulePath = join(options.target, '..', 'renderer.mjs');
  await writeFile(modulePath, 'export const render = page => `<main><h1>${page.title}</h1></main>`;');
  const working = new StaticRenderPool({ template, rendererURL: pathToFileURL(modulePath).href, size: 4 });
  try {
    await exportStatic({ ...options, records: records(), renderPage: record => working.render(record), parallelism: 4 });
    assert.equal(await readFile(join(options.target, 'manifest.json'), 'utf8'), serial);
  } finally {
    await working.close();
  }
});

test('parallel render failures preserve the previous release and finish pending work', async t => {
  const options = await fixture(t);
  await exportStatic({ ...options, records: records() });
  const previous = await readFile(join(options.target, 'manifest.json'), 'utf8');
  const modulePath = join(options.target, '..', 'broken-renderer.mjs');
  await writeFile(modulePath, 'export function render(page) { if (page.resource) throw new Error("bad schema"); return "<main>error</main>"; }');
  const pool = new StaticRenderPool({ template, rendererURL: pathToFileURL(modulePath).href, size: 4 });
  try {
    await assert.rejects(exportStatic({ ...options, records: records(), renderPage: record => pool.render(record), parallelism: 4 }), /bad schema/);
    assert.equal(await readFile(join(options.target, 'manifest.json'), 'utf8'), previous);
    await assert.rejects(readFile(join(options.target + '.tmp', 'manifest.json')), { code: 'ENOENT' });
  } finally {
    await pool.close();
  }
});
