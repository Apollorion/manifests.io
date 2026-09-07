import { fireEvent, render, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { App, AppBoundary, RecoveryPage } from './App';
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
    { name: 'metadata', type: 'object', description: 'Standard object metadata.', href: '/kubernetes/1.34/io.k8s.apimachinery.pkg.apis.meta.v1.ObjectMeta?path=Pod.metadata', required: true },
    { name: 'spec', type: 'object', description: 'Desired behavior.', href: '/kubernetes/1.34/io.k8s.api.core.v1.PodSpec?path=Pod.spec' },
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
    expect(screen.getByRole('link', { name: 'spec' })).toHaveAttribute('href', '/kubernetes/1.34/io.k8s.api.core.v1.PodSpec?path=Pod.spec');
    expect(within(screen.getByRole('navigation', { name: 'API versions' })).getByRole('link', { name: 'v1' })).toHaveAttribute('href', page.canonical);
    expect(screen.getByRole('link', { name: 'Pending' })).toHaveAttribute('href', page.resources[2].variants![0].href);
    expect(screen.getByText('enum: Ready, Pending')).toBeVisible();
    expect(within(screen.getByRole('navigation', { name: 'Breadcrumb' })).getByRole('link', { name: 'Pod' })).toHaveAttribute('aria-current', 'page');
  });

  it('preserves the current resource and navigation query only within the same product', () => {
    const nested = { ...page, resource: 'io.k8s.api.core.v1.PodSpec', path: 'Deployment.spec.template.spec', trail: 'encoded-history', canonical: '/kubernetes/1.34/io.k8s.api.core.v1.PodSpec' };
    expect(specURL(nested, 'kubernetes', '1.33')).toBe('/kubernetes/1.33/io.k8s.api.core.v1.PodSpec?path=Deployment.spec.template.spec&trail=encoded-history');
    expect(specURL(nested, 'flux', '2.0.1')).toBe('/flux/2.0.1');
    render(<App initialPage={nested}/>);
    expect(screen.getByRole('option', { name: 'kubernetes / 1.33' })).toHaveValue(specURL(nested, 'kubernetes', '1.33'));
  });

  it('identifies the selected API version independently of traversal context', () => {
    const selected = {
      ...page, path: 'Workload.pod',
      otherVersions: [
        { label: 'v1beta1', href: '/kubernetes/1.34/io.k8s.api.core.v1beta1.Pod' },
        { label: 'v1', href: page.canonical },
      ],
    };
    render(<App initialPage={selected}/>);
    const versions = within(screen.getByRole('navigation', { name: 'API versions' }));
    expect(versions.getByRole('link', { name: 'v1' })).toHaveAttribute('aria-current', 'page');
    expect(versions.getByRole('link', { name: 'v1beta1' })).not.toHaveAttribute('aria-current');
  });

  it('keeps the header home in its current product and exposes the footer jump outside the desktop note', () => {
    const { container } = render(<App initialPage={{ ...page, item: 'flux', version: '2.0.1' }}/>);
    expect(screen.getByRole('link', { name: 'Manifests.io home' })).toHaveAttribute('href', '/flux/2.0.1');
    const about = screen.getByRole('link', { name: 'About this project' });
    expect(about).toHaveAttribute('href', '#about');
    expect(about.closest('.sidebar-note')).toBeNull();
    expect(container.querySelector('#about')).toBeInTheDocument();
  });

  it('explains that unlisted definitions can still be opened directly', () => {
    render(<App initialPage={{ ...page, resource: undefined }}/>);
    expect(screen.getByText('Use Search all types to jump to any definition, including nested types not listed below.')).toBeVisible();
  });

  it('preserves an inline selector independently from its displayed navigation path', () => {
    const inline = { ...page, pointer: '/properties/spec', path: 'Workload.spec' };
    expect(specURL(inline, 'kubernetes', '1.33')).toBe('/kubernetes/1.33/io.k8s.api.core.v1.Pod?path=Workload.spec&pointer=%2Fproperties%2Fspec');
    expect(specURL(inline, 'flux', '2.0.1')).toBe('/flux/2.0.1');
  });

  it('stops circular fields and variants with visible explanations while other fields stay navigable', () => {
    const circular = {
      ...page,
      variants: [{ label: 'Recursive schema', href: '', circular: true }],
      resources: [...page.resources, {
        name: 'children', type: 'array', description: 'Nested children.', href: '', circular: true,
        variants: [{ label: 'Recursive child', href: '', circular: true }],
      }],
    };
    const { container } = render(<App initialPage={circular}/>);
    const blocked = container.querySelectorAll('.schema-circular');
    expect(blocked).toHaveLength(3);
    for (const element of blocked) {
      expect(element.querySelector('a, button, [tabindex]')).toBeNull();
      expect(within(element as HTMLElement).getByText('Circular reference')).toBeVisible();
      expect(within(element as HTMLElement).getByText('This schema has already been visited 3 times in this path.')).toBeVisible();
    }
    expect(screen.getByText('children')).toBeVisible();
    expect(screen.getByText('Nested children.')).toBeVisible();
    expect(screen.queryByRole('link', { name: 'children' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Recursive schema' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Recursive child' })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'spec' })).toHaveAttribute('href', page.resources[1].href);
    expect(renderPage(circular)).not.toContain('href=""');
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

  it('retains the specification selector and contextual issue report after a client rendering error', () => {
    function Broken(): never { throw new Error('Rendering failed'); }
    const capture = vi.fn();
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
    try {
      render(<AppBoundary page={page} onError={capture}><Broken/></AppBoundary>);
      expect(capture).toHaveBeenCalledOnce();
      expect(screen.getByRole('alert')).toHaveTextContent('choose another specification or version');
      expect(screen.getByRole('combobox', { name: 'Specification & version' })).toBeVisible();
      expect(screen.getByRole('option', { name: 'kubernetes / 1.33' })).toBeInTheDocument();
      expect(screen.getByRole('option', { name: 'flux / 2.0.1' })).toHaveValue('/flux/2.0.1');
      expect(screen.getByRole('link', { name: 'Browse available resources' })).toHaveAttribute('href', '/kubernetes/1.34');
      const issue = new URL(screen.getByRole('link', { name: 'See an issue here?' }).getAttribute('href')!);
      expect(issue.searchParams.get('title')).toBe('kubernetes - io.k8s.api.core.v1.Pod');
      expect(issue.searchParams.get('body')).toBe('## Description of issue\n');
    } finally { consoleError.mockRestore(); }
  });

  it('offers the same recovery layout when startup only knows the route and catalog', () => {
    render(<RecoveryPage page={{ item: 'flux', version: '2.0.1', catalog: page.catalog }}/>);
    expect(screen.getByRole('option', { name: 'kubernetes / 1.34' })).toHaveValue('/kubernetes/1.34');
    expect(screen.getByRole('link', { name: 'Browse available resources' })).toHaveAttribute('href', '/flux/2.0.1');
    expect(screen.getByRole('link', { name: 'See an issue here?' })).toBeVisible();
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
    const query = new URLSearchParams(pageQuery({ pathname: '/gatewayapi/1.2.0/Foo%2FBar', search: '?path=Foo.spec&pointer=%2Fproperties%2Fspec&trail=encoded-history&oneOf=0&item=ignored' }));
    expect(query.get('item')).toBe('gatewayapi');
    expect(query.get('resource')).toBe('Foo/Bar');
    expect(query.get('path')).toBe('Foo.spec');
    expect(query.get('pointer')).toBe('/properties/spec');
    expect(query.get('trail')).toBe('encoded-history');
    expect(query.get('oneOf')).toBe('0');
  });
});
