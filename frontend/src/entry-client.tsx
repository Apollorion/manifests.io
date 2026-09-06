import { createRoot, hydrateRoot } from 'react-dom/client';
import { App, AppBoundary } from './App';
import { pageQuery } from './navigation';
import type { Page } from './types';
import { captureError, initializeObservability } from './telemetry';
import './styles.css';

async function start() {
  const container = document.getElementById('root');
  if (!container) throw new Error('Missing application root');
  try {
    const data = document.getElementById('__PAGE_DATA__')?.textContent;
    let page: Page;
    if (data?.trim()) {
      page = JSON.parse(data) as Page;
    } else {
      const response = await fetch(`/api/page?${pageQuery(window.location)}`);
      if (!response.ok) throw new Error('The documentation could not be loaded. Please try again.');
      page = await response.json() as Page;
    }
    const app = <AppBoundary onError={captureError}><App initialPage={page}/></AppBoundary>;
    if (data?.trim() && container.hasChildNodes() && !container.hasAttribute('data-dynamic')) hydrateRoot(container, app);
    else createRoot(container).render(app);
    try { initializeObservability(); } catch (error) { captureError(error); }
  } catch (error) {
    captureError(error);
    createRoot(container).render(<main className="boot-error" role="alert"><h1>Documentation unavailable.</h1><p>We couldn’t load this page. Try reloading or open the resource library.</p><a className="action-button" href="/">Open the library</a></main>);
  }
}

void start();
