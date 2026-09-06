import type { Page } from './types';

export function specURL(page: Page, item: string, version: string): string {
  const base = `/${encodeURIComponent(item)}/${encodeURIComponent(version)}`;
  if (item !== page.item || !page.resource) return base;
  const query = new URLSearchParams();
  if (page.path) query.set('path', page.path);
  if (page.linked) query.set('linked', page.linked);
  const suffix = query.size ? `?${query}` : '';
  return `${base}/${encodeURIComponent(page.resource)}${suffix}`;
}

export function pageQuery(location: Pick<Location, 'pathname' | 'search'>): string {
  const [item = '', version = '', resource = ''] = location.pathname.split('/').filter(Boolean).map(decodeURIComponent);
  const query = new URLSearchParams(location.search);
  query.set('item', item);
  query.set('version', version);
  if (resource) query.set('resource', resource);
  return query.toString();
}
