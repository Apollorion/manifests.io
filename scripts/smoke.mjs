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
const kubernetes = products.find(product => product.name === 'kubernetes');
assert(kubernetes?.defaultVersion && kubernetes.versions.includes(kubernetes.defaultVersion));
const kubeVersion = kubernetes.defaultVersion;
const kubeBase = `/kubernetes/${encodeURIComponent(kubeVersion)}`;
const certmanager = products.find(product => product.name === 'certmanager');
assert(certmanager?.versions.length);
const certVersion = certmanager.versions.at(-1);
const certBase = `/certmanager/${encodeURIComponent(certVersion)}`;
const home = await fetch(base + '/', { redirect: 'manual' });
assert.equal(home.status, 307);
assert.equal(home.headers.get('location'), kubeBase);
assert(products.some(product => product.name === 'gateway api'));

const robotsResponse = await fetch(`${base}/robots.txt`);
assert.equal(robotsResponse.status, 200);
const robots = await robotsResponse.text();
assert(!robots.includes('apiextensions'), 'Robots policy still blocks recursive schema documentation');
assert(!/^Disallow:[ \t]*\S+/m.test(robots), 'Robots policy unexpectedly blocks documentation');
const sitemapURL = robots.match(/^Sitemap: (https?:\/\/\S+\/sitemap\.xml)$/m)?.[1];
assert(sitemapURL, 'Robots policy does not advertise an absolute sitemap URL');
const siteOrigin = new URL(sitemapURL).origin;
const indexResponse = await fetch(`${base}/sitemap.xml`);
assert.equal(indexResponse.status, 200);
assert(indexResponse.headers.get('content-type').startsWith('application/xml'));
const sitemapIndex = await indexResponse.text();
assert(sitemapIndex.includes('<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">'));
const sitemapChunks = [...sitemapIndex.matchAll(/<loc>([^<]+)<\/loc>/g)].map(match => new URL(match[1]));
assert(sitemapChunks.length > 0 && sitemapChunks.length <= 50_000);
const crawlerLocations = new Set();
for (const chunk of sitemapChunks) {
  assert.equal(chunk.origin, siteOrigin);
  assert(/^\/sitemap-[1-9]\d*\.xml$/.test(chunk.pathname));
  const response = await fetch(new URL(chunk.pathname, base));
  assert.equal(response.status, 200);
  assert.equal(response.headers.get('x-content-type-options'), 'nosniff');
  const body = await response.text();
  assert(Buffer.byteLength(body) <= 50 * 1024 * 1024);
  assert(body.includes('<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">'));
  const locations = [...body.matchAll(/<loc>([^<]+)<\/loc>/g)].map(match => match[1].replaceAll('&amp;', '&'));
  assert(locations.length > 0 && locations.length <= 50_000);
  for (const location of locations) {
    const url = new URL(location);
    assert.equal(url.origin, siteOrigin);
    assert.equal(url.hash, '');
    assert([...url.searchParams.keys()].every(key => key === 'pointer'), `Traversal URL leaked into sitemap: ${location}`);
    assert(!crawlerLocations.has(location), `Duplicate sitemap URL: ${location}`);
    crawlerLocations.add(location);
  }
}
for (const route of [
  `${kubeBase}/io.k8s.api.core.v1.ContainerStatus`,
  `${kubeBase}/io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.JSONSchemaProps`,
  `${certBase}/io.cert-manager.v1.Certificate?pointer=%2Fproperties%2Fspec`,
]) assert(crawlerLocations.has(siteOrigin + route), `Canonical documentation missing from sitemap: ${route}`);
assert(!crawlerLocations.has(`${siteOrigin}${certBase}/io.cert-manager.v1.CertificateSpec`), 'Legacy alias leaked into sitemap');
for (const route of ['/robots.txt', '/sitemap.xml', sitemapChunks[0].pathname]) {
  const response = await fetch(new URL(route, base), { method: 'HEAD' });
  assert.equal(response.status, 200);
  assert.equal(await response.text(), '');
}

for (const [item, version, name, title] of [['kubernetes', kubeVersion, 'ContainerStatus', 'ContainerStatus'], ['certmanager', certVersion, 'CertificateSpec', 'Certificate.spec']]) {
  const response = await fetch(`${base}/api/definitions?${new URLSearchParams({ item, version })}`);
  assert.equal(response.status, 200);
  const definitions = await response.json();
  const contextualDefinitions = await fetch(`${base}/api/definitions?${new URLSearchParams({ item, version, path: 'Workload.spec', trail: 'invalid' })}`);
  assert.equal(contextualDefinitions.status, 200);
  assert.deepEqual(await contextualDefinitions.json(), definitions, 'Traversal context fragmented the definition index');
  const match = definitions.find(definition => definition.name === name);
  assert(match, `Quick search cannot find nested type ${name}`);
  const document = await fetch(new URL(match.href, base));
  assert.equal(document.status, 200);
  assert((await document.text()).includes(`<h1>${title}</h1>`), `Search result opened the wrong type for ${name}`);
}

const pod = `${kubeBase}/io.k8s.api.core.v1.Pod`;
const podSpec = `${kubeBase}/io.k8s.api.core.v1.PodSpec`;
const deployment = `${kubeBase}/io.k8s.api.apps.v1.Deployment`;
const pageResponse = await fetch(`${base}/api/page?item=kubernetes&version=${encodeURIComponent(kubeVersion)}&resource=io.k8s.api.core.v1.Pod`);
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

for (const path of [pod, `${podSpec}?path=Deployment.spec.template.spec`, `${podSpec}?linked=Workload.spec`, `${certBase}/io.cert-manager.v1.CertificateSpec`]) {
  const response = await fetch(`${base}${path}`);
  assert.equal(response.status, 200, path);
  const body = await response.text();
  assert(body.includes(`rel="canonical" href="${siteOrigin}/`), 'Crawler documents use a different origin from page canonical URLs');
  assert(body.includes('<main'), `No rendered documentation at ${path}`);
  assert(body.includes('<tbody>'), `No rendered fields at ${path}`);
  assert(body.includes('id="__PAGE_DATA__"'), `No browser data at ${path}`);
  assert(!body.includes('<!--page-'), `Incomplete template at ${path}`);
  assert(!body.includes('baadaa'), 'Old design credit is still rendered');
  assert(body.includes('Search all types'), 'Quick search entry point is missing');
  if (path === pod) assert(body.includes(`href="${podSpec}?path=Pod.spec"`), 'Rendered spec link lost target or traversal');
  if (path.includes('path=Deployment.spec.template.spec')) assert(body.includes('<title>PodSpec | Manifests.io</title>'));
  assert.equal(response.headers.get('x-content-type-options'), 'nosniff');
}

for (const [resource, field, target] of [
  ['io.k8s.api.apps.v1.Deployment', 'spec', 'io.k8s.api.apps.v1.DeploymentSpec'],
  ['io.k8s.api.apps.v1.DeploymentSpec', 'template', 'io.k8s.api.core.v1.PodTemplateSpec'],
  ['io.k8s.api.core.v1.PodTemplateSpec', 'spec', 'io.k8s.api.core.v1.PodSpec'],
  ['io.k8s.api.core.v1.PodSpec', 'containers', 'io.k8s.api.core.v1.Container'],
]) {
  const query = new URLSearchParams({ item: 'kubernetes', version: kubeVersion, resource });
  const canonical = await (await fetch(`${base}/api/page?${query}`)).json();
  assert.equal(canonical.resource, resource);
  assert.equal(canonical.title, resource.split('.').at(-1));
  assert(!canonical.path && !canonical.trail, 'Visitor traversal leaked into canonical API data');
  const href = new URL(canonical.resources.find(row => row.name === field).href, base);
  assert.equal(href.pathname, `${kubeBase}/${target}`);
  for (const context of [
    { path: 'Deployment.spec.template.spec', trail: '{"unrelated#":2}' },
    { linked: 'Workload.spec', trail: 'invalid' },
  ]) {
    const contextual = await fetch(`${base}/api/page?${query}&${new URLSearchParams(context)}`);
    assert.equal(contextual.status, 200);
    assert.deepEqual(await contextual.json(), canonical, 'Traversal context fragmented API data');
  }
}

const container = await (await fetch(`${base}/api/page?item=kubernetes&version=${encodeURIComponent(kubeVersion)}&resource=io.k8s.api.core.v1.Pod&pointer=/properties/spec/properties/containers/items`)).json();
assert(container.resources.some(row => row.name === 'name' && row.required));
assert.equal(container.canonical, `${kubeBase}/io.k8s.api.core.v1.Container`);
assert.equal(container.resource, 'io.k8s.api.core.v1.Container');
assert.equal(container.path || '', '');
assert.equal(container.title, 'Container');
for (const route of [
  `${kubeBase}/missing`,
  '/kubernetes/missing',
  `/api/page?item=kubernetes&version=${encodeURIComponent(kubeVersion)}&resource=missing`,
  '/api/definitions?item=kubernetes&version=missing',
]) {
  const response = await fetch(base + route);
  assert.equal(response.status, 404, route);
  assert.equal(response.headers.get('cache-control'), 'public, max-age=0, s-maxage=604800, must-revalidate', `Missing schema is not cacheable: ${route}`);
}
assert.equal((await fetch(`${base}/api/page?item=kubernetes&item=flux&version=${encodeURIComponent(kubeVersion)}`)).status, 400);
assert.equal((await fetch(`${base}/api/catalog`, { method: 'POST' })).status, 405);
assert.equal(await (await fetch(`${base}${pod}`, { method: 'HEAD' })).text(), '');

const cyclicResource = 'io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.JSONSchemaProps';
const cyclicURL = new URL(`${kubeBase}/${cyclicResource}`, base);
const cyclicQuery = new URLSearchParams({ item: 'kubernetes', version: kubeVersion, resource: cyclicResource });
const cyclic = await (await fetch(`${base}/api/page?${cyclicQuery}`)).json();
assert(cyclic.cycles.includes(`${cyclicResource}#`), 'Canonical data lacks the recursive node identity');
assert.deepEqual(cyclic.cycles, [...new Set(cyclic.cycles)].sort(), 'Cycle identities are not sorted and unique');
assert(cyclic.resources.find(row => row.name === 'allOf').href, 'Canonical recursive navigation is unavailable');
assert(!cyclic.path && !cyclic.trail && !cyclic.canonical.includes('trail='));
const canonicalHTML = await (await fetch(cyclicURL)).text();
for (const visits of [1, 2, 3]) {
  const context = new URLSearchParams({ path: `Workload${'.allOf'.repeat(visits)}`, trail: JSON.stringify({ [`${cyclicResource}#`]: visits }) });
  const response = await fetch(`${base}/api/page?${cyclicQuery}&${context}`);
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), cyclic, 'Recursive traversal changed canonical API data');
  const html = await (await fetch(`${cyclicURL}?${context}`)).text();
  assert.equal(html, canonicalHTML, 'Recursive traversal changed canonical HTML');
  assert(html.includes('<h1>JSONSchemaProps</h1>'));
  const embedded = JSON.parse(html.match(/<script id="__PAGE_DATA__" type="application\/json">(.*?)<\/script>/s)[1]);
  assert.deepEqual(embedded, cyclic, 'Embedded schema data differs from canonical API data');
}
const hpa = await (await fetch(`${base}${kubeBase}/io.k8s.api.autoscaling.v2.HorizontalPodAutoscaler`)).text();
assert(hpa.includes(`href="${kubeBase}/io.k8s.api.autoscaling.v2.HorizontalPodAutoscaler" aria-current="page"`), 'Current API version is not marked');
for (const property of ['og:site_name', 'og:image:alt', 'og:image:width', 'og:image:height', 'og:image:type']) {
  assert(hpa.includes(`property="${property}"`), `Missing ${property}`);
}
const missing = await (await fetch(`${base}${kubeBase}/missing`)).text();
assert(missing.includes('Specification &amp; version') && missing.includes('See an issue here?'), 'Server error lost recovery controls');
console.log(`Container smoke checks passed: canonical API data, quick search, ${crawlerLocations.size} sitemap URLs, robots policy, descriptions, traversal sharing, cycle metadata, no resource redirects, nested SSR, required fields, cacheable schema 404s, and headers.`);
