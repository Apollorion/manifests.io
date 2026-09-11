# Architecture decisions

| # | Decision | In one line |
| --- | --- | --- |
| [0001](0001-read-source-schemas-with-one-go-model-and-render-documentati.md) | Read source schemas with one Go model and render documentation with React | One kin-openapi schema model and one React renderer served by a stateless Go process. |
| [0002](0002-render-contextual-react-pages-inside-the-go-backend.md) | Render contextual React pages inside the Go backend | Use Goja for current navigation HTML while retaining canonical prerender caching. |
| [0003](0003-import-immutable-upstream-schema-snapshots-through-reviewed.md) | Import immutable upstream schema snapshots through reviewed pull requests | Discover stable releases from a maintained source registry, validate with the application reader, and propose immutable additions. |
| [0004](0004-retain-five-schema-versions-per-release-track-and-default-to.md) | Retain five schema versions per release track and default to newest Kubernetes | Retain the five newest snapshots per product and track; derive the Kubernetes default from the catalog. |
| [0005](0005-share-canonical-html-and-restore-traversal-in-the-browser.md) | Share canonical HTML and restore traversal in the browser | Cache documentation by schema identity; load visitor traversal through the existing page API. |
| [0006](0006-restore-traversal-from-embedded-canonical-page-data.md) | Restore traversal from embedded canonical page data | Share HTML and API responses by schema identity and restore traversal locally without an API request. |
| [0007](0007-share-concurrent-renders-of-identical-page-data.md) | Share concurrent renders of identical page data | Coalesce identical in-flight renders within each process while preserving caller deadlines and cancellation. |
| [0008](0008-use-startup-checks-without-recurring-idle-probes.md) | Use startup checks without recurring idle probes | Retain startup readiness and request-based billing without periodic liveness charges. |
