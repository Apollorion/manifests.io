import { act, screen } from '@testing-library/react';
import { createRoot } from 'react-dom/client';
import { expect, it, vi } from 'vitest';
import { type TransportItem, TransportItemType } from '@grafana/faro-web-sdk';

vi.mock('react-dom/client', async importOriginal => {
  const actual = await importOriginal<typeof import('react-dom/client')>();
  return { ...actual, createRoot: vi.fn(actual.createRoot) };
});
const { items } = vi.hoisted(() => ({ items: [] as unknown[] }));
vi.mock('./telemetry', async importOriginal => {
  const actual = await importOriginal<typeof import('./telemetry')>();
  const sdk = await import('@grafana/faro-web-sdk');
  class CaptureTransport extends sdk.BaseTransport {
    name = 'startup-capture';
    version = '1';
    initialize() {}
    send(batch: TransportItem | TransportItem[]) {
      items.push(...JSON.parse(JSON.stringify(Array.isArray(batch) ? batch : [batch])));
    }
  }
  let faro: ReturnType<typeof sdk.initializeFaro> | undefined;
  return {
    ...actual,
    initializeObservability: () => {
      faro = sdk.initializeFaro({
        ...actual.telemetryConfig('http://localhost/collect', 'startup-test'),
        url: undefined,
        transports: [new CaptureTransport()],
        instrumentations: [],
        batching: { enabled: false },
        isolate: true,
        preventGlobalExposure: true,
      });
    },
    captureError: (error: unknown) => faro?.api.pushError(error instanceof Error ? error : new Error('Browser error')),
  };
});

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
    expect((items as TransportItem[]).filter(item => item.type === TransportItemType.EXCEPTION)).toHaveLength(1);
    expect(JSON.stringify(items)).toContain('startup-test');
    expect(JSON.stringify(items)).toContain('Browser error');
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
