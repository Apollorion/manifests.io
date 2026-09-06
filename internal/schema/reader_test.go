package schema

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const exampleCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
spec:
  group: example.io
  names:
    kind: Example
  versions:
    - name: v1
      schema:
        openAPIV3Schema:
          type: object
          required: [spec]
          properties:
            spec:
              type: object
              properties:
                port:
                  x-kubernetes-int-or-string: true
                  description: Named or numeric port.
                  anyOf:
                    - type: integer
                    - type: string
                mapping:
                  type: object
                  additionalProperties:
                    type: string
                freeform:
                  type: object
                  additionalProperties: true
                config:
                  oneOf:
                    - title: Hosted
                      type: object
                      required: [region]
                      properties:
                        region:
                          type: string
                          enum: [us, eu]
                          default: us
                    - title: Local
                      type: object
                      properties:
                        path:
                          type: string
                a/b~c:
                  type: object
                  properties:
                    value:
                      type: integer
                      minimum: 1
                      maximum: 5
                      exclusiveMinimum: true
`

func TestCRDWrappersAndSchemaFeatures(t *testing.T) {
	dir := t.TempDir()
	list := "kind: List\nitems:\n  - " + strings.ReplaceAll(strings.TrimSuffix(exampleCRD, "\n"), "\n", "\n    ") + "\n---\nkind: ConfigMap\nmetadata:\n  name: ignored\n"
	if err := os.WriteFile(filepath.Join(dir, "crds.yaml"), []byte(list), 0600); err != nil {
		t.Fatal(err)
	}
	d, err := readCRDs(dir)
	if err != nil {
		t.Fatal(err)
	}
	c := &Catalog{products: []Product{{"example", []string{"1"}}}, documents: map[string]*document{"example/1": d}}
	q := Query{Item: "example", Version: "1", Resource: "io.example.v1.Example", Pointer: "/properties/spec"}
	p, err := c.Page(q)
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]Row{}
	for _, row := range p.Resources {
		rows[row.Name] = row
	}
	if rows["port"].Type != "integer | string" || rows["port"].Description != "Named or numeric port." {
		t.Fatalf("port: %+v", rows["port"])
	}
	if rows["mapping"].Type != "map[string]string" || rows["freeform"].Type != "map[string]any" {
		t.Fatal("map types lost")
	}
	if len(rows["config"].Variants) != 2 {
		t.Fatal("union alternatives missing")
	}
	for _, row := range p.Resources {
		if row.Href != "" {
			if _, err := c.Page(queryFromHref(t, row.Href)); err != nil {
				t.Errorf("%s: %v", row.Href, err)
			}
		}
		for _, variant := range row.Variants {
			if _, err := c.Page(queryFromHref(t, variant.Href)); err != nil {
				t.Errorf("%s: %v", variant.Href, err)
			}
		}
	}
	q.OneOf, q.Key, q.Path = "Hosted", "config", "Example.spec"
	p, err = c.Page(q)
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Example.spec.config" || len(p.Resources) != 1 || !p.Resources[0].Required {
		t.Fatalf("union: %+v", p)
	}
	if !slices.Contains(p.Resources[0].Constraints, `enum: ["us","eu"]`) || !slices.Contains(p.Resources[0].Constraints, `default: "us"`) {
		t.Fatal("enum/default lost")
	}
	q.OneOf, q.Key, q.Path = "", "", ""
	q.Pointer = "/properties/spec/properties/a~1b~0c"
	p, err = c.Page(q)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(p.Resources[0].Constraints, "exclusiveMinimum: true") {
		t.Fatal("exclusive bound lost")
	}
	q.Pointer = "/properties/spec/properties/mapping"
	p, err = c.Page(q)
	if err != nil {
		t.Fatal(err)
	}
	if p.Resources[0].Name != "[key]" || p.Resources[0].Type != "string" {
		t.Fatalf("map page: %+v", p)
	}
	q.Resource = "io.example.v1.ExampleSpec"
	q.Pointer = ""
	p, err = c.Page(q)
	if err != nil {
		t.Fatal(err)
	}
	if p.Pointer != "/properties/spec" || p.Resource != "io.example.v1.Example" {
		t.Fatal("legacy alias not canonicalized")
	}
}

func TestMalformedCRDs(t *testing.T) {
	for name, data := range map[string]string{
		"invalid YAML":   "kind: [\n",
		"missing spec":   "kind: CustomResourceDefinition\n",
		"missing schema": strings.Replace(exampleCRD, "openAPIV3Schema:", "missingSchema:", 1),
		"duplicate":      exampleCRD + "---\n" + exampleCRD,
		"external ref":   strings.Replace(exampleCRD, "type: object", "$ref: https://example.com/schema.json", 1),
		"unresolved ref": strings.Replace(exampleCRD, "type: object", "$ref: '#/components/schemas/Missing'", 1),
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "crd.yaml"), []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := readCRDs(dir); err == nil {
				t.Fatal("malformed schema accepted")
			}
		})
	}
}

func TestBoundedCyclicNavigation(t *testing.T) {
	d, err := readOpenAPI([]byte(`{"openapi":"3.0.3","components":{"schemas":{"Thing":{"type":"object","properties":{"self":{"$ref":"#/components/schemas/Thing"}}}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.indexNodes(); err != nil {
		t.Fatal(err)
	}
	c := &Catalog{documents: map[string]*document{"example/1": d}}
	q := Query{Item: "example", Version: "1", Resource: "Thing"}
	for i := 0; i < 3; i++ {
		p, err := c.Page(q)
		if err != nil {
			t.Fatalf("depth %d: %v", i, err)
		}
		if len(p.Resources) != 1 {
			t.Fatal("cyclic schema expanded recursively")
		}
		if i == 2 {
			if !p.Resources[0].Circular || p.Resources[0].Href != "" {
				t.Fatal("fourth schema visit was not blocked")
			}
		} else {
			q = queryFromHref(t, p.Resources[0].Href)
		}
	}
	q.Pointer = strings.Repeat("/properties/self", 65)
	if _, err := c.Page(q); !errors.Is(err, ErrBadQuery) {
		t.Fatalf("depth limit: %v", err)
	}
}

func queryFromHref(t *testing.T, href string) Query {
	t.Helper()
	u, err := url.Parse(href)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	q := Query{Item: parts[0], Version: parts[1]}
	if len(parts) > 2 {
		q.Resource = parts[2]
	}
	values := u.Query()
	q.Pointer, q.Path, q.OneOf, q.Key = values.Get("pointer"), values.Get("path"), values.Get("oneOf"), values.Get("key")
	q.Trail = values.Get("trail")
	return q
}
