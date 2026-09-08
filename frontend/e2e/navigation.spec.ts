import type { Page } from '@playwright/test';
import { test, expect, catalogVersion } from './fixtures';
import type { Product } from '../src/types';

function routes(catalog: Product[]) {
  const version = catalogVersion(catalog, 'kubernetes');
  const base = `/kubernetes/${encodeURIComponent(version)}`;
  return {
    version, base,
    pod: `${base}/io.k8s.api.core.v1.Pod`,
    deployment: `${base}/io.k8s.api.apps.v1.Deployment`,
    recursive: `${base}/io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.JSONSchemaProps`,
    certBase: `/certmanager/${encodeURIComponent(catalogVersion(catalog, 'certmanager'))}`,
  };
}

async function ready(page: Page, url: string) {
  await page.goto(url);
  await expect(page.getByRole('button', { name: 'Search all types' })).toBeEnabled();
}

test('home follows the latest Kubernetes catalog default', async ({ page, catalog }) => {
  const current = routes(catalog);
  expect(catalog.find(product => product.name === 'kubernetes')?.defaultVersion).toBe(current.version);
  await ready(page, '/');
  await expect(page).toHaveURL(current.base);
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(`kubernetes ${current.version}`);
  await expect(page.getByRole('link', { name: 'Manifests.io home' })).toHaveAttribute('href', current.base);
  await expect(page.getByRole('combobox', { name: 'Specification & version' })).toHaveValue(current.base);
});

test('quick search keeps typed queries local and opens canonical nested types', async ({ page, catalog }) => {
  const current = routes(catalog);
  const requests: { url: string; body: string }[] = [];
  page.on('request', request => requests.push({ url: request.url(), body: request.postData() || '' }));
  await ready(page, `${current.deployment}?path=Deployment`);
  expect(requests.filter(request => request.url.includes('/api/definitions'))).toHaveLength(0);
  await page.getByRole('button', { name: 'Search all types' }).click();
  const input = page.getByRole('combobox', { name: 'Search all types' });
  await expect(input).toBeFocused();
  await expect(page.getByRole('listbox', { name: 'Matching types' })).toHaveAttribute('aria-busy', 'false');
  const query = 'ContainerStaus';
  await input.pressSequentially(query);
  const first = page.getByRole('listbox').getByRole('option').first();
  await expect(first).toContainText('ContainerStatus');
  await expect(first).toHaveAttribute('href', `${current.base}/io.k8s.api.core.v1.ContainerStatus`);
  const definitions = requests.filter(request => new URL(request.url).pathname === '/api/definitions');
  expect(definitions).toHaveLength(1);
  expect([...new URL(definitions[0].url).searchParams.entries()]).toEqual([['item', 'kubernetes'], ['version', current.version]]);
  expect(requests.filter(request => decodeURIComponent(request.url).includes(query) || request.body.includes(query))).toEqual([]);
  await input.press('Enter');
  await expect(page).toHaveURL(`${current.base}/io.k8s.api.core.v1.ContainerStatus`);
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('ContainerStatus');
  expect(new URL((await page.locator('link[rel="canonical"]').getAttribute('href'))!).pathname).toBe(`${current.base}/io.k8s.api.core.v1.ContainerStatus`);
  expect(requests.filter(request => decodeURIComponent(request.url).includes(query) || request.body.includes(query))).toEqual([]);
});

test('keyboard shortcuts trap focus, select with arrows, and restore the previous control', async ({ page, catalog }) => {
  const current = routes(catalog);
  await ready(page, current.pod);
  const trigger = page.getByRole('button', { name: 'Search all types' });
  const dialog = page.getByRole('dialog', { name: 'Jump to a type' });
  const input = page.getByRole('combobox', { name: 'Search all types' });
  await trigger.focus();
  for (const shortcut of ['Control+k', 'Control+p', 'Meta+k', 'Meta+p']) {
    await page.keyboard.press(shortcut);
    await expect(dialog).toBeVisible();
    await expect(input).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(dialog).not.toBeVisible();
    await expect(trigger).toBeFocused();
  }
  await trigger.click();
  await input.fill('ContainerState');
  await expect(dialog.getByRole('option').first()).toHaveAttribute('aria-selected', 'true');
  await trigger.evaluate(element => element.focus());
  await expect(input).toBeFocused();
  for (let step = 0; step < 6; step++) {
    await page.keyboard.press('Tab');
    // Native dialogs permit browser chrome focus, represented by body in Chromium.
    expect(await page.evaluate(() => document.activeElement === document.body || !!document.activeElement?.closest('dialog'))).toBe(true);
  }
  await input.focus();
  await input.press('ArrowDown');
  const selected = dialog.getByRole('option', { selected: true });
  await expect(selected).not.toHaveAttribute('id', 'type-result-0');
  const target = await selected.getAttribute('href');
  await input.press('Enter');
  await expect(page).toHaveURL(target!);
  await expect(trigger).toBeEnabled();
  await page.keyboard.press('/');
  await expect(page.getByRole('searchbox', { name: 'Filter fields' })).toBeFocused();
});

test('failed definitions load offers a working retry', async ({ page, catalog, expectedConsoleErrors }) => {
  const current = routes(catalog);
  expectedConsoleErrors.push(/^Failed to load resource: net::ERR_FAILED$/);
  let attempts = 0;
  await page.route('**/api/definitions?*', async route => {
    attempts++;
    if (attempts === 1) await route.abort('failed');
    else await route.continue();
  });
  await ready(page, current.pod);
  await page.getByRole('button', { name: 'Search all types' }).click();
  await expect(page.getByRole('dialog').getByRole('status')).toHaveText('Could not load types.');
  await page.getByRole('button', { name: 'Retry loading types' }).click();
  await page.getByRole('combobox', { name: 'Search all types' }).fill('ContainerStatus');
  await expect(page.getByRole('listbox').getByRole('option').first()).toContainText('ContainerStatus');
  expect(attempts).toBe(2);
});

test('search follows the selected catalog and preserves inline CRD targets', async ({ page, catalog }) => {
  const current = routes(catalog);
  await ready(page, current.base);
  await page.getByRole('button', { name: 'Search all types' }).click();
  await page.getByRole('combobox', { name: 'Search all types' }).fill('ContainerStatus');
  await expect(page.getByRole('listbox').getByRole('option').first()).toHaveAttribute('href', `${current.base}/io.k8s.api.core.v1.ContainerStatus`);
  await page.keyboard.press('Escape');
  await page.getByRole('combobox', { name: 'Specification & version' }).selectOption(current.certBase);
  await expect(page).toHaveURL(current.certBase);
  await expect(page.getByRole('button', { name: 'Search all types' })).toBeEnabled();
  await page.getByRole('button', { name: 'Search all types' }).click();
  await page.getByRole('combobox', { name: 'Search all types' }).fill('CertificateSpecAdditionaloutputformats');
  const target = `${current.certBase}/io.cert-manager.v1.Certificate?pointer=%2Fproperties%2Fspec%2Fproperties%2FadditionalOutputFormats`;
  await expect(page.getByRole('listbox').getByRole('option').first()).toHaveAttribute('href', target);
  await page.getByRole('combobox', { name: 'Search all types' }).press('Enter');
  await expect(page).toHaveURL(target);
  await expect(page.getByRole('heading', { level: 1 })).toContainText('additionalOutputFormats');
});

for (const javaScriptEnabled of [true, false]) {
  test.describe(javaScriptEnabled ? 'hydrated navigation' : 'without JavaScript', () => {
    test.use({ javaScriptEnabled });

    test('Deployment traversal keeps readable context at the actual referenced type', async ({ page, catalog }) => {
      const current = routes(catalog);
      await page.goto(current.deployment);
      if (javaScriptEnabled) await expect(page.getByRole('button', { name: 'Search all types' })).toBeEnabled();
      for (const field of ['spec', 'template', 'spec']) await page.getByRole('link', { name: field, exact: true }).click();
      await expect(page).toHaveURL(`${current.base}/io.k8s.api.core.v1.PodSpec?path=Deployment.spec.template.spec`);
      await expect(page.getByRole('heading', { level: 1 })).toHaveText('Deployment.spec.template.spec');
      await expect(page.getByRole('link', { name: 'containers', exact: true })).toBeVisible();
    });

    test('circular references stop at three visits across refresh and history', async ({ page, catalog }) => {
      const current = routes(catalog);
      await page.goto(current.recursive);
      if (javaScriptEnabled) await expect(page.getByRole('button', { name: 'Search all types' })).toBeEnabled();
      await page.getByRole('link', { name: 'allOf', exact: true }).click();
      const secondVisit = page.url();
      await page.getByRole('link', { name: 'allOf', exact: true }).click();
      const thirdVisit = page.url();
      const blocked = page.getByRole('row').filter({ has: page.locator('th .field-name', { hasText: /^allOf/ }) });
      await expect(page.getByRole('heading', { level: 1 })).toHaveText('JSONSchemaProps.allOf.allOf');
      await expect(blocked).toContainText('Circular reference');
      await expect(blocked.getByRole('link')).toHaveCount(0);
      await page.reload();
      await expect(blocked).toContainText('This schema has already been visited 3 times in this path.');
      await expect(blocked.getByRole('link')).toHaveCount(0);
      await page.goBack();
      await expect(page).toHaveURL(secondVisit);
      await expect(page.getByRole('link', { name: 'allOf', exact: true })).toHaveCount(1);
      await page.goForward();
      await expect(page).toHaveURL(thirdVisit);
      await expect(blocked.getByRole('link')).toHaveCount(0);
      await page.getByRole('link', { name: 'externalDocs', exact: true }).click();
      await expect(page.getByRole('heading', { level: 1 })).toHaveText('JSONSchemaProps.allOf.allOf.externalDocs');
    });
  });
}

test('320px search remains usable without horizontal overflow', async ({ page, catalog }) => {
  const current = routes(catalog);
  await page.setViewportSize({ width: 320, height: 740 });
  await ready(page, current.pod);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(320);
  await page.getByRole('button', { name: 'Search all types' }).click();
  await page.getByRole('combobox', { name: 'Search all types' }).fill('CustomResourceDefinition');
  await expect(page.getByRole('listbox').getByRole('option').first()).toBeVisible();
  const bounds = await page.getByRole('dialog').boundingBox();
  expect(bounds!.x).toBeGreaterThanOrEqual(0);
  expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(320);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(320);
  await page.getByRole('button', { name: 'Close type search' }).click();
  await expect(page.getByRole('button', { name: 'Search all types' })).toBeFocused();
});
