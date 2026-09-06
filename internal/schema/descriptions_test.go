package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestKubernetesFieldMetadataParity(t *testing.T) {
	c, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("../../oaspec/kubernetes/*.json")
	if err != nil {
		t.Fatal(err)
	}
	fields, refs := 0, 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var source struct {
			Definitions map[string]struct {
				Description string   `json:"description"`
				Required    []string `json:"required"`
				Properties  map[string]struct {
					Description string `json:"description"`
					Ref         string `json:"$ref"`
				} `json:"properties"`
			} `json:"definitions"`
		}
		if err := json.Unmarshal(data, &source); err != nil {
			t.Fatal(err)
		}
		version := filepath.Base(file)
		version = version[:len(version)-len(".json")]
		for name, original := range source.Definitions {
			p, err := c.Page(Query{Item: "kubernetes", Version: version, Resource: name})
			if err != nil {
				t.Fatal(err)
			}
			if p.Description != original.Description {
				t.Errorf("%s %s: target description changed", version, name)
			}
			rows := map[string]Row{}
			for _, row := range p.Resources {
				rows[row.Name] = row
			}
			for key, field := range original.Properties {
				row, ok := rows[key]
				if !ok {
					t.Errorf("%s %s: missing field %s", version, name, key)
					continue
				}
				if row.Description != field.Description {
					t.Errorf("%s %s.%s: description %q, want %q", version, name, key, row.Description, field.Description)
				}
				if row.Required != slices.Contains(original.Required, key) {
					t.Errorf("%s %s.%s: required flag changed", version, name, key)
				}
				fields++
				if field.Ref != "" {
					refs++
				}
			}
		}
	}
	if fields == 0 || refs == 0 {
		t.Fatal("corpus metadata comparison did not cover references")
	}
	t.Logf("validated descriptions and required flags for %d fields, including %d references", fields, refs)
}

func TestReferenceDescriptionsAreEdgeSpecific(t *testing.T) {
	for _, version := range []string{"2.0", "3.0.3", "3.1.0"} {
		t.Run(version, func(t *testing.T) {
			prefix := `{"swagger":"2.0","definitions":`
			refPrefix := "#/definitions/"
			suffix := "}"
			if version != "2.0" {
				prefix = `{"openapi":"` + version + `","components":{"schemas":`
				suffix = "}}"
				refPrefix = "#/components/schemas/"
			}
			data := prefix + `{
				"Thing":{"type":"object","description":"Shared target","properties":{"value":{"type":"string"}}},
				"Root":{"type":"object","properties":{
					"left":{"$ref":"` + refPrefix + `Thing","description":"Left field"},
					"right":{"$ref":"` + refPrefix + `Thing","description":"Right field"},
					"missing":{"$ref":"` + refPrefix + `Thing"},
					"empty":{"$ref":"` + refPrefix + `Thing","description":""},
					"nested":{"type":"object","properties":{"child":{"$ref":"` + refPrefix + `Thing","description":"Nested field"}}}
				}}
			}` + suffix
			d, err := readOpenAPI([]byte(data))
			if err != nil {
				t.Fatal(err)
			}
			c := &Catalog{documents: map[string]*document{"example/1": d}}
			p, err := c.Page(Query{Item: "example", Version: "1", Resource: "Root"})
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]string{"left": "Left field", "right": "Right field", "missing": "", "empty": "", "nested": ""}
			for _, row := range p.Resources {
				if row.Description != want[row.Name] {
					t.Errorf("%s description: %q", row.Name, row.Description)
				}
			}
			p, err = c.Page(Query{Item: "example", Version: "1", Resource: "Root", Path: "/properties/nested"})
			if err != nil {
				t.Fatal(err)
			}
			if p.Resources[0].Description != "Nested field" {
				t.Fatal("nested reference description missing")
			}
			if d.schemas["Thing"].Value.Description != "Shared target" {
				t.Fatal("shared target mutated")
			}
		})
	}
}
