import { readdir } from 'node:fs/promises';
import { join } from 'node:path';
import { uploadSourceMap } from '@grafana/faro-bundlers-shared';

const directory = process.argv[2];
const release = process.env.GITHUB_SHA;
const apiKey = process.env.FARO_SOURCEMAP_API_KEY?.trim();
if (!directory || !/^[a-f0-9]{40}$/.test(release ?? '') || !apiKey) {
  throw new Error('Source-map directory, full release SHA, and upload credential are required');
}
const files = (await readdir(directory)).filter(file => file.endsWith('.js.map'));
if (!files.length) throw new Error('No production browser source maps found');
for (const filename of files) {
  const uploaded = await uploadSourceMap({
    sourcemapEndpoint: `https://faro-api-prod-us-east-3.grafana.net/faro/api/v1/app/848/sourcemaps/${release}`,
    stackId: '1807923',
    apiKey,
    filePath: join(directory, filename),
    filename,
    keepSourcemaps: true,
  });
  if (!uploaded) throw new Error(`Source-map upload failed for ${filename}`);
}
console.log(`Uploaded ${files.length} private source map(s) for ${release}`);
