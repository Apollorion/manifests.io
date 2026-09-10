# 6. Restore traversal from embedded canonical page data

Date: 2026-09-10

## Status

Accepted. Supersedes [ADR-0005](0005-share-canonical-html-and-restore-traversal-in-the-browser.md).

## Context and Problem Statement

After sharing HTML, contextual page API requests accounted for 1,256 of 2,542 origin requests in a ten-minute production sample.
Traversal changes headings, links and bounded visit counts, while the embedded canonical page already contains resolved fields and targets.
Crawling is acceptable; repeated origin work for equivalent resources is the problem.

## Considered Options

1. Restore traversal over embedded canonical Page data with Go-provided cycle identities.
2. Keep contextual page API requests and their complete query cache keys.
3. Block more crawlers.
4. Ship the schema graph and reimplement schema resolution in the browser.

## Decision Outcome

Chosen: **option 1**.

The browser restores path, linked and trail over the canonical Page embedded in HTML. Go supplies sorted cycle identities and scalar-row metadata; it remains the schema resolver. Generated Go fixtures verify browser traversal parity across recursion, arrays, maps, inline schemas and variants. Page and definitions APIs ignore traversal context. The Worker whitelists and sorts semantic selectors, shares public responses across irrelevant cookies and reload directives, and rejects malformed or oversized schema queries before origin. Stable schema 404s advertise the same seven-day shared lifetime as valid pages. Deploy application support before Worker normalization, then purge. Explicit origin diagnostics use X-Manifests-Cache-Bypass: 1.

## Consequences

### Good

- Contextual browser visits issue no page API request, and repeated crawler contexts reuse canonical cache entries.
- Schema selection retains pointer and legacy oneOf/key semantics without a second schema resolver.
- Repeated missing schema requests can reuse a cached 404.

### Bad

- The browser owns a small traversal transformation that must stay in parity with Go; fixtures and browser tests enforce this contract.
- Cloudflare Cache API entries remain local to a data center and can expire or be evicted; cold fills and explicit bypasses still reach origin.
- Seven-day cached 404s require a purge when deployment adds previously missing resources.
- The app and Worker require an ordered rollout; old browser documents may require refresh after the purge.

### Rejected because

- Contextual API keys reproduce the unbounded traversal variants even after HTML is shared.
- Broader bot blocking does not meet the requirement to allow crawling while avoiding repeated origin work.
- Shipping and resolving the entire schema graph duplicates the existing resolver and increases browser payload unnecessarily.
