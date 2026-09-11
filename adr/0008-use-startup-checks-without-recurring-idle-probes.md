# 8. Use startup checks without recurring idle probes

Date: 2026-09-11

## Status

Accepted.

## Context and Problem Statement

Manifests.io must not spend compute while idle. Cloud Run uses request-based billing and zero minimum instances, but its HTTP liveness probe runs every 30 seconds. Google allocates and bills CPU and memory when probes run. The health handler returns a static success response without testing schema rendering.

## Considered Options

1. Retain startup readiness and remove recurring liveness probes
2. Keep the 30-second liveness probe
3. Increase the liveness interval

## Decision Outcome

Chosen: **option 1**.

Keep the HTTP startup probe on /readyz so schema loading and server initialization complete before traffic is accepted. Remove the recurring liveness probe. Keep cpu_idle=true and minimum instances zero. Process exits remain visible to Cloud Run; the HTTP server and renderer retain their existing deadlines. Idle container residence alone is free under this billing configuration. Serving cache misses, startup and shutdown remain billable.

## Consequences

### Good

- Eliminates periodic probe compute while there are no user requests.
- Preserves startup readiness without changing application behavior, cache policy or telemetry.

### Bad

- A process-wide deadlock will no longer trigger a restart through the periodic liveness check; request failures and platform lifecycle controls must reveal or recover it.
- Removing the probe does not eliminate legitimate request charges or guarantee that steady traffic lets the service reach zero instances.

### Rejected because

- Keeping the 30-second probe spends compute while idle and its static response does not validate the renderer.
- A longer interval reduces recurring charges but does not meet the zero-idle-compute requirement.
