import { createHash } from 'node:crypto';
import { execFileSync, spawn } from 'node:child_process';
import { mkdir, readFile, readdir, rename, rm, writeFile } from 'node:fs/promises';
import { extname, join, resolve } from 'node:path';
import { availableParallelism } from 'node:os';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';
import { renderStaticPage } from './static-render.mjs';
import { StaticRenderPool } from './static-render-pool.mjs';

const types = {
  '.html': 'text/html; charset=utf-8', '.json': 'application/json; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8', '.css': 'text/css; charset=utf-8',
  '.txt': 'text/plain; charset=utf-8', '.xml': 'application/xml; charset=utf-8',
  '.svg': 'image/svg+xml', '.png': 'image/png', '.jpg': 'image/jpeg', '.jpeg': 'image/jpeg',
  '.webp': 'image/webp', '.gif': 'image/gif', '.ico': 'image/x-icon',
  '.woff': 'font/woff', '.woff2': 'font/woff2', '.ttf': 'font/ttf',
  '.webmanifest': 'application/manifest+json', '.wasm': 'application/wasm',
};

export async function exportStatic({ records, target, template, render, renderPage, parallelism = 1, release, webDir, onProgress = () => {} }) {
  if (!/^[a-f0-9]{40}$/.test(release)) throw new Error('Static release must be a full Git commit SHA');
  if (!Number.isInteger(parallelism) || parallelism < 1 || parallelism > 4) throw new Error('Static export parallelism must be between one and four');
  for (const marker of ['<!--app-html-->', '<!--page-head-->', '<!--page-data-->']) {
    if (!template.includes(marker)) throw new Error(`Static template lacks ${marker}`);
  }
  const staging = `${target}.tmp`;
  await rm(staging, { recursive: true, force: true });
  await mkdir(join(staging, 'objects'), { recursive: true });
  const inventory = new Map();
  const stats = { pages: 0, documents: 0, largestShardBytes: 0, largestShard: '', storedBytes: 0 };
  let manifest, current, complete = false;
  let pages = new Map();
  let pendingPages = [];
  async function flushPages() {
    const results = await Promise.allSettled(pendingPages);
    pendingPages = [];
    const failure = results.find(result => result.status === 'rejected');
    if (failure) throw failure.reason;
  }
  async function storeObject(body, extension, contentType, compress) {
    const digest = createHash('sha256').update(body).digest('hex');
    const ref = { object: `objects/${digest}${extension}`, contentType, ...(compress ? { contentEncoding: 'gzip' } : {}) };
    if (!inventory.has(ref.object)) {
      inventory.set(ref.object, { ...ref, bytes: body.length });
      stats.storedBytes += body.length;
      await writeFile(join(staging, ref.object), body, { flag: 'wx' });
    }
    return ref;
  }
  async function object(data, extension, contentType = types[extension]) {
    if (!contentType) throw new Error(`Unsupported static object type: ${extension}`);
    const compress = /^(text\/|application\/(json|xml|manifest\+json))/.test(contentType) || contentType === 'image/svg+xml';
    const body = compress ? gzipSync(data) : Buffer.from(data);
    return storeObject(body, extension, contentType, compress);
  }
  const jsonObject = value => object(JSON.stringify(value) + '\n', '.json');
  async function copyAssets(directory, relative = '') {
    const entries = await readdir(join(directory, relative), { withFileTypes: true });
    entries.sort((a, b) => a.name < b.name ? -1 : a.name > b.name ? 1 : 0);
    for (const entry of entries) {
      if (entry.name.startsWith('.') || entry.name.endsWith('.map')) continue;
      const path = join(relative, entry.name);
      if (entry.isDirectory()) {
        await copyAssets(directory, path);
      } else if (entry.isFile() && path !== 'index.html') {
        const extension = extname(entry.name).toLowerCase();
        const ref = await object(await readFile(join(directory, path)), extension);
        manifest.files['/' + path.split('\\').join('/')] = ref;
      } else if (entry.isSymbolicLink()) {
        throw new Error(`Static assets must not contain symlinks: ${path}`);
      }
    }
  }
  try {
    for await (const record of records) {
      if (record.kind !== 'page') await flushPages();
      if (complete) throw new Error('Static records continued after completion');
      if (!manifest && record.kind !== 'root') throw new Error('Static records must begin with the root');
      switch (record.kind) {
        case 'root':
          if (manifest) throw new Error('Duplicate static root');
          manifest = { format: 1, release, site: record.site, default: record.default, catalog: await jsonObject(record.catalog), documents: {}, files: {}, errors: {} };
          break;
        case 'document':
          if (current) throw new Error('Unfinished static document');
          current = { ...record.graph, errors: {} };
          pages = new Map();
          break;
        case 'page': {
          const work = (async () => {
            const page = JSON.parse(record.data);
            const rendered = renderPage ? await renderPage(record) : renderStaticPage(template, record, render);
            const refs = { html: await storeObject(rendered.html, '.html', types['.html'], true), json: await storeObject(rendered.json, '.json', types['.json'], true) };
            if (record.status === 200) {
              if (!current || pages.has(page.canonical)) throw new Error(`Duplicate or orphan static page: ${page.canonical}`);
              if (page.path || page.trail) throw new Error('Traversal context leaked into a static page');
              pages.set(page.canonical, refs);
              if (!page.resource) current.index = refs;
              stats.pages++;
            } else if ([400, 404].includes(record.status)) {
              (current || manifest).errors[record.status] = refs;
            } else {
              throw new Error(`Unsupported static page status: ${record.status}`);
            }
          })();
          work.catch(() => {});
          pendingPages.push(work);
          if (pendingPages.length >= parallelism) await flushPages();
          break;
        }
        case 'definitions':
          if (!current) throw new Error('Definitions outside a document');
          current.definitions = await jsonObject(record.data);
          break;
        case 'document-end': {
          if (!current?.index || !current.definitions) throw new Error('Incomplete static document');
          for (const node of Object.values(current.nodes)) {
            const refs = pages.get(node.canonical);
            if (!refs) throw new Error(`Canonical node has no static page: ${node.canonical}`);
            Object.assign(node, refs);
            for (const target of [...Object.values(node.edges || {}), ...(node.oneOf || []).map(variant => variant.node)]) {
              if (!Object.hasOwn(current.nodes, target)) throw new Error(`Static edge target is absent: ${target}`);
            }
          }
          for (const target of Object.values(current.resources)) {
            if (!Object.hasOwn(current.nodes, target)) throw new Error(`Static resource target is absent: ${target}`);
          }
          if (pages.size !== Object.keys(current.nodes).length + 1) throw new Error('Static page and node counts differ');
          const key = `${current.item}/${current.version}`;
          if (Object.hasOwn(manifest.documents, key)) throw new Error(`Duplicate static document: ${key}`);
          const shard = JSON.stringify(current) + '\n';
          const size = Buffer.byteLength(shard);
          if (size > 16 * 1024 * 1024) throw new Error(`Static shard exceeds the edge metadata limit: ${key}`);
          if (size > stats.largestShardBytes) { stats.largestShardBytes = size; stats.largestShard = key; }
          manifest.documents[key] = await object(shard, '.json');
          stats.documents++;
          onProgress({ document: key, pages: stats.pages, documents: stats.documents });
          current = undefined;
          pages.clear();
          break;
        }
        case 'file':
          manifest.files[record.path] = await object(record.data, extname(record.path), record.contentType);
          break;
        case 'complete':
          if (current || !manifest.errors[400] || !manifest.errors[404]) throw new Error('Incomplete static export');
          complete = true;
          break;
        default:
          throw new Error(`Unknown static record kind: ${record.kind}`);
      }
    }
    if (!complete) throw new Error('Static stream ended before completion');
    await copyAssets(webDir);
    await writeFile(join(staging, 'manifest.json'), JSON.stringify(manifest, null, 2) + '\n');
    const objects = [...inventory.values()].sort((a, b) => a.object.localeCompare(b.object));
    await writeFile(join(staging, 'upload-manifest.json'), JSON.stringify({ format: 1, objects }, null, 2) + '\n');
    await rm(target, { recursive: true, force: true });
    await rename(staging, target);
    return { ...stats, objects: objects.length };
  } catch (error) {
    await Promise.allSettled(pendingPages);
    await rm(staging, { recursive: true, force: true });
    throw error;
  }
}

async function main() {
  const root = fileURLToPath(new URL('../../', import.meta.url));
  const webDir = resolve(root, 'frontend/dist');
  const target = resolve(process.argv[2] || join(root, 'build/static'));
  const configuredRelease = process.env.STATIC_RELEASE || process.env.VITE_APP_VERSION;
  const release = configuredRelease || execFileSync('git', ['rev-parse', 'HEAD'], { cwd: root, encoding: 'utf8' }).trim();
  const template = await readFile(join(webDir, 'index.html'), 'utf8');
  const parallelism = Math.min(4, availableParallelism());
  const pool = new StaticRenderPool({ template, rendererURL: new URL('../dist-server/entry-server.js', import.meta.url).href, size: parallelism });
  const child = spawn(resolve(root, 'build/manifests'), ['-data', root, '-export-static'], { stdio: ['ignore', 'pipe', 'inherit'] });
  const done = new Promise((resolve, reject) => {
    child.on('error', reject);
    child.on('close', code => code === 0 ? resolve() : reject(new Error(`Static data export exited with ${code}`)));
  });
  done.catch(() => {});
  async function* records() {
    child.stdout.setEncoding('utf8');
    let pending = '';
    for await (const chunk of child.stdout) {
      pending += chunk;
      let newline;
      while ((newline = pending.indexOf('\n')) !== -1) {
        const line = pending.slice(0, newline);
        pending = pending.slice(newline + 1);
        yield JSON.parse(line);
      }
    }
    if (pending) throw new Error('Static stream ended with an incomplete record');
    await done;
  }
  try {
    const stats = await exportStatic({ records: records(), target, template, renderPage: record => pool.render(record), parallelism, release, webDir,
      onProgress: progress => console.log(`Static export: ${JSON.stringify(progress)}`) });
    console.log(`Static release ${release}: ${JSON.stringify(stats)}`);
  } catch (error) {
    child.kill('SIGTERM');
    throw error;
  } finally {
    await pool.close();
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) await main();
