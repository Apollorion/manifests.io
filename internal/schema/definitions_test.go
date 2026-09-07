package schema

import (
	"errors"
	"net/url"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestDefinitionsIncludeNestedTypesAcrossTheCorpus(t *testing.T) {
	c, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, product := range c.Products() {
		for _, version := range product.Versions {
			q := Query{Item: product.Name, Version: version}
			definitions, err := c.Definitions(q)
			if err != nil {
				t.Fatal(err)
			}
			doc := c.documents[product.Name+"/"+version]
			seen := make(map[string]bool)
			for i, definition := range definitions {
				if seen[definition.Resource] {
					t.Fatalf("duplicate definition %s in %v", definition.Resource, q)
				}
				seen[definition.Resource] = true
				if i > 0 {
					previous := definitions[i-1]
					if previous.Name > definition.Name || (previous.Name == definition.Name && previous.Resource >= definition.Resource) {
						t.Fatalf("unsorted definitions: %+v then %+v", previous, definition)
					}
				}
				if _, err := c.Page(queryFromHref(t, definition.Href)); err != nil {
					t.Fatalf("definition %s has invalid href %s: %v", definition.Resource, definition.Href, err)
				}
			}
			for name := range doc.schemas {
				if !seen[name] {
					t.Errorf("missing schema %s in %v", name, q)
				}
			}
			for name := range doc.aliases {
				if !seen[name] {
					t.Errorf("missing nested CRD definition %s in %v", name, q)
				}
			}
			if product.Name == "kubernetes" && (!seen["io.k8s.api.core.v1.ContainerStatus"] || !seen["io.k8s.api.core.v1.Pod"]) {
				t.Errorf("top-level and nested Kubernetes types must both be searchable in %s", version)
			}
		}
	}
}

func TestDefinitionsPreserveIdentityAndEscapeURLs(t *testing.T) {
	doc := newDocument(openapi3.Schemas{
		"group.v1.Thing":   {Value: openapi3.NewObjectSchema()},
		"group.v2.Thing":   {Value: openapi3.NewObjectSchema()},
		"group.v1.Thing?#": {Value: openapi3.NewObjectSchema()},
	})
	c := &Catalog{documents: map[string]*document{"gateway api/1.2.0 standard": doc}}
	definitions, err := c.Definitions(Query{Item: "gateway api", Version: "1.2.0 standard"})
	if err != nil {
		t.Fatal(err)
	}
	if len(definitions) != 3 || definitions[0].Name != "Thing" || definitions[1].Name != "Thing" || definitions[0].Resource == definitions[1].Resource {
		t.Fatalf("same-named types were lost: %+v", definitions)
	}
	for _, definition := range definitions {
		u, err := url.Parse(definition.Href)
		if err != nil || u.RawQuery != "" || u.Fragment != "" || u.Host != "" || strings.Contains(definition.Href, " ") {
			t.Fatalf("unsafe definition URL %q: %v", definition.Href, err)
		}
		if u.Path != "/gateway api/1.2.0 standard/"+definition.Resource {
			t.Fatalf("definition URL changed identity: %+v", definition)
		}
	}
}

func TestDefinitionsRejectInvalidQueries(t *testing.T) {
	c, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []Query{
		{}, {Item: "kubernetes"}, {Version: "1.34"},
		{Item: "kubernetes/", Version: "1.34"},
		{Item: "kubernetes", Version: "1.34", Resource: "io.k8s.api.core.v1.Pod"},
		{Item: "kubernetes", Version: "1.34", Path: "Pod.spec"},
	} {
		if _, err := c.Definitions(q); !errors.Is(err, ErrBadQuery) {
			t.Errorf("query %+v: got %v, want bad query", q, err)
		}
	}
	for _, q := range []Query{{Item: "missing", Version: "1.34"}, {Item: "kubernetes", Version: "missing"}} {
		if _, err := c.Definitions(q); !errors.Is(err, ErrNotFound) {
			t.Errorf("query %+v: got %v, want not found", q, err)
		}
	}
}
