package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/TheOutdoorProgrammer/manifests.io/internal/schema"
)

func TestDeleteOptionsContextualRendering(t *testing.T) {
	const bundle = "../../frontend/dist-render/renderer.js"
	if _, err := os.Stat(bundle); os.IsNotExist(err) {
		t.Skip("run npm --prefix frontend run build for React integration")
	}
	catalog, err := schema.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	s := testServer(t)
	s.catalog = catalog
	s.renderer, err = newReactRenderer(bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, product := range catalog.Products() {
		if product.Name != "kubernetes" {
			continue
		}
		for _, version := range product.Versions {
			for _, field := range []struct{ path, pointer string }{
				{"orphanDependents", "/properties/orphanDependents"},
				{"dryRun[]", "/properties/dryRun/items"},
				{"propagationPolicy", "/properties/propagationPolicy"},
				{"kind", "/properties/kind"},
			} {
				t.Run(version+"/"+field.path, func(t *testing.T) {
					q := schema.Query{Item: product.Name, Version: version, Resource: "io.k8s.apimachinery.pkg.apis.meta.v1.DeleteOptions", Path: "DeleteOptions." + field.path, Pointer: field.pointer}
					request := httptest.NewRequest(http.MethodGet, schema.Href(q), nil)
					response := httptest.NewRecorder()
					started := time.Now()
					s.ServeHTTP(response, request)
					t.Logf("contextual rendering: status=%d elapsed=%s bytes=%d", response.Code, time.Since(started), response.Body.Len())
					if response.Code != http.StatusOK {
						t.Fatalf("contextual documentation failed: %d %s", response.Code, response.Body)
					}
					if !strings.Contains(response.Body.String(), "<h1>"+q.Path+"</h1>") || !strings.Contains(response.Body.String(), `aria-label="API versions"`) {
						t.Fatal("missing server-rendered field heading or API versions")
					}
				})
			}
		}
	}
}
