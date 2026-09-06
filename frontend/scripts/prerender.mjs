import { spawn } from 'node:child_process';
import { mkdir, readFile, rename, rm, writeFile } from 'node:fs/promises';
import { createInterface } from 'node:readline';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
import { gzipSync } from 'node:zlib';
import { render } from '../dist-server/entry-server.js';

const root = fileURLToPath(new URL('../', import.meta.url));
const template = await readFile(resolve(root, 'dist/index.html'), 'utf8');
const target = resolve(root, 'prerender');
const staging = `${target}.tmp`;
await rm(staging, { recursive: true, force: true });
await mkdir(staging, { recursive: true });
const child = spawn(resolve(root, '../build/manifests'), ['-data', resolve(root, '..'), '-export'], {
  stdio: ['ignore', 'pipe', 'inherit'],
});
const done = new Promise((resolve, reject) => {
  child.on('error', reject);
  child.on('close', code => code === 0 ? resolve() : reject(new Error(`Page export exited with ${code}`)));
});
done.catch(() => {});
let count = 0;
try {
  for await (const line of createInterface({ input: child.stdout, crlfDelay: Infinity })) {
    const { page, file } = JSON.parse(line);
    if (!/^[a-f0-9]{64}\.html\.gz$/.test(file)) throw new Error('Invalid exported page filename');
    const html = template.replace('<!--app-html-->', () => render(page));
    await writeFile(resolve(staging, file), gzipSync(html));
    count++;
  }
  await done;
  await rm(target, { recursive: true, force: true });
  await rename(staging, target);
  console.log(`Prerendered ${count} documentation pages.`);
} catch (error) {
  child.kill('SIGTERM');
  await rm(staging, { recursive: true, force: true });
  throw error;
}
