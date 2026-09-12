# 9. Serve complete static releases from Cloud Storage through Cloudflare

Date: 2026-09-12

## Status

Accepted.

## Context and Problem Statement

Cloud Run still receives uncached crawler traffic after shared-cache normalization and tiered caching.
The site corpus is static between releases, and Joey requires zero idle origin compute costs.
The current 72,362 canonical pages are expected to exceed Cloudflare Static Assets' 100,000-file ceiling as CRDs grow.

## Considered Options

1. Keep Cloud Run with tiered caching
2. Use Cloudflare Workers Static Assets
3. Use GCS object storage behind Cloudflare

## Decision Outcome

Chosen: **option 3**.

Build complete HTML, embedded page data, JSON API responses, search indexes, errors, crawler documents, and assets before deployment. Export a finite routing graph per specification so Cloudflare can resolve aliases, recursive pointers, and legacy union selectors without rendering or accessing the original corpus. Publish verified files as an immutable OCI artifact through the existing GitHub WIF identity. OpenTofu and Spacelift provision the bucket, verify and upload content-addressed objects, then atomically update a release pointer. Cloudflare reads immutable objects over Google's HTTPS endpoint, avoiding a GCP load balancer. Remove the production Cloud Run service only after public static verification. Keep the Go server for development and independent previews.

## Consequences

### Good

- No production origin process, recurring probes, or idle compute billing.
- Content-addressed objects share cache fills across equivalent URLs and avoid rewriting unchanged files.
- The bucket does not impose the static-hosting file ceiling, and publication stays separate from deployment.
- Complete HTML remains readable without JavaScript; browser traversal restoration and Faro remain intact.

### Bad

- Builds and storage replace runtime rendering; releases require complete export and upload verification.
- The Worker routing graph must stay compatible with the Go schema resolver; shard memory is explicitly bounded.
- Old immutable objects consume storage until a reference-aware retention process removes unreferenced objects.
- Release-pointer caching permits up to 60 seconds of propagation delay, and object storage still charges for storage, reads, writes, and transfer.

### Rejected because

- Cloud Run retains runtime origin work for cache misses and does not meet the desired static-origin cost model.
- Cloudflare Workers Static Assets' 100,000-file paid limit leaves inadequate capacity for expected CRD growth.
