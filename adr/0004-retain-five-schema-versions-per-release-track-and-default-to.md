# 4. Retain five schema versions per release track and default to newest Kubernetes

Date: 2026-09-08

## Status

Accepted. Supersedes [ADR-0003](0003-import-immutable-upstream-schema-snapshots-through-reviewed.md).

## Context and Problem Statement

Unlimited snapshots make the serving corpus and prerender output grow with every upstream release.
The owner requested at most five versions of each product and automatic promotion of newly imported Kubernetes versions to the default.

## Considered Options

1. Retain the five newest semantic versions per product and release track
2. Keep all snapshots and only hide older entries in the version selector

## Decision Outcome

Chosen: **option 1**.

Apply retention to the checked-in serving corpus as part of the existing updater, even when no new upstream release is available. Treat Gateway API standard and experimental as separate tracks. Preserve retained snapshot bytes, stage and validate the remaining catalog before removal, and show additions and checksum-backed retirements in the same reviewed PR. Retired URLs return the normal unavailable-documentation response; Git history remains the archive. Derive the site's default Kubernetes version from the latest stable semantic version in the loaded catalog and share that backend choice with React recovery links.

## Consequences

### Good

- Serving and prerendering retain at most five versions per product and track.
- New Kubernetes imports become the default without another code edit.
- Retained URLs keep their original content and old snapshots remain recoverable from Git.

### Bad

- URLs for retired snapshots stop serving their former documentation.
- Git history still contains retired source files, so this does not shrink repository history.
- Schema PRs now include removals as well as additions, requiring complete-file validation and rollback handling.

### Rejected because

- Hiding old entries leaves the serving corpus and prerender output unbounded and does not implement the requested retention limit.
