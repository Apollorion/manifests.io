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
vi.mock('./telemetry', () => ({ captureError: vi.fn(), initializeObservability: vi.fn() }));

it('hydrates contextual server markup with the circular limit already rendered', async () => {
  const page: Page = {
    item: 'example', version: '1', resource: 'Node', title: 'Node.children.children',
    path: 'Node.children.children', trail: '{"Node#":2}',
    description: '', canonical: '/example/1/Node', catalog: [], breadcrumbs: [],
    otherVersions: [], variants: [],
    resources: [{ name: 'children', type: 'Node', description: 'Child nodes.', circular: true }],
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
  expect(screen.getByText('Circular reference')).toBeVisible();
  expect(screen.queryByRole('link', { name: 'children' })).not.toBeInTheDocument();
  await act(async () => { await import('./entry-client'); });

  expect(hydrateRoot).toHaveBeenCalledOnce();
  expect(createRoot).not.toHaveBeenCalled();
  expect(container.querySelector('h1')).toBe(originalHeading);
  expect(screen.getByText('Circular reference')).toBeVisible();
  expect(screen.queryByRole('link', { name: 'children' })).not.toBeInTheDocument();
  await act(async () => { vi.mocked(hydrateRoot).mock.results[0].value.unmount(); });
  container.remove();
  data.remove();
});
