import { createRoot, hydrateRoot } from 'react-dom/client';
import { App, AppBoundary, RecoveryPage } from './App';
import { pageQuery } from './navigation';
import type { Page } from './types';
import { captureError, captureSearchEvent, initializeObservability } from './telemetry';
import './styles.css';

async function start() {
  const container = document.getElementById('root');
  if (!container) throw new Error('Missing application root');
  let page: Page | undefined;
  try {
    const data = document.getElementById('__PAGE_DATA__')?.textContent;
    if (data?.trim()) {
      page = JSON.parse(data) as Page;
    } else {
      const response = await fetch(`/api/page?${pageQuery(window.location)}`);
      if (!response.ok) throw new Error('The documentation could not be loaded. Please try again.');
      page = await response.json() as Page;
    }
    if (!page) throw new Error('The documentation response was empty.');
    const app = <AppBoundary page={page} onError={captureError}><App initialPage={page} onSearchEvent={captureSearchEvent}/></AppBoundary>;
    if (data?.trim() && container.hasChildNodes()) hydrateRoot(container, app);
    else createRoot(container).render(app);
    try { initializeObservability(); } catch (error) { captureError(error); }
  } catch (error) {
    captureError(error);
    const recovery: Partial<Page> = page ?? {};
    if (!page) {
      try {
        const query = new URLSearchParams(pageQuery(window.location));
        recovery.item = query.get('item') || undefined;
        recovery.version = query.get('version') || undefined;
        recovery.resource = query.get('resource') || undefined;
      } catch { /* A malformed route still gets the default library and issue link. */ }
      try {
        const response = await fetch('/api/catalog');
        if (response.ok) recovery.catalog = await response.json();
      } catch { /* Recovery remains available if the catalog is unreachable. */ }
    }
    createRoot(container).render(<RecoveryPage page={recovery}/>);
  }
}

void start();
