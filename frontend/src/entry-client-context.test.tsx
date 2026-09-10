import { act, screen } from '@testing-library/react';
import { createRoot, hydrateRoot } from 'react-dom/client';
import { renderToString } from 'react-dom/server';
import { expect, it, vi } from 'vitest';
import { App, AppBoundary } from './App';
import type { Page } from './types';
import { captureError } from './telemetry';

vi.mock('react-dom/client', async importOriginal => {
  const actual = await importOriginal<typeof import('react-dom/client')>();
  return { ...actual, createRoot: vi.fn(actual.createRoot), hydrateRoot: vi.fn(actual.hydrateRoot) };
});
vi.mock('./telemetry', () => ({ captureError: vi.fn(), captureSearchEvent: vi.fn(), initializeObservability: vi.fn() }));

it('restores this visitor’s traversal from shared HTML before enabling navigation', async () => {
  const canonical: Page = {
    item: 'example', version: '1', resource: 'Node', title: 'Node',
    description: '', canonical: '/example/1/Node', catalog: [], breadcrumbs: [],
    otherVersions: [], variants: [],
    resources: [{ name: 'children', type: 'Node', description: 'Child nodes.', href: '/example/1/Node?path=Node.children' }],
  };
  const contextual: Page = {
    ...canonical, title: 'Node.children.children', path: 'Node.children.children', trail: '{"Node#":2}',
    resources: [{ name: 'children', type: 'Node', description: 'Child nodes.', circular: true }],
  };
  window.history.replaceState({}, '', '/example/1/Node?path=Node.children.children&trail=%7B%22Node%23%22%3A2%7D');
  const container = document.createElement('div');
  container.id = 'root';
  container.innerHTML = renderToString(<AppBoundary page={canonical}><App initialPage={canonical}/></AppBoundary>);
  const data = document.createElement('script');
  data.id = '__PAGE_DATA__';
  data.type = 'application/json';
  data.textContent = JSON.stringify(canonical);
  document.body.append(container, data);
  document.documentElement.inert = true;
  let resolve!: (response: unknown) => void;
  const fetchPage = vi.fn((_url: string) => new Promise(done => { resolve = done; }));
  vi.stubGlobal('fetch', fetchPage);
  try {
    await act(async () => { await import('./entry-client'); });
    expect(container.inert).toBe(true);
    expect(document.documentElement.inert).toBe(true);
    expect(container).toHaveAttribute('aria-busy', 'true');
    const query = new URL(fetchPage.mock.calls[0][0] as string, window.location.origin).searchParams;
    expect(query.get('path')).toBe(contextual.path);
    expect(query.get('trail')).toBe(contextual.trail);
    expect(query.get('resource')).toBe('Node');
    await act(async () => { resolve({ ok: true, json: async () => contextual }); });
    expect(container.inert).toBe(false);
    expect(document.documentElement.inert).toBe(false);
    expect(container).not.toHaveAttribute('aria-busy');
    expect(hydrateRoot).not.toHaveBeenCalled();
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(contextual.title);
    expect(document.title).toBe(`${contextual.title} | Manifests.io`);
    expect(screen.getByText('Circular reference')).toBeVisible();
    expect(screen.queryByRole('link', { name: 'children' })).not.toBeInTheDocument();
    expect(captureError).not.toHaveBeenCalled();
  } finally {
    await act(async () => { vi.mocked(createRoot).mock.results[0]?.value.unmount(); });
    container.remove();
    data.remove();
    window.history.replaceState({}, '', '/');
    vi.unstubAllGlobals();
    document.documentElement.inert = false;
  }
});
