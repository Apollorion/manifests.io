import { test as base, expect } from '@playwright/test';

export const test = base.extend<{ browserErrors: void; expectedConsoleErrors: RegExp[] }>({
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
