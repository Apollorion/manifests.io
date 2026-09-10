package schema

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func cycleCatalog(t *testing.T, schemas string) *Catalog {
	t.Helper()
	d, err := readOpenAPI([]byte(`{"openapi":"3.0.3","components":{"schemas":` + schemas + `}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.indexNodes(); err != nil {
		t.Fatal(err)
	}
	return &Catalog{documents: map[string]*document{"example/1": d, "example/2": d}}
}

var circularSchemaCases = []struct {
	name, schemas string
	clicks        int
	variant       bool
}{
	{"mutual", `{"A":{"properties":{"next":{"$ref":"#/components/schemas/B"}}},"B":{"properties":{"next":{"$ref":"#/components/schemas/A"}}}}`, 5, false},
	{"array", `{"A":{"properties":{"next":{"type":"array","items":{"$ref":"#/components/schemas/A"}}}}}`, 2, false},
	{"map", `{"A":{"additionalProperties":{"$ref":"#/components/schemas/A"}}}`, 2, false},
	{"inline", `{"A":{"properties":{"next":{"properties":{"next":{"$ref":"#/components/schemas/A"}}}}}}`, 5, false},
	{"oneOf", `{"A":{"oneOf":[{"$ref":"#/components/schemas/A"}]}}`, 2, true},
	{"anyOf", `{"A":{"anyOf":[{"$ref":"#/components/schemas/A"}]}}`, 2, true},
	{"allOf", `{"A":{"allOf":[{"$ref":"#/components/schemas/A"}]}}`, 2, true},
}

func TestCircularLinksAcrossSchemaShapes(t *testing.T) {
	for _, tc := range circularSchemaCases {
		t.Run(tc.name, func(t *testing.T) {
			c := cycleCatalog(t, tc.schemas)
			q := Query{Item: "example", Version: "1", Resource: "A"}
			for step := 0; step <= tc.clicks; step++ {
				p, err := c.Page(q)
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(p.Canonical, "trail=") {
					t.Fatal("visit history leaked into canonical URL")
				}
				reloaded, err := c.Page(q)
				if err != nil || !reflect.DeepEqual(p, reloaded) {
					t.Fatal("refresh changed visit counts")
				}
				var href string
				var blocked bool
				if tc.variant {
					href, blocked = p.Variants[0].Href, p.Variants[0].Circular
				} else {
					href, blocked = p.Resources[0].Href, p.Resources[0].Circular
				}
				if step == tc.clicks {
					if !blocked || href != "" {
						t.Fatalf("fourth visit allowed: %+v", p)
					}
					q.Version = "2"
					other, err := c.Page(q)
					if err != nil || !reflect.DeepEqual(p.Resources, other.Resources) || !reflect.DeepEqual(p.Variants, other.Variants) {
						t.Fatal("version switch lost cycle limit")
					}
				} else {
					if blocked || href == "" {
						t.Fatalf("blocked before three visits: %+v", p)
					}
					q = queryFromHref(t, href)
				}
			}
		})
	}
}

func TestCircularLimitLeavesOtherFieldsOpen(t *testing.T) {
	c := cycleCatalog(t, `{"A":{"properties":{"loop":{"$ref":"#/components/schemas/A"},"other":{"$ref":"#/components/schemas/B"}}},"B":{"properties":{"other":{"properties":{"other":{"type":"string"}}}}}}`)
	q := Query{Item: "example", Version: "1", Resource: "A"}
	for range 2 {
		p, err := c.Page(q)
		if err != nil {
			t.Fatal(err)
		}
		q = queryFromHref(t, p.Resources[0].Href)
	}
	p, err := c.Page(q)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Resources[0].Circular || p.Resources[1].Circular || p.Resources[1].Href == "" {
		t.Fatalf("unrelated field blocked: %+v", p.Resources)
	}
	q = queryFromHref(t, p.Resources[1].Href)
	for range 2 {
		p, err = c.Page(q)
		if err != nil {
			t.Fatal(err)
		}
		if p.Resources[0].Circular {
			t.Fatal("repeated field name mistaken for schema cycle")
		}
		if p.Resources[0].Href != "" {
			q = queryFromHref(t, p.Resources[0].Href)
		}
	}
	if len(c.documents["example/1"].cyclic) != 1 {
		t.Fatal("shared acyclic schema classified as recursive")
	}
}

func TestInvalidCircularHistory(t *testing.T) {
	c := cycleCatalog(t, `{"A":{"properties":{"next":{"$ref":"#/components/schemas/A"}}}}`)
	for _, trail := range []string{`null`, `[]`, `{`, `{"A#":0}`, `{"A#":-1}`, `{"A#":4}`, `{"A#":3}`, `{"A#":1.5}`, strings.Repeat("x", 4097)} {
		if _, err := c.Page(Query{Item: "example", Version: "1", Resource: "A", Trail: trail}); !errors.Is(err, ErrBadQuery) {
			t.Errorf("accepted invalid trail %q: %v", trail, err)
		}
	}
}

func TestPageExposesCanonicalCycleIdentities(t *testing.T) {
	c := cycleCatalog(t, `{"A":{"properties":{"next":{"$ref":"#/components/schemas/B"},"plain":{"type":"string"}}},"B":{"properties":{"next":{"$ref":"#/components/schemas/A"}}}}`)
	page, err := c.Page(Query{Item: "example", Version: "1", Resource: "A"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(page.Cycles, []string{"A#", "B#"}) {
		t.Fatalf("unexpected cycle identities: %v", page.Cycles)
	}
	for _, id := range page.Cycles {
		resource, pointer, ok := strings.Cut(id, "#")
		selected, err := c.Page(Query{Item: "example", Version: "1", Resource: resource, Pointer: pointer})
		if !ok || err != nil || selected.Resource+"#"+selected.Pointer != id {
			t.Fatalf("cycle identity does not select its canonical schema: %q", id)
		}
	}
	listing, err := c.Page(Query{Item: "example", Version: "1"})
	if err != nil || len(listing.Cycles) != 0 {
		t.Fatal("resource listing included unnecessary cycle metadata")
	}
}

func TestLeafMetadataDistinguishesFallbackRows(t *testing.T) {
	c := cycleCatalog(t, `{"A":{"properties":{"A":{"type":"string"}}}}`)
	for _, pointer := range []string{"", "/properties/A"} {
		page, err := c.Page(Query{Item: "example", Version: "1", Resource: "A", Pointer: pointer})
		if err != nil || page.Leaf != (pointer != "") {
			t.Fatalf("leaf metadata confused a named property with a fallback row: %+v, %v", page, err)
		}
	}
}
