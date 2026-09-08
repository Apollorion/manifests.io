package schema

import (
	"reflect"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestAPIVersionsDoNotMultiplySharedKinds(t *testing.T) {
	d := &document{
		schemas: openapi3.Schemas{
			"Shared": {Value: openapi3.NewObjectSchema()},
			"Other":  {Value: openapi3.NewObjectSchema()},
		},
		gvks: map[string][]gvk{
			"Shared": {{Group: "a", Version: "v1", Kind: "Options"}, {Group: "b", Version: "v1", Kind: "Options"}, {Group: "b", Version: "v2", Kind: "Options"}, {Group: "c", Version: "v1", Kind: "Alternate"}},
			"Other":  {{Group: "a", Version: "v2", Kind: "Options"}, {Group: "c", Version: "v2", Kind: "Alternate"}, {Group: "a", Version: "v1", Kind: "Unrelated"}},
		},
	}
	c := &Catalog{documents: map[string]*document{"test/1": d}}
	p, err := c.Page(Query{Item: "test", Version: "1", Resource: "Shared"})
	if err != nil {
		t.Fatal(err)
	}
	want := []Link{
		{Label: "a/v2/Options", Href: "/test/1/Other"},
		{Label: "c/v2/Alternate", Href: "/test/1/Other"},
		{Label: "a/v1/Options", Href: "/test/1/Shared"},
		{Label: "b/v1/Options", Href: "/test/1/Shared"},
		{Label: "b/v2/Options", Href: "/test/1/Shared"},
		{Label: "c/v1/Alternate", Href: "/test/1/Shared"},
	}
	if !reflect.DeepEqual(p.OtherVersions, want) {
		t.Fatalf("API versions = %v, want %v", p.OtherVersions, want)
	}
}

func TestDeleteOptionsAPIVersions(t *testing.T) {
	c, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	for _, product := range c.Products() {
		if product.Name != "kubernetes" {
			continue
		}
		for _, version := range product.Versions {
			t.Run(version, func(t *testing.T) {
				const resource = "io.k8s.apimachinery.pkg.apis.meta.v1.DeleteOptions"
				p, err := c.Page(Query{Item: product.Name, Version: version, Resource: resource, Path: "DeleteOptions.orphanDependents", Pointer: "/properties/orphanDependents"})
				if err != nil {
					t.Fatal(err)
				}
				declared := c.documents[product.Name+"/"+version].gvks[resource]
				if len(p.OtherVersions) != len(declared) {
					t.Fatalf("got %d API version links for %d declared versions", len(p.OtherVersions), len(declared))
				}
				seen := make(map[Link]bool)
				for _, link := range p.OtherVersions {
					if seen[link] {
						t.Fatalf("duplicate API version: %v", link)
					}
					seen[link] = true
				}
			})
		}
	}
}
