import { fireEvent, render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { App } from './App';
import { render as renderPage } from './entry-server';
import { pageQuery, specURL } from './navigation';
import type { Page } from './types';

const page: Page = {
  item: 'kubernetes', version: '1.34', resource: 'io.k8s.api.core.v1.Pod', title: 'Pod',
  description: 'A group of containers.', canonical: '/kubernetes/1.34/io.k8s.api.core.v1.Pod',
  catalog: [{ name: 'kubernetes', versions: ['1.34', '1.33'] }, { name: 'flux', versions: ['2.0.1'] }],
  breadcrumbs: [{ label: 'Kubernetes', href: '/kubernetes/1.34' }, { label: 'Pod', href: '/kubernetes/1.34/io.k8s.api.core.v1.Pod' }],
  otherVersions: [{ label: 'v1', href: '/kubernetes/1.34/io.k8s.api.core.v1.Pod' }],
  variants: [],
  resources: [
    { name: 'metadata', type: 'object', description: 'Standard object metadata.', href: '/kubernetes/1.34/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta', required: true },
    { name: 'spec', type: 'object', description: 'Desired behavior.', href: '/kubernetes/1.34/io.k8s.api.core.v1.PodSpec' },
    { name: 'status', type: 'string', description: 'Observed status.', constraints: ['enum: Ready, Pending'], variants: [{ label: 'Pending', href: '/kubernetes/1.34/io.k8s.api.core.v1.Pod?oneOf=Pending&key=status' }] },
  ],
};

describe('schema browser', () => {
  it('filters by name case-insensitively and recovers from an empty result', () => {
    render(<App initialPage={page}/>);
    const filter = screen.getByRole('searchbox', { name: 'Filter fields' });
    fireEvent.change(filter, { target: { value: 'META' } });
    expect(screen.getByRole('status')).toHaveTextContent('1 of 3 fields');
    expect(screen.getByRole('link', { name: 'metadata' })).toBeVisible();
    expect(screen.queryByRole('link', { name: 'spec' })).not.toBeInTheDocument();
    fireEvent.change(filter, { target: { value: 'does not exist' } });
    expect(screen.getByRole('heading', { name: 'No fields match “does not exist”' })).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: 'Clear filter' }));
    expect(screen.getByRole('status')).toHaveTextContent('3 fields');
    expect(filter).toHaveFocus();
  });

  it('exposes required fields, nested schema navigation, GVK links and union choices', () => {
    render(<App initialPage={page}/>);
    expect(screen.getByText('required')).toBeVisible();
    expect(screen.getByRole('link', { name: 'spec' })).toHaveAttribute('href', '/kubernetes/1.34/io.k8s.api.core.v1.PodSpec');
    expect(within(screen.getByRole('navigation', { name: 'API versions' })).getByRole('link', { name: 'v1' })).toHaveAttribute('href', page.canonical);
    expect(screen.getByRole('link', { name: 'Pending' })).toHaveAttribute('href', page.resources[2].variants![0].href);
    expect(screen.getByText('enum: Ready, Pending')).toBeVisible();
    expect(within(screen.getByRole('navigation', { name: 'Breadcrumb' })).getByRole('link', { name: 'Pod' })).toHaveAttribute('aria-current', 'page');
  });

  it('preserves the current resource and navigation query only within the same product', () => {
    const nested = { ...page, path: '/properties/spec', linked: 'Pod.spec', canonical: '/kubernetes/1.34/io.k8s.api.core.v1.PodSpec' };
    expect(specURL(nested, 'kubernetes', '1.33')).toBe('/kubernetes/1.33/io.k8s.api.core.v1.Pod?path=%2Fproperties%2Fspec&linked=Pod.spec');
    expect(specURL(nested, 'flux', '2.0.1')).toBe('/flux/2.0.1');
    render(<App initialPage={nested}/>);
    expect(screen.getByRole('option', { name: 'kubernetes / 1.33' })).toHaveValue(specURL(nested, 'kubernetes', '1.33'));
  });

  it('renders untrusted descriptions as text in browser and server output', () => {
    const hostile = '<img src=x onerror=alert(1)>';
    const unsafe = { ...page, description: hostile, resources: [{ name: 'test', type: 'string', description: hostile }] };
    const { container } = render(<App initialPage={unsafe}/>);
    expect(container.querySelector('img')).toBeNull();
    const html = renderPage(unsafe);
    expect(html).toContain('&lt;img src=x onerror=alert(1)&gt;');
    expect(html).not.toContain('<img src=x');
  });

  it('offers recovery when a schema is missing', () => {
    render(<App initialPage={{ ...page, error: 'Unknown resource.' }}/>);
    expect(screen.getByRole('alert')).toHaveTextContent('Unknown resource.');
    expect(screen.getByRole('link', { name: 'Browse available resources' })).toHaveAttribute('href', '/kubernetes/1.34');
    expect(screen.queryByRole('searchbox')).not.toBeInTheDocument();
  });

  it('supports keyboard focus, Escape clearing, and persisted theme choice', () => {
    render(<App initialPage={page}/>);
    fireEvent.keyDown(document.body, { key: '/' });
    const filter = screen.getByRole('searchbox');
    expect(filter).toHaveFocus();
    fireEvent.change(filter, { target: { value: 'meta' } });
    fireEvent.keyDown(filter, { key: 'Escape' });
    expect(filter).toHaveValue('');
    fireEvent.click(screen.getByRole('button', { name: 'Switch to dark theme' }));
    expect(document.documentElement.dataset.theme).toBe('dark');
    expect(localStorage.getItem('theme')).toBe('dark');
  });

  it('builds API requests from the route and retains schema navigation parameters', () => {
    const query = new URLSearchParams(pageQuery({ pathname: '/gatewayapi/1.2.0/Foo%2FBar', search: '?path=%2Fproperties%2Fspec&oneOf=0&item=ignored' }));
    expect(query.get('item')).toBe('gatewayapi');
    expect(query.get('resource')).toBe('Foo/Bar');
    expect(query.get('path')).toBe('/properties/spec');
    expect(query.get('oneOf')).toBe('0');
  });
});
