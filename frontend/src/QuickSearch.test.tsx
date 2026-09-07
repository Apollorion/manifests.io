import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { QuickSearch, type Definition } from './QuickSearch';

const definitions: Definition[] = [
  { name: 'Container', resource: 'io.k8s.api.core.v1.Container', href: '/kubernetes/1.34/io.k8s.api.core.v1.Container' },
  { name: 'ContainerStatus', resource: 'io.k8s.api.core.v1.ContainerStatus', href: '/kubernetes/1.34/io.k8s.api.core.v1.ContainerStatus' },
  { name: 'Pod', resource: 'io.k8s.api.core.v1.Pod', href: '/kubernetes/1.34/io.k8s.api.core.v1.Pod' },
];

beforeEach(() => {
  Object.defineProperty(HTMLDialogElement.prototype, 'showModal', { configurable: true, value: function (this: HTMLDialogElement) { this.open = true; } });
  Object.defineProperty(HTMLDialogElement.prototype, 'close', { configurable: true, value: function (this: HTMLDialogElement) { this.open = false; this.dispatchEvent(new Event('close')); } });
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: async () => definitions }));
});
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

describe('quick type search', () => {
  it('loads lazily, finds nested types with typos, and never transmits the search text', async () => {
    const onEvent = vi.fn();
    render(<QuickSearch item="kubernetes" version="1.34" onEvent={onEvent}/>);
    expect(fetch).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Search all types' }));
    const input = screen.getByRole('combobox', { name: 'Search all types' });
    expect(input).toHaveFocus();
    await screen.findByRole('option', { name: /ContainerStatus/ });
    fireEvent.change(input, { target: { value: 'ContanerStatus' } });
    const result = screen.getAllByRole('option')[0];
    expect(result).toHaveTextContent('ContainerStatus');
    expect(result).toHaveAttribute('href', definitions[1].href);
    expect(input).toHaveAttribute('aria-activedescendant', result.id);
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(fetch).toHaveBeenCalledWith('/api/definitions?item=kubernetes&version=1.34', expect.objectContaining({ signal: expect.any(AbortSignal) }));
    fireEvent.keyDown(input, { key: 'Escape' });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Search all types' }));
    expect(input).toHaveValue('ContanerStatus');
    expect(fetch).toHaveBeenCalledTimes(1);
    expect(onEvent.mock.calls).toEqual([['search_open'], ['search_open']]);
  });

  it('supports shortcuts, arrow selection and Enter with real direct links', async () => {
    const onEvent = vi.fn();
    render(<QuickSearch item="kubernetes" version="1.34" onEvent={onEvent}/>);
    fireEvent.keyDown(document.body, { key: 'k', ctrlKey: true });
    const input = screen.getByRole('combobox', { name: 'Search all types' });
    await screen.findByRole('option', { name: /ContainerStatus/ });
    fireEvent.keyDown(input, { key: 'ArrowDown' });
    const result = screen.getByRole('option', { name: /ContainerStatus/ });
    expect(result).toHaveAttribute('aria-selected', 'true');
    result.addEventListener('click', event => event.preventDefault());
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(onEvent).toHaveBeenLastCalledWith('search_select');
    fireEvent.keyDown(input, { key: 'Escape' });
    fireEvent.keyDown(document.body, { key: 'p', metaKey: true });
    expect(screen.getByRole('dialog')).toBeVisible();
    expect(input).toHaveFocus();
  });

  it('recovers from failed requests and empty matches', async () => {
    vi.mocked(fetch).mockResolvedValueOnce({ ok: false } as Response);
    const onEvent = vi.fn();
    render(<QuickSearch item="kubernetes" version="1.34" onEvent={onEvent}/>);
    fireEvent.click(screen.getByRole('button', { name: 'Search all types' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Retry loading types' }));
    await screen.findByRole('option', { name: /ContainerStatus/ });
    fireEvent.change(screen.getByRole('combobox', { name: 'Search all types' }), { target: { value: 'zzzzzzzzzzzz' } });
    expect(screen.getByRole('status')).toHaveTextContent('No matching types');
    expect(screen.queryAllByRole('option')).toHaveLength(0);
    expect(onEvent.mock.calls).toEqual([['search_open'], ['search_load_failed']]);
  });

  it('ignores completion after closing and retries on reopening', async () => {
    let resolve!: (response: Response) => void;
    vi.mocked(fetch).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
    render(<QuickSearch item="kubernetes" version="1.34"/>);
    fireEvent.click(screen.getByRole('button', { name: 'Search all types' }));
    fireEvent.click(screen.getByRole('button', { name: 'Close type search' }));
    resolve({ ok: true, json: async () => [] } as unknown as Response);
    await waitFor(() => expect(vi.mocked(fetch).mock.calls[0][1]?.signal?.aborted).toBe(true));
    fireEvent.click(screen.getByRole('button', { name: 'Search all types' }));
    await screen.findByRole('option', { name: /ContainerStatus/ });
  });
});
