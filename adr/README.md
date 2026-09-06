# Architecture decisions

| # | Decision | In one line |
| --- | --- | --- |
| [0001](0001-read-source-schemas-with-one-go-model-and-render-documentati.md) | Read source schemas with one Go model and render documentation with React | One kin-openapi schema model and one React renderer served by a stateless Go process. |
| [0002](0002-render-contextual-react-pages-inside-the-go-backend.md) | Render contextual React pages inside the Go backend | Use Goja for current navigation HTML while retaining canonical prerender caching. |
