---
dusk: v1alpha1
namespace: stout
kind: repository
name: manifests.io
title: Manifests.io
attributes:
  language: go
  frontend: react-typescript
  url: https://www.manifests.io
  public: true
  deploys_to: gcs-static
  project: nwf-shared
  region: us-east1
  spacelift_stack: manifests-production
  bucket: nwf-shared-manifests-production-static
  worker: manifests-static
  edge: cloudflare-workers
  deployment: spacelift
---

Browse Kubernetes and custom resource schemas through a complete static Go/React build. Kubernetes OpenAPI JSON and original CRD YAML share kin-openapi's schema model; product/version discovery is automatic and there is no schema conversion step. Go resolves the corpus at build time, and React emits complete HTML, embedded canonical page JSON, search indexes, crawler documents and error pages. The Go server remains available for development and independent previews.

The production configuration serves [the public site](https://www.manifests.io) through the manifests-static Cloudflare Worker and the nwf-shared-manifests-production-static GCS bucket in nwf-shared/us-east1. The Worker resolves semantic selectors through bounded per-document graphs and fetches immutable objects. Cloudflare owns public DNS/TLS and caching; no origin process is required. GitHub Actions verifies and publishes a scratch static artifact using OIDC. A committed source SHA and OCI digest select an OpenTofu deployment through the manifests-production Spacelift stack. Upload verification precedes an atomic release-pointer update; publication alone does not deploy.

Named schema URLs identify the actual target. `pointer`, `oneOf` and `key` select schema content. The browser restores `path`, `linked` and `trail` traversal context from embedded data. Recursive nodes allow three visits, then disable fourth-visit links. Cloudflare emits sanitized origin-failure logs, and browser Grafana Faro uses private source maps matched to the source SHA. Static production has no Go backend telemetry exporter.

Current rollout status, operational runbooks and deployment evidence live in Dusk notes attached to this repository, service:stout/manifests.io and service:stout/nwf-shared.
