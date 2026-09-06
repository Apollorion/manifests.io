import { render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { expect, it, vi } from 'vitest';

vi.mock('react-dom/client', async importOriginal => {
  const actual = await importOriginal<typeof import('react-dom/client')>();
  return { ...actual, createRoot: vi.fn(actual.createRoot) };
});
vi.mock('./telemetry', () => ({ captureError: vi.fn(), initializeObservability: vi.fn() }));

it('keeps stale navigation inert until the fresh schema commits', async () => {
  const container = document.createElement('div');
  container.id = 'root';
  container.setAttribute('data-dynamic', 'true');
  container.setAttribute('inert', '');
  container.innerHTML = '<a href="/stale-loop">children</a>';
  const data = document.createElement('script');
  data.id = '__PAGE_DATA__';
  data.type = 'application/json';
  data.textContent = JSON.stringify({
    item: 'example', version: '1', resource: 'Node', title: 'Node.children.children',
    description: '', canonical: '/example/1/Node', catalog: [], breadcrumbs: [],
    otherVersions: [], variants: [],
    resources: [{ name: 'children', type: 'Node', description: 'Child nodes.', circular: true }],
  });
  document.body.append(container, data);
  const scheduleRender = vi.fn<(app: ReactNode) => void>();
  vi.mocked(createRoot).mockImplementationOnce(() => ({ render: scheduleRender }) as unknown as Root);

  await import('./entry-client');

  expect(container).toHaveAttribute('inert');
  expect(container.querySelector('a')).toHaveAttribute('href', '/stale-loop');
  expect(scheduleRender).toHaveBeenCalledOnce();
  render(scheduleRender.mock.calls[0][0], { container });
  expect(container).not.toHaveAttribute('inert');
  expect(container.querySelector('a[href="/stale-loop"]')).toBeNull();
  expect(screen.getByText('Circular reference')).toBeVisible();
  expect(screen.queryByRole('link', { name: 'children' })).not.toBeInTheDocument();
  data.remove();
});
