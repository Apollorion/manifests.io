# 5. Share canonical HTML and restore traversal in the browser

Date: 2026-09-10

## Status

Accepted. Fetching contextual page data from the API is superseded by [ADR-0006](0006-restore-traversal-from-embedded-canonical-page-data.md).

Canonical HTML, semantic selectors, and the synchronous startup guard remain in force. Supersedes [ADR-0002](0002-render-contextual-react-pages-inside-the-go-backend.md).

## Context and Problem Statement

Recursive crawler traffic creates large numbers of path and trail combinations for the same schema.
A two-hour production sample had approximately 62,125 visitor requests and 779 cache hits; 99.6% of dynamic requests carried query strings.
The current server embeds traversal into both HTML and page JSON, so dropping those parameters from the cache key alone would serve incorrect headings and links.
Joey requested sharing cached documentation across frontend traversal parameters.

## Considered Options

1. Serve canonical schema HTML and request traversal page data in the browser.
2. Ignore all query parameters only in the Worker cache key.
3. Retain contextual HTML caching and increase TTL or add tiered caching.
4. Move all traversal computation into a second client implementation of the schema graph.

## Decision Outcome

Chosen: **option 1**.

HTML ignores path, linked and trail while keeping pointer and legacy oneOf/key schema selectors. The browser reads the original URL and obtains contextual Page data through the existing Go API before enabling contextual navigation. Canonical visits hydrate without another API request. The Worker removes traversal parameters from both the HTML cache key and the origin request, and leaves API queries intact. Deploy the application first, then the Worker and purge old entries. Existing contextual API behavior, traversal limits and telemetry remain authoritative.

## Consequences

### Good

- Repeated contextual HTML requests reuse one schema response, including when a contextual URL warms the cache first.
- Non-JavaScript crawlers receive bounded canonical navigation instead of accumulating traversal history.
- Existing schema resolution and cycle tracking are reused; human reloads, history and shared links retain their context.

### Bad

- Contextual browser visits require an additional API request; its failure must display recovery controls.
- Without JavaScript, schema content and links remain available but accumulated traversal headings and visit limits are not restored.
- The app and Worker require an ordered rollout and purge; rolling back only the app while keeping normalized edge caching breaks context restoration.

### Rejected because

- Dropping selectors would merge different inline schemas, while changing only the key lets the first contextual visitor populate shared HTML.
- A longer TTL or tiered cache does not merge unique traversal keys; most measured traffic entered one data center already.
- Duplicating Go graph resolution and visit validation in the browser creates two implementations to maintain and is unnecessary for sharing HTML.
