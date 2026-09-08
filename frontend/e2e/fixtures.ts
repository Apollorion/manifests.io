import { test as base, expect } from '@playwright/test';
import type { Product } from '../src/types';

export const test = base.extend<{ browserErrors: void; expectedConsoleErrors: RegExp[]; catalog: Product[] }>({
  catalog: async ({ request }, use) => {
    const response = await request.get('/api/catalog');
    expect(response.ok()).toBe(true);
    await use(await response.json());
  },
  expectedConsoleErrors: async ({}, use) => { await use([]); },
  browserErrors: [async ({ page, expectedConsoleErrors }, use) => {
    const errors: string[] = [];
    page.on('pageerror', error => errors.push(error.message));
    page.on('console', message => {
      if (message.type() === 'error' && !expectedConsoleErrors.some(pattern => pattern.test(message.text()))) errors.push(message.text());
    });
    await use();
    expect(errors, 'Browser exceptions, console errors, and hydration failures').toEqual([]);
  }, { auto: true }],
});

export { expect };

export function catalogVersion(catalog: Product[], name: string): string {
  const product = catalog.find(product => product.name === name);
  expect(product?.versions.length, `Missing ${name} versions`).toBeGreaterThan(0);
  const version = product!.defaultVersion || product!.versions.at(-1)!;
  expect(product!.versions).toContain(version);
  return version;
}
