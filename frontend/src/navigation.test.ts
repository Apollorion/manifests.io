import { describe, expect, it } from 'vitest';
import { pageQuery, restoreTraversal } from './navigation';
import type { Page } from './types';
import fixtures from './fixtures/traversal.json';

function semantics(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(semantics);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value)
    .filter(([, entry]) => entry !== undefined && entry !== '' && entry !== false)
    .map(([key, entry]) => [key, semantics(entry)]));
  return value;
}

const canonical: Page = {
  item: 'example', version: '1', resource: 'Node', title: 'Node',
  description: 'A recursive node.', canonical: '/example/1/Node',
  catalog: [], otherVersions: [], variants: [], cycles: ['Node#'],
  breadcrumbs: [{ label: 'example 1', href: '/example/1' }, { label: 'Node', href: '/example/1/Node' }],
  resources: [
    { name: 'children', type: 'Node[]', description: '', href: '/example/1/Node?path=Node.children&trail=%7B%22Node%23%22%3A1%7D' },
    { name: 'other', type: 'Other', description: '', href: '/example/1/Other?path=Node.other&trail=%7B%22Node%23%22%3A1%7D' },
  ],
};

describe('browser traversal over canonical data', () => {
  it.each(fixtures)('matches the Go traversal contract: $name', fixture => {
    expect(semantics(restoreTraversal(fixture.canonical as Page, new URLSearchParams(fixture.search)))).toEqual(semantics(fixture.expected));
  });

  it('reuses canonical pages unchanged and builds only semantic API queries', () => {
    expect(restoreTraversal(canonical, new URLSearchParams())).toBe(canonical);
    expect(pageQuery({ pathname: '/example/1/Node', search: '?path=Workload.node&trail=ignored&utm_source=test&pointer=%2Fproperties%2Fspec' }))
      .toBe('item=example&pointer=%2Fproperties%2Fspec&resource=Node&version=1');
  });

  it('changes headings and links without mutating shared data', () => {
    const before = structuredClone(canonical);
    const page = restoreTraversal(canonical, new URLSearchParams({ linked: 'Workload.spec' }));
    expect(page.title).toBe('Workload.spec');
    expect(page.path).toBe('Workload.spec');
    expect(page.canonical).toBe(canonical.canonical);
    expect(new URL(page.resources[0].href!, 'https://example.test').searchParams.get('path')).toBe('Workload.spec.children');
    expect(page.breadcrumbs.at(-1)?.label).toBe('Workload.spec');
    expect(canonical).toEqual(before);
  });

  it('blocks the fourth visit while preserving other fields, refresh and incoming history', () => {
    const query = new URLSearchParams({ path: 'Node.children.children', trail: '{"Node#":2,"unknown#":1}' });
    const page = restoreTraversal(canonical, query);
    expect(page.resources[0]).toMatchObject({ circular: true, href: '' });
    const other = new URL(page.resources[1].href!, 'https://example.test');
    expect(other.searchParams.get('path')).toBe('Node.children.children.other');
    expect(other.searchParams.get('trail')).toBe('{"Node#":3}');
    expect(page.trail).toBe(query.get('trail'));
    expect(restoreTraversal(canonical, query)).toEqual(page);
  });

  it('applies cycle limits to variants and tracks canonical inline identities', () => {
    const inline: Page = {
      ...canonical, pointer: '/properties/spec', title: 'Node.spec',
      cycles: ['Node#/properties/spec'], canonical: '/example/1/Node?pointer=%2Fproperties%2Fspec',
      variants: [{ label: 'nested', href: '/example/1/Node?path=Node.spec&pointer=%2Fproperties%2Fspec' }],
    };
    const page = restoreTraversal(inline, new URLSearchParams({ path: 'Workload.spec', trail: '{"Node#/properties/spec":2}' }));
    expect(page.variants[0]).toMatchObject({ circular: true, href: '' });
    expect(page.resources[0].circular).toBe(false);
  });

  it('restores legacy oneOf context and leaf names', () => {
    const leaf: Page = { ...canonical, leaf: true, title: 'Node.mode (oneOf 1)', resources: [{ name: 'Node.mode (oneOf 1)', type: 'string', description: '' }] };
    const page = restoreTraversal(leaf, new URLSearchParams({ path: 'Workload.spec', key: 'mode', oneOf: 'Mode' }));
    expect(page.title).toBe('Workload.spec.mode');
    expect(page.resources[0].name).toBe(page.title);
  });

  it.each(['null', '[]', '{', '{"Node#":0}', '{"Node#":-1}', '{"Node#":3}', '{"Node#":4}', '{"Node#":1.5}', '{"Node#":"2"}', 'x'.repeat(4097)])('rejects invalid or exhausted history %s', trail => {
    expect(() => restoreTraversal(canonical, new URLSearchParams({ trail }))).toThrow('This documentation URL is invalid.');
  });

  it('bounds traversal size and history entries', () => {
    expect(() => restoreTraversal(canonical, new URLSearchParams({ path: 'é'.repeat(4097) }))).toThrow();
    const trail = JSON.stringify(Object.fromEntries(Array.from({ length: 129 }, (_, i) => [`${i}#`, 1])));
    expect(() => restoreTraversal(canonical, new URLSearchParams({ trail }))).toThrow();
  });
});
