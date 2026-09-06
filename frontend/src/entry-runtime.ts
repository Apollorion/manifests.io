import 'url-search-params-polyfill';
import { render } from './entry-server';
import type { Page } from './types';

export function renderPage(json: string): string {
  return render(JSON.parse(json) as Page);
}
