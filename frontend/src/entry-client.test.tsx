import { act, screen } from '@testing-library/react';
import { createRoot, hydrateRoot } from 'react-dom/client';
import { renderToString } from 'react-dom/server';
import { expect, it, vi } from 'vitest';
import { App, AppBoundary } from './App';
import type { Page } from './types';

vi.mock('react-dom/client', async importOriginal => {
  const actual = await importOriginal<typeof import('react-dom/client')>();
  return { ...actual, createRoot: vi.fn(actual.createRoot), hydrateRoot: vi.fn(actual.hydrateRoot) };
});
vi.mock('./telemetry', () => ({ captureError: vi.fn(), captureSearchEvent: vi.fn(), initializeObservability: vi.fn() }));

it('hydrates canonical server markup without fetching page data', async () => {
  window.history.replaceState({}, '', '/example/1/Node');
  const page: Page = {
    item: 'example', version: '1', resource: 'Node', title: 'Node',
    description: '', canonical: '/example/1/Node', catalog: [], breadcrumbs: [],
    otherVersions: [], variants: [],
    cycles: ['Node#'],
    resources: [{ name: 'children', type: 'Node', description: 'Child nodes.', href: '/example/1/Node?path=Node.children&trail=%7B%22Node%23%22%3A1%7D' }],
  };
  const container = document.createElement('div');
  container.id = 'root';
  container.innerHTML = renderToString(<AppBoundary page={page}><App initialPage={page}/></AppBoundary>);
  const originalHeading = container.querySelector('h1');
  const data = document.createElement('script');
  data.id = '__PAGE_DATA__';
  data.type = 'application/json';
  data.textContent = JSON.stringify(page);
  document.body.append(container, data);

  expect(container).not.toHaveAttribute('inert');
  const fetchPage = vi.fn(() => Promise.reject(new Error('Unexpected origin request')));
  vi.stubGlobal('fetch', fetchPage);
  expect(screen.getByRole('link', { name: 'children' })).toBeVisible();
  await act(async () => { await import('./entry-client'); });

  expect(hydrateRoot).toHaveBeenCalledOnce();
  expect(createRoot).not.toHaveBeenCalled();
  expect(container.querySelector('h1')).toBe(originalHeading);
  expect(screen.getByRole('link', { name: 'children' })).toBeVisible();
  expect(fetchPage).not.toHaveBeenCalled();
  await act(async () => { vi.mocked(hydrateRoot).mock.results[0].value.unmount(); });
  container.remove();
  data.remove();
  window.history.replaceState({}, '', '/');
  vi.unstubAllGlobals();
});
