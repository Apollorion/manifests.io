import assert from 'node:assert/strict';
import { setTimeout } from 'node:timers/promises';

const base = process.argv[2] || 'http://localhost:8080';
let ready = false;
for (let attempt = 0; attempt < 30; attempt++) {
  try {
    ready = (await fetch(`${base}/readyz`, { signal: AbortSignal.timeout(1000) })).ok;
    if (ready) break;
  } catch {}
  await setTimeout(1000);
}
assert(ready, 'Server never became ready');

const products = await (await fetch(`${base}/api/catalog`)).json();
assert(products.some(product => product.name === 'kubernetes' && product.versions.includes('1.34')));
assert(products.some(product => product.name === 'gateway api'));

const pod = '/kubernetes/1.34/io.k8s.api.core.v1.Pod';
const pageResponse = await fetch(`${base}/api/page?item=kubernetes&version=1.34&resource=io.k8s.api.core.v1.Pod`);
assert.equal(pageResponse.status, 200);
const page = await pageResponse.json();
const spec = page.resources.find(row => row.name === 'spec');
assert.equal(spec.type, 'PodSpec');
assert(spec.description.startsWith('Specification of the desired behavior of the pod.'));

for (const path of [pod, `${pod}?path=%2Fproperties%2Fspec`, `${pod}?linked=Workload&path=%2Fproperties%2Fspec`, '/certmanager/1.14/io.cert-manager.v1.CertificateSpec']) {
  const response = await fetch(`${base}${path}`);
  assert.equal(response.status, 200, path);
  const body = await response.text();
  assert(body.includes('<main'), `No rendered documentation at ${path}`);
  assert(body.includes('<tbody>'), `No rendered fields at ${path}`);
  assert(body.includes('id="__PAGE_DATA__"'), `No browser data at ${path}`);
  assert(!body.includes('<!--page-'), `Incomplete template at ${path}`);
  assert.equal(response.headers.get('x-content-type-options'), 'nosniff');
}

const container = await (await fetch(`${base}/api/page?item=kubernetes&version=1.34&resource=io.k8s.api.core.v1.Pod&path=/properties/spec/properties/containers/items`)).json();
assert(container.resources.some(row => row.name === 'name' && row.required));
assert.equal(container.canonical, '/kubernetes/1.34/io.k8s.api.core.v1.Container');
assert.equal((await fetch(`${base}/kubernetes/1.34/missing`)).status, 404);
assert.equal((await fetch(`${base}/api/page?item=kubernetes&item=flux&version=1.34`)).status, 400);
assert.equal((await fetch(`${base}/api/catalog`, { method: 'POST' })).status, 405);
assert.equal(await (await fetch(`${base}${pod}`, { method: 'HEAD' })).text(), '');
console.log('Container smoke checks passed: API, field descriptions, nested SSR, legacy URLs, required fields, errors, and headers.');
