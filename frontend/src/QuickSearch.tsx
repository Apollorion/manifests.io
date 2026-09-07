import { useEffect, useMemo, useRef, useState } from 'react';
import Fuse from 'fuse.js';

export interface Definition {
  name: string;
  resource: string;
  href: string;
}

export type SearchEvent = 'search_open' | 'search_select' | 'search_load_failed';

export function QuickSearch({ item, version, onEvent }: { item: string; version: string; onEvent?: (event: SearchEvent) => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const input = useRef<HTMLInputElement>(null);
  const [enabled, setEnabled] = useState(false);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [definitions, setDefinitions] = useState<Definition[] | null>(null);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [active, setActive] = useState(0);
  const [shortcut, setShortcut] = useState('Ctrl K');
  const index = useMemo(() => definitions && new Fuse(definitions, {
    keys: [{ name: 'name', weight: 3 }, 'resource'], threshold: 0.3, ignoreLocation: true,
  }), [definitions]);
  const matches = useMemo(() => query.trim() ? index?.search(query.trim()).map(result => result.item) ?? [] : definitions ?? [], [index, definitions, query]);
  const visible = matches.slice(0, 50);

  function show() {
    if (dialog.current?.open) { input.current?.focus(); return; }
    dialog.current?.showModal();
    setOpen(true);
    input.current?.focus();
    onEvent?.('search_open');
  }

  useEffect(() => {
    setEnabled(true);
    if (/Mac|iPhone|iPad/.test(navigator.platform)) setShortcut('⌘ K');
    function keydown(event: KeyboardEvent) {
      if (event.defaultPrevented || event.isComposing || event.repeat || event.altKey || event.shiftKey) return;
      if ((event.metaKey || event.ctrlKey) && ['k', 'p'].includes(event.key.toLowerCase())) {
        event.preventDefault();
        show();
      }
    }
    document.addEventListener('keydown', keydown);
    return () => document.removeEventListener('keydown', keydown);
  }, [onEvent]);

  useEffect(() => {
    if (!open || definitions) return;
    const controller = new AbortController();
    setFailed(false);
    void (async () => {
      try {
        const response = await fetch(`/api/definitions?${new URLSearchParams({ item, version })}`, { signal: controller.signal });
        if (!response.ok) throw new Error('Definitions unavailable');
        const data: Definition[] = await response.json();
        if (!Array.isArray(data) || data.some(entry => typeof entry.name !== 'string' || typeof entry.resource !== 'string' || typeof entry.href !== 'string')) throw new Error('Invalid definitions');
        if (!controller.signal.aborted) setDefinitions(data);
      } catch {
        if (!controller.signal.aborted) { setFailed(true); onEvent?.('search_load_failed'); }
      }
    })();
    return () => controller.abort();
  }, [open, definitions, item, version, attempt, onEvent]);

  useEffect(() => {
    if (open) document.getElementById(`type-result-${active}`)?.scrollIntoView?.({ block: 'nearest' });
  }, [active, open, query]);

  return <>
    <button className="quick-search-trigger" disabled={!enabled} onClick={show} aria-haspopup="dialog" aria-keyshortcuts="Control+k Meta+k Control+p Meta+p">
      <span aria-hidden="true">⌕</span><span>Search all types</span><kbd aria-hidden="true">{shortcut}</kbd>
    </button>
    <dialog ref={dialog} className="quick-search-dialog" aria-labelledby="type-search-title" onClose={() => setOpen(false)} onClick={event => { if (event.target === event.currentTarget) dialog.current?.close(); }}>
      <div className="quick-search-content">
        <div className="quick-search-heading"><div><h2 id="type-search-title">Jump to a type</h2><p>{item} / {version}, including nested types</p></div><button className="quick-search-close" aria-label="Close type search" onClick={() => dialog.current?.close()}>×</button></div>
        <label className="sr-only" htmlFor="type-search">Search all types</label>
        <input ref={input} id="type-search" type="search" role="combobox" aria-autocomplete="list" aria-expanded={open} aria-controls="type-search-results" aria-activedescendant={visible[active] ? `type-result-${active}` : undefined} aria-describedby="type-search-help" placeholder="Try ContainerStatus or PodSpec…" autoComplete="off" spellCheck={false} value={query} onChange={event => { setQuery(event.target.value); setActive(0); }} onKeyDown={event => {
          if (event.nativeEvent.isComposing) return;
          if (event.key === 'Escape') {
            event.preventDefault();
            dialog.current?.close();
          } else if (['ArrowDown', 'ArrowUp'].includes(event.key)) {
            event.preventDefault();
            setActive(current => visible.length ? (current + (event.key === 'ArrowDown' ? 1 : -1) + visible.length) % visible.length : 0);
          } else if (event.key === 'Enter' && visible[active]) {
            event.preventDefault();
            document.getElementById(`type-result-${active}`)?.click();
          }
        }}/>
        <div className="quick-search-status" role="status" aria-live="polite">
          {failed ? 'Could not load types.' : !definitions ? 'Loading types…' : !matches.length ? 'No matching types. Try a shorter name or check the selected specification.' : `${matches.length} types${matches.length > visible.length ? `, showing the first ${visible.length}. Keep typing to narrow results.` : ''}`}
        </div>
        {failed && <button className="action-button search-retry" onClick={() => setAttempt(value => value + 1)}>Retry loading types</button>}
        <div className="quick-search-results" id="type-search-results" role="listbox" aria-label="Matching types" aria-busy={!definitions && !failed}>
          {visible.map((definition, position) => <a key={definition.resource} id={`type-result-${position}`} role="option" aria-selected={position === active} tabIndex={-1} href={definition.href} onMouseMove={() => setActive(position)} onClick={() => onEvent?.('search_select')}>
            <strong>{definition.name}</strong><span>{definition.resource}</span>
          </a>)}
        </div>
        <p id="type-search-help" className="quick-search-help"><span><kbd>↑</kbd> <kbd>↓</kbd> choose <kbd>Enter</kbd> open <kbd>Esc</kbd> close</span><span>Search stays in your browser.</span></p>
      </div>
    </dialog>
  </>;
}
