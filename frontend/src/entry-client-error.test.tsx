import { act, screen } from '@testing-library/react';
import { createRoot } from 'react-dom/client';
import { expect, it, vi } from 'vitest';
import { captureError } from './telemetry';

vi.mock('react-dom/client', async importOriginal => {
  const actual = await importOriginal<typeof import('react-dom/client')>();
  return { ...actual, createRoot: vi.fn(actual.createRoot) };
});
vi.mock('./telemetry', () => ({ captureError: vi.fn(), captureSearchEvent: vi.fn(), initializeObservability: vi.fn() }));

it('recovers from invalid startup data with the route, full catalog, and issue reporting', async () => {
  const previousURL = window.location.href;
  window.history.replaceState({}, '', '/flux/2.0.1/HelmRelease');
  const container = document.createElement('div');
  container.id = 'root';
  const data = document.createElement('script');
  data.id = '__PAGE_DATA__';
  data.type = 'application/json';
  data.textContent = '{';
  document.body.append(container, data);
  const fetchCatalog = vi.fn().mockResolvedValue({ ok: true, json: async () => [
    { name: 'flux', versions: ['2.0.1', '0.31.2'] },
    { name: 'kubernetes', versions: ['1.34'] },
  ] });
  vi.stubGlobal('fetch', fetchCatalog);
  try {
    await act(async () => { await import('./entry-client'); });
    expect(captureError).toHaveBeenCalledOnce();
    expect(fetchCatalog).toHaveBeenCalledWith('/api/catalog');
    expect(screen.getByRole('alert')).toBeVisible();
    expect(screen.getByRole('option', { name: 'flux / 0.31.2' })).toHaveValue('/flux/0.31.2/HelmRelease');
    expect(screen.getByRole('option', { name: 'kubernetes / 1.34' })).toHaveValue('/kubernetes/1.34');
    expect(screen.getByRole('link', { name: 'Manifests.io home' })).toHaveAttribute('href', '/flux/2.0.1');
    const issue = new URL(screen.getByRole('link', { name: 'See an issue here?' }).getAttribute('href')!);
    expect(issue.searchParams.get('title')).toBe('flux - HelmRelease');
  } finally {
    await act(async () => { vi.mocked(createRoot).mock.results[0]?.value.unmount(); });
    container.remove();
    data.remove();
    window.history.replaceState({}, '', previousURL);
    vi.unstubAllGlobals();
  }
});
