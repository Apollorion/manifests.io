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

for (const [item, version, name, title] of [['kubernetes', '1.34', 'ContainerStatus', 'ContainerStatus'], ['certmanager', '1.14', 'CertificateSpec', 'Certificate.spec']]) {
  const response = await fetch(`${base}/api/definitions?${new URLSearchParams({ item, version })}`);
  assert.equal(response.status, 200);
  const definitions = await response.json();
  const match = definitions.find(definition => definition.name === name);
  assert(match, `Quick search cannot find nested type ${name}`);
  const document = await fetch(new URL(match.href, base));
  assert.equal(document.status, 200);
  assert((await document.text()).includes(`<h1>${title}</h1>`), `Search result opened the wrong type for ${name}`);
}

const pod = '/kubernetes/1.34/io.k8s.api.core.v1.Pod';
const podSpec = '/kubernetes/1.34/io.k8s.api.core.v1.PodSpec';
const deployment = '/kubernetes/1.34/io.k8s.api.apps.v1.Deployment';
const pageResponse = await fetch(`${base}/api/page?item=kubernetes&version=1.34&resource=io.k8s.api.core.v1.Pod`);
assert.equal(pageResponse.status, 200);
const page = await pageResponse.json();
const spec = page.resources.find(row => row.name === 'spec');
assert.equal(spec.type, 'PodSpec');
assert.equal(spec.href, `${podSpec}?path=Pod.spec`);
assert(spec.description.startsWith('Specification of the desired behavior of the pod.'));

for (const path of [
  `${podSpec}?path=Deployment.spec.template.spec`,
  `${podSpec}?linked=Deployment.spec.template.spec`,
  `${deployment}?pointer=%2Fproperties%2Fspec%2Fproperties%2Ftemplate%2Fproperties%2Fspec`,
]) {
  for (const method of ['GET', 'HEAD']) {
    const response = await fetch(`${base}${path}`, { method, redirect: 'manual' });
    assert.equal(response.status, 200, `${method} ${path}`);
    assert.equal(response.headers.get('location'), null, path);
    if (method === 'HEAD') assert.equal(await response.text(), '');
  }
  const response = await fetch(`${base}${path}`);
  assert.equal(response.url, `${base}${path}`);
}

for (const path of [pod, `${podSpec}?path=Deployment.spec.template.spec`, `${podSpec}?linked=Workload.spec`, '/certmanager/1.14/io.cert-manager.v1.CertificateSpec']) {
  const response = await fetch(`${base}${path}`);
  assert.equal(response.status, 200, path);
  const body = await response.text();
  assert(body.includes('<main'), `No rendered documentation at ${path}`);
  assert(body.includes('<tbody>'), `No rendered fields at ${path}`);
  assert(body.includes('id="__PAGE_DATA__"'), `No browser data at ${path}`);
  assert(!body.includes('<!--page-'), `Incomplete template at ${path}`);
  assert(!body.includes('baadaa'), 'Old design credit is still rendered');
  assert(body.includes('Search all types'), 'Quick search entry point is missing');
  if (path === pod) assert(body.includes(`href="${podSpec}?path=Pod.spec"`), 'Rendered spec link lost target or traversal');
  if (path.includes('path=Deployment.spec.template.spec')) assert(body.includes('<title>Deployment.spec.template.spec | Manifests.io</title>'));
  assert.equal(response.headers.get('x-content-type-options'), 'nosniff');
}

let resource = 'io.k8s.api.apps.v1.Deployment';
let context = 'Deployment';
for (const [field, target] of [
  ['spec', 'io.k8s.api.apps.v1.DeploymentSpec'],
  ['template', 'io.k8s.api.core.v1.PodTemplateSpec'],
  ['spec', 'io.k8s.api.core.v1.PodSpec'],
  ['containers', 'io.k8s.api.core.v1.Container'],
]) {
  const selected = await (await fetch(`${base}/api/page?${new URLSearchParams({item:'kubernetes', version:'1.34', resource, path:context})}`)).json();
  context += `.${field}`;
  assert.equal(selected.resources.find(row => row.name === field).href, `/kubernetes/1.34/${target}?path=${context}`);
  resource = target;
}
const selected = await (await fetch(`${base}/api/page?${new URLSearchParams({item:'kubernetes', version:'1.34', resource, path:context})}`)).json();
assert.equal(selected.title, context);
assert.equal(selected.resource, resource);
assert.equal(selected.path, context);
assert.equal(selected.pointer || '', '');

const container = await (await fetch(`${base}/api/page?item=kubernetes&version=1.34&resource=io.k8s.api.core.v1.Pod&pointer=/properties/spec/properties/containers/items`)).json();
assert(container.resources.some(row => row.name === 'name' && row.required));
assert.equal(container.canonical, '/kubernetes/1.34/io.k8s.api.core.v1.Container');
assert.equal(container.resource, 'io.k8s.api.core.v1.Container');
assert.equal(container.path || '', '');
assert.equal(container.title, 'Container');
assert.equal((await fetch(`${base}/kubernetes/1.34/missing`)).status, 404);
assert.equal((await fetch(`${base}/api/page?item=kubernetes&item=flux&version=1.34`)).status, 400);
assert.equal((await fetch(`${base}/api/catalog`, { method: 'POST' })).status, 405);
assert.equal(await (await fetch(`${base}${pod}`, { method: 'HEAD' })).text(), '');

let cyclicURL = new URL('/kubernetes/1.34/io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.JSONSchemaProps', base);
for (let visit = 1; visit <= 3; visit++) {
  const [, item, version, resource] = cyclicURL.pathname.split('/');
  const query = new URLSearchParams(cyclicURL.search);
  query.set('item', item);
  query.set('version', version);
  query.set('resource', resource);
  const response = await fetch(`${base}/api/page?${query}`);
  assert.equal(response.status, 200);
  const current = await response.json();
  const recursive = current.resources.find(row => row.name === 'allOf');
  assert.equal(!!recursive.circular, visit === 3);
  assert(!current.canonical.includes('trail='));
  if (visit < 3) {
    assert(recursive.href);
    cyclicURL = new URL(recursive.href, base);
  } else {
    assert(!recursive.href);
    assert(current.resources.find(row => row.name === 'externalDocs').href, 'Unrelated field was blocked');
    const reloaded = await (await fetch(`${base}/api/page?${query}`)).json();
    assert.deepEqual(reloaded, current, 'Refresh changed the recursion limit');
    const html = await (await fetch(cyclicURL)).text();
    assert(!html.includes(' inert'), 'Server-rendered navigation is disabled');
    assert(html.includes('Circular reference'), 'Circular state was not server-rendered');
    assert(!/<a[^>]*>allOf<span/.test(html), 'Server-rendered recursive link bypasses the limit');
    assert(html.includes('<h1>JSONSchemaProps.allOf.allOf</h1>'), 'Server-rendered title lost traversal');
    const embedded = JSON.parse(html.match(/<script id="__PAGE_DATA__" type="application\/json">(.*?)<\/script>/s)[1]);
    assert(embedded.resources.find(row => row.name === 'allOf').circular, 'HTML page data lost circular state');
  }
}
const hpa = await (await fetch(`${base}/kubernetes/1.34/io.k8s.api.autoscaling.v2.HorizontalPodAutoscaler`)).text();
assert(/href="\/kubernetes\/1\.34\/io\.k8s\.api\.autoscaling\.v2\.HorizontalPodAutoscaler" aria-current="page"/.test(hpa), 'Current API version is not marked');
for (const property of ['og:site_name', 'og:image:alt', 'og:image:width', 'og:image:height', 'og:image:type']) {
  assert(hpa.includes(`property="${property}"`), `Missing ${property}`);
}
const missing = await (await fetch(`${base}/kubernetes/1.34/missing`)).text();
assert(missing.includes('Specification &amp; version') && missing.includes('See an issue here?'), 'Server error lost recovery controls');
console.log('Container smoke checks passed: API, quick search, descriptions, traversal URLs, circular limits, no resource redirects, nested SSR, required fields, errors, and headers.');
