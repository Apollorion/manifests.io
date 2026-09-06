# 1. Read source schemas with one Go model and render documentation with React

Date: 2026-09-06

## Status

Accepted.

## Context and Problem Statement

The existing Next.js app imports generated CRD JSON and depends on a Python converter that flattens schemas and invents reference names.
The requested rewrite must preserve browsing behavior and public links, run on Cloud Run, and reuse maintained OpenAPI packages.

## Considered Options

1. Use kin-openapi for a shared schema model, thin CRD extraction, one React renderer, and build-time prerendering served by Go.
2. Port the Python flattening pipeline into Go and retain generated CRD JSON.
3. Keep Next.js and move its container to Cloud Run.

## Decision Outcome

Chosen: **option 1**.

Use kin-openapi's OpenAPI types and reference resolver, converting existing OpenAPI v2 inputs into the shared v3 model in memory. Extract CRD version schemas without flattening their source trees. Keep legacy generated names only as navigation aliases. React renders both prerendered documentation and interactive browser views; Go serves the immutable build and page API in one Cloud Run service.

## Consequences

### Good

- Original CRDs remain the source of truth; adding inputs does not require running a converter.
- One schema model and one React renderer cover all supported products.
- Prerendered resource pages preserve crawlable content without a Node production runtime.

### Bad

- Legacy URL aliases require compatibility tests against the previous corpus.
- Prerendered pages increase image size and build work.
- Library upgrades require corpus regression tests because Kubernetes schema extensions are not ordinary endpoint documentation.

### Rejected because

- Porting the converter preserves a redundant intermediate format and the lossy flattening behavior the user wants removed.
- Keeping Next.js would satisfy hosting alone but does not satisfy the requested Go backend rewrite.
