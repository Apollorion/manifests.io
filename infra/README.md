# Cloud Run deployment

This OpenTofu root runs the Go API, React frontend, and immutable schema corpus in one public Cloud Run service. It needs an existing billed project, a published linux/amd64 container image, and an existing Secret Manager secret. It creates no database, buckets, background workers, or secret payloads.

The runtime has one CPU, 1 GiB memory for the parsed schema cache, at most five instances by default, and scales to zero. It uses request-based billing with CPU throttled between requests (`cpu_idle = true`). Startup probes allow up to two minutes for schema loading; measure peak memory with the production corpus before lowering the allocation.

## Prepare a deployment

Build and publish the image using the repository Dockerfile after reviewing the changes. Resolve the published digest with:

```sh
gcloud artifacts docker images describe REGION-docker.pkg.dev/PROJECT/REPOSITORY/manifests-io:VERSION --format='value(image_summary.digest)'
```

Set `image` to the complete image reference ending in `@sha256:...`. Mutable tags are rejected. The image must be readable by the project's Cloud Run service agent.

The existing Secret Manager secret must contain the complete standard OTLP header assignment, `Authorization=Basic <credential>`, rather than just the Basic value. Refer to the secret by ID and a pinned numeric version. Keep the value out of `.tfvars`, shell arguments, build arguments, Git, and the browser bundle. Cloud Run injects it only at runtime; OpenTofu never reads its payload into state.

Supply these non-secret variables through your existing OpenTofu runner or a local untracked variables file:

```hcl
project_id                  = "YOUR_PROJECT"
image                       = "REGION-docker.pkg.dev/PROJECT/REPOSITORY/manifests-io@sha256:YOUR_DIGEST"
otlp_headers_secret         = "YOUR_EXISTING_SECRET_ID"
otlp_headers_secret_version = "1"
```

Store state in an existing private, versioned GCS bucket. Initialize with `tofu init -backend-config="bucket=YOUR_STATE_BUCKET" -backend-config="prefix=apps/manifests-preview"`. Use a distinct prefix per deployment. Credentials come from the runner's Google authentication, never a backend configuration file. Validate and review the OpenTofu plan before applying it. Deletion protection is enabled. This root leaves the current public domain unchanged; move DNS or a load balancer only after smoke checks and a deliberate cutover decision.

For a preview, set `service_name = "manifests-preview"` and `site_url` to its Cloud Run HTTPS origin so canonical links and the sitemap point at the preview. Google exposes a deterministic origin as `https://SERVICE-PROJECT_NUMBER.REGION.run.app`; use the project's numeric identifier, not its project ID.

## Verify the revision

```sh
gcloud run services describe manifests-io --project YOUR_PROJECT --region us-east1 --format='value(status.url)'
curl --fail --silent --show-error https://YOUR_CLOUD_RUN_URL/readyz
curl --fail --silent --show-error https://YOUR_CLOUD_RUN_URL/api/catalog
curl --fail --silent --show-error 'https://YOUR_CLOUD_RUN_URL/api/page?item=kubernetes&version=1.34'
gcloud logging read 'resource.type="cloud_run_revision" AND resource.labels.service_name="manifests-io" AND severity>=WARNING' --project YOUR_PROJECT --limit 20
```

Use an item and version from `/api/catalog` for the page check. Confirm the deployed release in Grafana, an API server trace and its correlated log, and browser Faro telemetry. Check for `telemetry export failed` warnings and inspect exported payloads for queries, fragments, authorization headers, or freeform errors. Never use real personal information as test data.

## Telemetry configuration

Go uses OTLP HTTP/protobuf for traces and logs, plus structured stdout logs. Standard `OTEL_EXPORTER_OTLP_ENDPOINT`, signal-specific endpoint variables, and `OTEL_EXPORTER_OTLP_HEADERS` configure export. With no endpoint, export is disabled. `OTEL_SDK_DISABLED=true` disables both exporters; `OTEL_TRACES_EXPORTER=none` and `OTEL_LOGS_EXPORTER=none` disable individual signals. W3C TraceContext and Baggage propagation remain configured. Logs accept static messages and bounded attributes; never interpolate request data into log messages.

The HTTP middleware ends the server span and emits the completion log before flushing both batch exporters concurrently. It retains the last response byte, or the headers for a bodyless response, until those flushes finish. This keeps the request active while Cloud Run supplies CPU without buffering whole schema documents. Export adds collector latency to origin requests, bounded by a shared one-second deadline; collector failures release the response and emit a sanitized local warning. Request cancellation does not cancel the bounded export attempt. Health and readiness probes bypass telemetry so collector outages cannot fail probes. Shutdown still drains requests and flushes remaining telemetry.

The production container reuses the existing constrained public Faro collector; its CSP permits that collector. Docker's `VERSION` build argument also identifies the frontend build. Development builds and production previews on localhost export nothing. For Vite development with a local test collector, `VITE_FARO_URL` may point at that collector; `VITE_TELEMETRY_DISABLED=true` disables browser telemetry in a direct Vite build. These Vite development overrides are not Docker build arguments. No OTLP credential is ever a frontend build variable. Faro payloads are reconstructed through a shared privacy filter before SDK transport, with transient session IDs and no persistent session storage.

References: [Cloud Run service resource](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/cloud_run_v2_service), [OpenTelemetry Go exporters](https://opentelemetry.io/docs/languages/go/exporters/), [Grafana Faro](https://grafana.com/docs/grafana-cloud/monitor-applications/frontend-observability/instrument/faro/).
