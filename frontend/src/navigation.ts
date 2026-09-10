import type { Link, Page } from './types';

export function specURL(page: Page, item: string, version: string): string {
  const base = `/${encodeURIComponent(item)}/${encodeURIComponent(version)}`;
  if (item !== page.item || !page.resource) return base;
  const query = new URLSearchParams();
  if (page.path) query.set('path', page.path);
  if (page.pointer) query.set('pointer', page.pointer);
  if (page.trail) query.set('trail', page.trail);
  const suffix = query.size ? `?${query}` : '';
  return `${base}/${encodeURIComponent(page.resource)}${suffix}`;
}

export function pageQuery(location: Pick<Location, 'pathname' | 'search'>): string {
  const [item = '', version = '', resource = ''] = location.pathname.split('/').filter(Boolean).map(decodeURIComponent);
  const original = new URLSearchParams(location.search);
  const query = new URLSearchParams();
  for (const key of ['pointer', 'oneOf', 'key']) {
    const value = original.get(key);
    if (value) query.set(key, value);
  }
  query.set('item', item);
  query.set('version', version);
  if (resource) query.set('resource', resource);
  query.sort();
  return query.toString();
}

export function restoreTraversal(page: Page, query: URLSearchParams): Page {
  if (page.error || !page.resource) return page;
  let path = query.get('path') || query.get('linked') || '';
  const trail = query.get('trail') || '';
  if (!path && !trail) return page;
  const invalid = () => new Error('This documentation URL is invalid.');
  const bytes = (value: string) => new TextEncoder().encode(value).length;
  if (bytes(path) > 8192 || bytes(trail) > 4096) throw invalid();
  if (path && query.get('key')) path += `.${query.get('key')}`;
  const cycles = new Set(page.cycles ?? []);
  const counts = new Map<string, number>();
  if (trail) {
    let value: unknown;
    try { value = JSON.parse(trail); } catch { throw invalid(); }
    if (!value || typeof value !== 'object' || Array.isArray(value)) throw invalid();
    const entries = Object.entries(value);
    if (entries.length > 128) throw invalid();
    for (const [node, count] of entries) {
      if (!Number.isInteger(count) || count < 1 || count > 3) throw invalid();
      if (cycles.has(node)) counts.set(node, count);
    }
  }
  const current = `${page.resource}#${page.pointer || ''}`;
  if (cycles.has(current)) {
    const visits = counts.get(current) ?? 0;
    if (visits >= 3) throw invalid();
    counts.set(current, visits + 1);
  }
  const nextTrail = counts.size ? JSON.stringify(Object.fromEntries([...counts].sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0))) : '';
  const title = path || page.title;
  function link<T extends { href?: string; circular?: boolean }>(original: T): T {
    if (!original.href) return original;
    const url = new URL(original.href, 'https://manifests.io');
    const target = `${decodeURIComponent(url.pathname.split('/').at(-1) || '')}#${url.searchParams.get('pointer') || ''}`;
    if (cycles.has(target) && (counts.get(target) ?? 0) >= 3) return { ...original, href: '', circular: true };
    const canonicalPath = url.searchParams.get('path');
    if (canonicalPath !== null && canonicalPath.startsWith(page.title)) {
      url.searchParams.set('path', title + canonicalPath.slice(page.title.length));
    }
    if (nextTrail) url.searchParams.set('trail', nextTrail);
    else url.searchParams.delete('trail');
    url.searchParams.sort();
    return { ...original, href: url.pathname + url.search, circular: false };
  }
  const breadcrumbs: Link[] = page.breadcrumbs.slice(0, 2);
  const rootName = page.resource.split(/[./]/).at(-1);
  if (title !== rootName) {
    const url = new URL(page.canonical, 'https://manifests.io');
    url.searchParams.set('path', title);
    if (trail) url.searchParams.set('trail', trail);
    url.searchParams.sort();
    breadcrumbs.push({ label: title, href: url.pathname + url.search });
  }
  return {
    ...page, title, path, trail, breadcrumbs,
    variants: page.variants?.map(link) ?? page.variants,
    resources: page.resources.map(row => ({
      ...link(row),
      name: page.leaf ? title : row.name,
      variants: row.variants?.map(link),
    })),
  };
}
