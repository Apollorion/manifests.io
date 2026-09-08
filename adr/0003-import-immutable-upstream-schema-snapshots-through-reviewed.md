# 3. Import immutable upstream schema snapshots through reviewed pull requests

Date: 2026-09-08

## Status

Accepted.

## Context and Problem Statement

The documentation reader discovers checked-in Kubernetes OpenAPI and CRD snapshots automatically, but adding versions still requires manual source discovery.
Automating those additions must preserve existing documentation URLs and keep malformed upstream documents out of production.

## Considered Options

1. Add immutable release snapshots through a scheduled pull request workflow
2. Replace the existing version directories with the latest upstream schemas
3. Fetch upstream schemas from the running documentation service

## Decision Outcome

Chosen: **option 1**.

Use a checked-in source registry covering every supported product and release track. A Go updater discovers stable releases, downloads their source schemas into staging, and validates the combined corpus with the same reader used by the application. Existing snapshots are retained. The scheduled workflow proposes additions in a pull request, with source provenance for review. Normal verification and the existing Spacelift promotion remain the release boundary.
Use the repository-scoped GITHUB_TOKEN instead of adding a long-lived personal credential. GitHub can require approval for the resulting pull request checks; that fits the explicit review step.

## Consequences

### Good

- Existing links continue documenting the same schema snapshots.
- Source validation reuses the production reader instead of introducing a converter.
- Serving documentation needs no upstream network access or new runtime credentials.
- Updates remain inspectable and roll back with an application image.

### Bad

- The source registry needs maintenance when upstream projects move files or change release layouts.
- Retained versions increase repository, image and prerender size.
- New releases wait for a reviewed pull request and production promotion.
- Upstream release or download failures can delay a scheduled refresh.

### Rejected because

- Replacing an existing version silently changes documentation at previously shared URLs and prevents reproducing its original content.
- Runtime fetching couples availability to upstream services, makes a deployed image's content nondeterministic, and moves validation failures into user requests.
