import { gzipSync } from 'node:zlib';

export function renderStaticPage(template, record, render) {
  const html = template.replace('<!--app-html-->', () => render(JSON.parse(record.data)))
    .replace('<!--page-head-->', () => record.head)
    .replace('<!--page-data-->', () => `<script id="__PAGE_DATA__" type="application/json">${record.data}</script>`);
  if (html.includes('<!--page-') || html.includes('<!--app-html-->')) throw new Error('Static page contains unfinished placeholders');
  return { html: gzipSync(html), json: gzipSync(record.data + '\n') };
}
