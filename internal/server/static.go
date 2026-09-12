package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/TheOutdoorProgrammer/manifests.io/internal/schema"
)

type StaticCatalog interface {
	Catalog
	StaticDocument(schema.Query) (schema.StaticDocument, error)
}

func ExportStatic(ctx context.Context, catalog StaticCatalog, site string, output io.Writer) error {
	if site == "" {
		site = "https://www.manifests.io"
	}
	origin, err := url.Parse(site)
	if err != nil || origin.Host == "" || (origin.Scheme != "https" && origin.Scheme != "http") || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || (origin.Path != "" && origin.Path != "/") {
		return fmt.Errorf("static site URL must be an HTTP(S) origin")
	}
	site = strings.TrimRight(site, "/")
	encoder := json.NewEncoder(output)
	emit := func(record any) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return encoder.Encode(record)
	}
	products := catalog.Products()
	landing := schema.DefaultQuery(products)
	if landing.Item == "" || landing.Version == "" {
		return fmt.Errorf("static export requires a default documentation page")
	}
	if err := emit(map[string]any{"kind": "root", "site": site, "default": schema.Href(landing), "catalog": products}); err != nil {
		return err
	}
	emitPage := func(page schema.Page, status int) error {
		data, err := json.Marshal(page)
		if err != nil {
			return err
		}
		return emit(map[string]any{"kind": "page", "data": string(data), "head": pageHead(page, site, status), "status": status})
	}
	emitErrors := func(query schema.Query) error {
		for _, status := range []int{http.StatusBadRequest, http.StatusNotFound} {
			message := "This documentation URL is invalid."
			if status == http.StatusNotFound {
				message = "This resource or version was not found."
			}
			page := failurePage(catalog, query, message)
			if err := emitPage(page, status); err != nil {
				return err
			}
		}
		return nil
	}
	if err := emitErrors(landing); err != nil {
		return err
	}
	for _, product := range products {
		for _, version := range product.Versions {
			query := schema.Query{Item: product.Name, Version: version}
			graph, err := catalog.StaticDocument(query)
			if err != nil {
				return err
			}
			if err := emit(map[string]any{"kind": "document", "graph": graph}); err != nil {
				return err
			}
			page, err := catalog.Page(query)
			if err != nil {
				return err
			}
			if err := emitPage(page, http.StatusOK); err != nil {
				return err
			}
			for _, id := range slices.Sorted(maps.Keys(graph.Nodes)) {
				page, err := catalog.Page(graph.Nodes[id].Query)
				if err != nil {
					return fmt.Errorf("export %s: %w", graph.Nodes[id].Canonical, err)
				}
				if page.Canonical != graph.Nodes[id].Canonical {
					return fmt.Errorf("static node canonical URL changed: %s", id)
				}
				if err := emitPage(page, http.StatusOK); err != nil {
					return err
				}
			}
			definitions, err := catalog.Definitions(query)
			if err != nil {
				return err
			}
			if err := emit(map[string]any{"kind": "definitions", "data": definitions}); err != nil {
				return err
			}
			if err := emitErrors(query); err != nil {
				return err
			}
			if err := emit(map[string]any{"kind": "document-end"}); err != nil {
				return err
			}
		}
	}
	crawlers, err := buildCrawlerFiles(site, catalog.Routes())
	if err != nil {
		return err
	}
	for _, path := range slices.Sorted(maps.Keys(crawlers)) {
		file := crawlers[path]
		if err := emit(map[string]any{"kind": "file", "path": path, "contentType": file.contentType, "data": string(file.body)}); err != nil {
			return err
		}
	}
	return emit(map[string]any{"kind": "complete"})
}
