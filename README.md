# Manifests.io

Browse Kubernetes and custom resource schemas with a Go backend and one React renderer.

Choose a product and version, filter resources, then follow fields into their types. Descriptions, required fields, arrays, maps, unions, validation constraints, and alternate API versions come from the original schemas. Existing resource URLs and the legacy `linked`, `oneOf`, and `key` parameters remain supported.

The resource in the URL selects the schema, while `path` records the field traversal shown in the heading. For example, `/kubernetes/1.34/io.k8s.api.core.v1.PodSpec?path=Deployment.spec.template.spec` displays PodSpec in its Deployment context. Each field click extends that path while linking directly to its target schema. Unnamed inline schemas use a separate JSON Pointer in `pointer`; version switching preserves both values. Schema navigation does not redirect to canonical URLs.

Recursive schemas can be visited three times within a traversal. Links that would visit the same schema a fourth time show a red × and “Circular reference”; unrelated fields remain available. Go identifies recursive components in the resolved schema graph, covering fields, arrays, maps, and schema variants. Only recursive schemas add a `trail` query containing visit counts, so reloads, shared links, browser history, and version switching retain the limit without depending on browser storage. Canonical URLs omit this context. Opening a resource directly starts a new traversal.

## Run locally

Requirements: Go 1.27+, Node.js 24.15+, and npm.

```sh
make build
PORT=18080 ./build/manifests
```

Open [localhost:18080](http://localhost:18080). `make build` installs locked frontend dependencies, builds the browser and server rendering bundles, compiles Go, and prerenders documentation. Node runs during the build only. Production uses one Go process.

For live frontend development after the first build:

```sh
make dev
```

Vite listens on port 5173 and proxies `/api` to the Go service on port 8080. Use the built Go server to verify prerendering, status codes, and security headers. Vite's development fallback serves the application shell for every URL.

## Source schemas

There is no converter or generated CRD JSON to maintain.

| Source | Location |
| --- | --- |
| Kubernetes OpenAPI JSON | `oaspec/kubernetes/<version>.json` |
| Original CRD YAML or JSON | `ETL/crds/<product>-<version>/` |

`ETL/crds` retains its existing path to preserve source history; the ETL executable is gone. Drop a new version into the appropriate directory and rebuild. Product/version discovery is automatic. Six existing product display-name aliases preserve URLs containing spaces; Gateway API keeps its existing standard/experimental labels. The default landing page remains Kubernetes 1.34 for compatibility.

The reader accepts OpenAPI v2 definitions and OpenAPI v3 component schemas, including schema-only documents. CRDs may be individual documents, multi-document YAML, or Kubernetes Lists. It extracts each CRD version's `openAPIV3Schema`, with support for older `spec.validation` schemas. Unresolved or external references fail loading instead of reading arbitrary files or making network requests.

[kin-openapi](https://github.com/getkin/kin-openapi) supplies the OpenAPI types, v2-to-v3 conversion, and reference resolution. Both sources use its `openapi3.Schema` model. Our code handles catalog discovery, navigation, legacy URL aliases, and the finite page data consumed by React. Kubernetes extensions are retained. See [ADR 0001](adr/0001-read-source-schemas-with-one-go-model-and-render-documentati.md) for the trade-offs.

## HTTP interface

| Route | Response |
| --- | --- |
| `/` | Redirect to `/kubernetes/1.34` |
| `/<item>/<version>` | Resource index |
| `/<item>/<version>/<resource>` | Schema documentation |
| `/api/catalog` | Products and versions as JSON |
| `/api/page?item=...&version=...&resource=...` | The same page data used by React |
| `/api/definitions?item=kubernetes&version=1.34` | All named types and nested CRD aliases for quick search |
| `/healthz`, `/readyz` | Ready after the corpus loads successfully |

Nested inline schemas use `pointer`, a JSON Pointer through schema keywords such as `/properties/spec/properties/containers/items`; `path` carries the displayed field traversal. References are resolved by the library. Canonical schema locations make recursive references navigable without infinitely expanding the tree. Field filters remain local to the browser and are never sent to the API.

Prerendering calls the same React component used by the browser. Pages are stored compressed to bound image size. Canonical requests can use this cache; contextual URLs and errors render the same React App inside Go using [Goja](https://github.com/dop251/goja). The response already contains current headings, traversal links, circular-reference limits, and recovery controls before browser JavaScript loads. React hydrates that markup for filtering, version selection, and theme controls. Unknown resources return HTTP 404; malformed queries return HTTP 400.

The build bundles the synchronous React renderer with a URLSearchParams polyfill into `frontend/dist-render/renderer.js`. Production runs two isolated Goja workers with exclusive access, a two-second rendering deadline, cancellation, and a call-stack limit. JavaScript receives page JSON and no filesystem or network bindings. See [ADR 0002](adr/0002-render-contextual-react-pages-inside-the-go-backend.md) for the runtime-rendering trade-offs and React upgrade checks.

### Quick type search

Use **Search all types** in the header, or press **Ctrl/Cmd+K** (also **Ctrl/Cmd+P**), to jump directly to any named type in the selected specification and version. This includes nested types such as `ContainerStatus`, not just top-level resources. Search tolerates typos and shows full type identifiers to distinguish API versions. Use arrow keys and Enter to open a result, or Escape to close the dialog and return focus.

The browser loads the selected specification's definition index when search is first opened, then uses Fuse.js locally. Search text never enters URLs, API requests, logs, or telemetry. The existing `/` shortcut still focuses the current table's field filter. Search requires JavaScript; documentation and field navigation remain server-rendered.

## Verification

```sh
make build
make test
go vet ./...
npm --prefix frontend run typecheck
tofu -chdir=infra init -backend=false
tofu -chdir=infra validate
```

Go tests cover the entire original corpus, including the 7,174 legacy definition names captured before removing generated files. Additional tests cover references and cycles, CRD envelopes, unions, required fields, invalid input, HTTP errors, script-safe serialization, and OTLP correlation/privacy. Frontend tests exercise filters, selectors, navigation, error states, and the real Faro transport.

## Container and Cloud Run

```sh
docker build --platform linux/amd64 --build-arg VERSION="$(git rev-parse --short HEAD)" -t manifests:local .
docker run --rm -p 18080:8080 manifests:local
```

With the container running, `node scripts/smoke.mjs http://localhost:18080` verifies its API, original field descriptions, nested HTML, legacy links, required fields, and error responses.

The image includes the immutable corpus, prerendered HTML, and browser assets. It runs as a non-root user, listens on `0.0.0.0:$PORT`, and needs no database, persistent disk, cluster access, or Node runtime. Schema changes require a new build. SIGTERM drains requests and flushes telemetry within Cloud Run's shutdown window.

The [OpenTofu service module](infra/README.md) defines one Cloud Run service with an immutable image digest, a dedicated service account, telemetry secret access, and scale-to-zero behavior. It uses an existing GCP project and Secret Manager secret.

### Production releases through Spacelift

The [Verify workflow](.github/workflows/ci.yml) runs the application checks and smoke-tests a Linux AMD64 container. After a successful push to `main`, or a manual workflow run against `main`, its publish job sends that exact tested image to `us-east1-docker.pkg.dev/nwf-shared/apps/manifests-io`. The full Git commit SHA is the immutable revision tag. The `production` tag identifies the latest verified candidate from the current `main` commit. Older runs cannot replace a newer candidate, and an existing SHA tag cannot be overwritten with a different image.

Publishing uses [GitHub OIDC through Google Workload Identity Federation](https://github.com/google-github-actions/auth) with `manifests-builder@nwf-shared.iam.gserviceaccount.com`; no service-account key is stored in GitHub. Only the publish job requests an identity token. Pull requests and other branches run verification without publishing.

The production Spacelift stack uses [`TheOutdoorProgrammer/configurations`, `manifests/production`](https://github.com/TheOutdoorProgrammer/configurations/tree/main/manifests/production). Its OpenTofu configuration resolves the `production` tag to an immutable digest and plans the Cloud Run update. GitHub Actions publishes images; Spacelift owns infrastructure and deployment.

1. Merge the application change into `main` and wait for both Verify jobs to succeed. A manual run on `main` follows the same checks.
2. Start a production run in Spacelift and review the planned container digest and infrastructure changes.
3. Approve the plan to deploy the candidate. Publishing an image alone does not change the live service.

To rebuild an already published commit with updated dependencies or base images, create a new commit so the revision tag remains immutable.

Runtime configuration:

| Variable | Default |
| --- | --- |
| `PORT` | `8080` |
| `DATA_DIR` | `.` |
| `WEB_DIR` | `frontend/dist` |
| `RENDER_DIR` | `frontend/prerender` |
| `PUBLIC_DIR` | `public` |
| `SITE_URL` | `https://www.manifests.io` |

Directory options also have corresponding command flags; run `./build/manifests -help`. The `-export` flag emits page data as NDJSON for the React prerender build.

## Observability

Structured stdout logs include trace/span IDs. Configure `OTEL_EXPORTER_OTLP_ENDPOINT` and runtime-only `OTEL_EXPORTER_OTLP_HEADERS` for OTLP HTTP/protobuf traces and logs. Without an endpoint, export is disabled. `OTEL_SDK_DISABLED=true` disables export explicitly. W3C TraceContext and Baggage propagation are configured in both cases.

Production browser telemetry uses the existing public Grafana Faro collector. The existing PostHog integration retains manual pageview events on the production domains, with automatic capture, recording, persistence, and person profiles disabled. Local previews do not send production telemetry. Search values, query strings, request bodies, cookies, authorization headers, raw URLs, and freeform exceptions are excluded from exported telemetry. General OTLP credentials never enter the browser build. See [deployment telemetry configuration](infra/README.md#telemetry-configuration) for details.

## License

[MIT](LICENSE). Original authorship remains in the site footer.
