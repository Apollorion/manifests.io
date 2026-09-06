import { Component, useEffect, useRef, useState, type ReactNode } from 'react';
import { specURL } from './navigation';
import type { Link, Page, Row } from './types';

const repository = 'https://github.com/TheOutdoorProgrammer/manifests.io';

function ThemeButton() {
  const [dark, setDark] = useState(false);
  useEffect(() => {
    let saved: string | null = null;
    try { saved = localStorage.getItem('theme'); } catch { /* Storage can be disabled by the browser. */ }
    const enabled = saved ? saved === 'dark' : window.matchMedia('(prefers-color-scheme: dark)').matches;
    setDark(enabled);
    document.documentElement.dataset.theme = enabled ? 'dark' : 'light';
  }, []);

  function toggle() {
    const next = !dark;
    setDark(next);
    document.documentElement.dataset.theme = next ? 'dark' : 'light';
    try { localStorage.setItem('theme', next ? 'dark' : 'light'); } catch { /* Theme still works without persistent storage. */ }
  }

  return <button className="theme-button" onClick={toggle} aria-label={`Switch to ${dark ? 'light' : 'dark'} theme`}>
    <span aria-hidden="true">{dark ? '☀' : '◐'}</span>
  </button>;
}

function SchemaLink({ label, href, circular }: { label: string; href?: string; circular?: boolean }) {
  if (circular) return <span className="schema-circular">
    <span>{label}</span>
    <span className="circular-label"><span aria-hidden="true">× </span>Circular reference</span>
    <span className="circular-explanation">This schema has already been visited 3 times in this path.</span>
  </span>;
  return href ? <a href={href}>{label}<span className="field-arrow" aria-hidden="true"> ↗</span></a> : <>{label}</>;
}

function LinkList({ links, label }: { links: Link[]; label: string }) {
  if (!links?.length) return null;
  return <nav className="related-links" aria-label={label}>
    <span className="eyebrow">{label}</span>
    <ul>{links.map((link, index) => <li key={`${link.href}-${link.label}-${index}`}><SchemaLink {...link}/></li>)}</ul>
  </nav>;
}

function FieldRow({ row }: { row: Row }) {
  return <tr>
    <th scope="row">
      <div className="field-name"><SchemaLink label={row.name} href={row.href} circular={row.circular}/></div>
      <div className="field-meta"><code>{row.type || 'schema'}</code>{row.required && <span className="required"><span aria-hidden="true">*</span> required</span>}</div>
    </th>
    <td>
      {row.description ? <p className="description">{row.description}</p> : <p className="undocumented">No description provided.</p>}
      {!!row.constraints?.length && <ul className="constraints" aria-label={`${row.name} constraints`}>{row.constraints.map((constraint, index) => <li key={index}><code>{constraint}</code></li>)}</ul>}
      <LinkList links={row.variants ?? []} label={`${row.name} variants`} />
    </td>
  </tr>;
}

function SchemaTable({ page }: { page: Page }) {
  const [query, setQuery] = useState('');
  const search = useRef<HTMLInputElement>(null);
  const label = page.resource ? 'fields' : 'resources';
  const rows = page.resources ?? [];
  const filtered = rows.filter(row => row.name.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()));

  useEffect(() => {
    function focusSearch(event: KeyboardEvent) {
      const target = event.target as HTMLElement;
      if (event.key === '/' && !event.metaKey && !event.ctrlKey && !event.altKey && !target.isContentEditable && !['INPUT', 'SELECT', 'TEXTAREA'].includes(target.tagName)) {
        event.preventDefault();
        search.current?.focus();
      }
    }
    document.addEventListener('keydown', focusSearch);
    return () => document.removeEventListener('keydown', focusSearch);
  }, []);

  return <section className="schema-panel" aria-label={`${page.title} ${label}`}>
    <div className="table-toolbar">
      <div className="search-wrap">
        <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7"><circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/></svg>
        <label className="sr-only" htmlFor="field-search">Filter {label}</label>
        <input ref={search} id="field-search" type="search" placeholder={`Filter ${label}…`} value={query} onChange={event => setQuery(event.target.value)} onKeyDown={event => { if (event.key === 'Escape') setQuery(''); }} autoComplete="off" spellCheck={false}/>
        <kbd aria-hidden="true">/</kbd>
      </div>
      <span className="result-count" role="status" aria-live="polite">{filtered.length === rows.length ? `${rows.length} ${label}` : `${filtered.length} of ${rows.length} ${label}`}</span>
    </div>
    {filtered.length ? <table>
      <caption className="sr-only">{page.title} {label} and descriptions</caption>
      <thead><tr><th scope="col">{page.resource ? 'Field / Type' : 'Resource / Type'}</th><th scope="col">Description</th></tr></thead>
      <tbody>{filtered.map(row => <FieldRow key={row.name} row={row}/>)}</tbody>
    </table> : <div className="empty-state">
      <span className="empty-symbol" aria-hidden="true">∅</span>
      <h2>{query ? `No ${label} match “${query}”` : `No ${label} to display`}</h2>
      <p>{query ? 'Try a shorter name or clear your filter.' : 'This schema has no documented child fields.'}</p>
      {query && <button className="action-button" onClick={() => { setQuery(''); search.current?.focus(); }}>Clear filter</button>}
    </div>}
  </section>;
}

export function App({ initialPage: page }: { initialPage: Page }) {
  const listURL = `/${encodeURIComponent(page.item)}/${encodeURIComponent(page.version)}`;
  const issueURL = `${repository}/issues/new?${new URLSearchParams({ title: page.resource ? `${page.item} - ${page.resource}` : page.item, body: '## Description of issue\n' })}`;
  return <>
    <a className="skip-link" href="#main">Skip to documentation</a>
    <header className="site-header">
      <div className="header-inner">
        <a className="brand" href="/" aria-label="Manifests.io home"><span className="brand-mark" aria-hidden="true">{'{m}'}</span><span>manifests<span className="brand-domain">.io</span></span></a>
        <span className="header-tagline">Kubernetes, documented.</span>
        <div className="header-actions"><a className="github-link" href={repository}>GitHub <span aria-hidden="true">↗</span></a><ThemeButton/></div>
      </div>
    </header>
    <div className="workspace">
      <aside className="workspace-sidebar" aria-label="Documentation context">
        <div className="sidebar-inner">
          <span className="eyebrow">Reference library</span>
          <h2>Find your spec.</h2>
          <p className="sidebar-description">Resources, fields, and the details that make your manifests work.</p>
          <label className="select-label" htmlFor="spec">Specification &amp; version</label>
          <select id="spec" value={specURL(page, page.item, page.version)} onChange={event => window.location.assign(event.target.value)}>
            {(page.catalog ?? []).map(product => <optgroup key={product.name} label={product.name}>{product.versions.map(version => <option key={version} value={specURL(page, product.name, version)}>{product.name} / {version}</option>)}</optgroup>)}
          </select>
          <a className={`resource-nav ${!page.resource && !page.error ? 'active' : ''}`} href={listURL}><span aria-hidden="true">▦</span> All resources<span aria-hidden="true">↗</span></a>
          <div className="sidebar-note"><span className="notation" aria-hidden="true">spec:</span><p>Explore a resource, then follow its fields to see what goes inside.</p><a href="#about">About this project</a></div>
        </div>
      </aside>
      <main id="main" tabIndex={-1}>
        <nav className="breadcrumbs" aria-label="Breadcrumb"><ol>{(page.breadcrumbs ?? []).map((link, index, links) => <li key={`${link.href}-${index}`}><a href={link.href} aria-current={index === links.length - 1 ? 'page' : undefined}>{link.label}</a></li>)}</ol></nav>
        <div className="page-heading"><div><span className="eyebrow">{page.item} <span className="version-tag">v{page.version}</span></span><h1>{page.title || 'Documentation'}</h1></div><span className="page-symbol" aria-hidden="true">{page.resource ? '{}' : '[]'}</span></div>
        {page.description && <p className="page-description">{page.description}</p>}
        {page.error ? <section className="error-state" role="alert"><h2>We couldn’t open this schema.</h2><p>{page.error}</p><a className="action-button" href={listURL}>Browse available resources</a></section> : <>
          <LinkList links={page.otherVersions ?? []} label="API versions"/>
          <LinkList links={page.variants ?? []} label="Schema variants"/>
          <SchemaTable key={page.canonical} page={page}/>
        </>}
        <div className="page-footer"><span>{page.resource ? <><span className="required">*</span> marks a required field</> : 'Select a resource to explore its schema.'}</span><a href={issueURL}>See an issue here? <span aria-hidden="true">↗</span></a></div>
      </main>
    </div>
    <footer id="about" className="site-footer"><div><a className="footer-brand" href="/">manifests.io</a><p>Easy to use Kubernetes documentation.</p></div><div className="credits"><p><a href={repository}>View in GitHub</a><span aria-hidden="true"> · </span>K8s Is Awesome<span aria-hidden="true"> · </span>Made with <span className="heart" role="img" aria-label="love">♥</span></p><p>Authored by <a href="https://github.com/TheOutdoorProgrammer/">TheOutdoorProgrammer</a></p></div></footer>
  </>;
}

export class AppBoundary extends Component<{ children: ReactNode; onError?: (error: unknown) => void }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() { return { failed: true }; }
  componentDidCatch(error: Error) { this.props.onError?.(error); }
  render() {
    return this.state.failed ? <main className="boot-error" role="alert"><h1>Something went wrong.</h1><p>Reload the page to try again, or return to the resource library.</p><a className="action-button" href="/">Open the library</a></main> : this.props.children;
  }
}
