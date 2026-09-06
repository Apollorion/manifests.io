# 2. Render contextual React pages inside the Go backend

Date: 2026-09-06

## Status

Accepted. Supersedes [ADR-0001](0001-read-source-schemas-with-one-go-model-and-render-documentati.md).

## Context and Problem Statement

The original site rendered contextual headings and links on each request.
Canonical prerendering cannot represent arbitrary path and circular-reference history, and browser-only updates lost working links when JavaScript was unavailable.
The user requires full user-facing parity, one Go backend and one React renderer.

## Considered Options

1. Run the existing synchronous React renderer in an embedded Goja runtime, retaining canonical prerender caching.
2. Add a Node rendering process or service.
3. Maintain a separate Go HTML renderer or rewrite prerendered HTML bindings.
4. Keep canonical HTML inert until browser JavaScript applies context.

## Decision Outcome

Chosen: **option 1**.

Bundle the existing React App and synchronous renderToString renderer for Goja. Go loads a fixed trusted bundle and calls it with serialized Page data for contextual requests and errors. Canonical pages may use the existing React prerender cache. Keep two isolated runtimes, exclusive ownership per render, request cancellation, a rendering deadline and call-stack limit. No filesystem or network APIs are exposed to the JavaScript runtime. The bundle includes a URLSearchParams polyfill and selects React's installed synchronous browser renderer to avoid initializing unused streaming APIs.

## Consequences

### Good

- Contextual links, titles, disabled circular references and error recovery render before browser JavaScript runs.
- The same React components remain authoritative in build output, Go responses and browser hydration.
- Production stays a single stateless Go process with no Node runtime or cgo.

### Bad

- Goja executes JavaScript more slowly than a native JavaScript engine, so concurrency and cancellation must remain bounded.
- The renderer bundle and compatibility tests must be rebuilt when React or Goja changes; selecting React's synchronous browser implementation depends on its installed package layout.
- Canonical prerendered pages still increase image size and build work.

### Rejected because

- A Node process adds a second runtime service lifecycle to the user's one-backend design.
- A separate Go renderer or HTML patching duplicates React presentation behavior and creates another source of drift.
- Inert canonical HTML breaks the old site's contextual navigation when JavaScript is unavailable.
