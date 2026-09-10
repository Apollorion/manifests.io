# 7. Share concurrent renders of identical page data

Date: 2026-09-10

## Status

Accepted.

## Context and Problem Statement

Concurrent cold origin requests can ask the Goja renderer for the same canonical page before Cloudflare stores the first response.
The existing one-CPU benchmark sent forty simultaneous large-page requests and twenty-five exceeded the two-second rendering deadline.
Repeated rendering consumes origin CPU and transient errors prevent successful edge fills.

## Considered Options

1. Share identical in-flight renders, with cancellation when their last waiting caller leaves.
2. Add a persistent process-local rendered-page cache.
3. Increase renderer concurrency or its deadline.

## Decision Outcome

Chosen: **option 1**.

Hash complete render JSON and share only active identical work inside each process. Each caller retains its own two-second deadline and render span. Shared work has a separate two-second limit, survives an individual caller's cancellation, and is canceled when no callers remain. Remove completed and failed work immediately. Limit tracked distinct flights to 128; excess distinct work uses the existing bounded renderer pool. A small waiter-aware implementation is necessary because no existing helper provides last-caller cancellation.

## Consequences

### Good

- A burst for one cold page performs one render and can complete inside the existing production deadline.
- Distinct page data never shares output, and failed results cannot become a persistent cache entry.
- Canceled callers stop waiting without breaking other visitors or leaving abandoned rendering active.

### Bad

- The process now manages synchronized waiter and cancellation state, requiring deterministic concurrency and race tests.
- Sharing applies only while work is active and only within one process; separate instances and later requests may still render on legitimate edge misses.
- Excess distinct inputs beyond the tracking bound retain the original pool behavior rather than gaining coalescing.

### Rejected because

- A persistent local cache introduces a second eviction and memory-budget policy for responses already owned by the edge cache.
- More workers compete for one production CPU; longer deadlines retain duplicate work and increase billed duration.
