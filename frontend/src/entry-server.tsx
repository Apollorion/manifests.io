import { renderToString } from 'react-dom/server';
import { App, AppBoundary } from './App';
import type { Page } from './types';

export function render(page: Page): string {
  return renderToString(<AppBoundary page={page}><App initialPage={page}/></AppBoundary>);
}
