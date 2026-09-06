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
  deploys_to: gcp-cloud-run
  project: nwf-shared
  region: us-east1
  service: manifests-production
  edge: cloudflare-workers
  deployment: spacelift
---

Browse Kubernetes and custom resource schemas through one React renderer and one Go backend. Kubernetes OpenAPI JSON and original CRD YAML share kin-openapi's schema model; product/version discovery is automatic and there is no Python conversion step. The image includes its source corpus and prerendered pages. Contextual pages and errors render the same React App inside Goja in the Go process, so navigation and circular-reference limits work before browser JavaScript loads.

Production runs on Cloud Run service manifests-production in GCP project nwf-shared, us-east1. Cloudflare owns public DNS/TLS and routes www.manifests.io through a fixed-origin Worker proxy. GitHub Actions verifies and publishes images using OIDC. The manifests-production Spacelift stack resolves the production candidate tag to an immutable digest and performs an approved deployment; image publication alone does not deploy.

Named schema URLs identify the actual target. `path` retains the readable traversal; `pointer` selects unnamed inline schemas. Recursive nodes allow three visits, then disable fourth-visit links. Backend OpenTelemetry and browser Grafana Faro are active.

Operational runbooks and deployment evidence live in Dusk notes attached to this repository and service:stout/nwf-shared.
