package schema

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestStaticGraphPreservesCanonicalSelection(t *testing.T) {
	for _, tc := range circularSchemaCases {
		t.Run(tc.name, func(t *testing.T) {
			c := cycleCatalog(t, tc.schemas)
			graph, err := c.StaticDocument(Query{Item: "example", Version: "1"})
			if err != nil {
				t.Fatal(err)
			}
			for id, node := range graph.Nodes {
				page, err := c.Page(node.Query)
				if err != nil || page.Canonical != node.Canonical || page.Resource+"#"+page.Pointer != id {
					t.Fatalf("static node differs from canonical page: %s, %v", id, err)
				}
				for pointer, target := range node.Edges {
					q := node.Query
					q.Pointer += pointer
					selected, err := c.Page(q)
					if err != nil || selected.Canonical != graph.Nodes[target].Canonical {
						t.Fatalf("static edge differs from schema navigation: %s%s, %v", id, pointer, err)
					}
				}
			}
		})
	}
}

func TestStaticGraphAliasesAndLegacyChoices(t *testing.T) {
	c := cycleCatalog(t, `{"A":{"properties":{"escaped/~":{"type":"string"},"choice":{"oneOf":[{"title":"First","type":"string"},{"type":"integer"}]}}}}`)
	c.documents["example/1"].aliases = map[string]location{"Alias": {resource: "A", path: "/properties/choice"}}
	graph, err := c.StaticDocument(Query{Item: "example", Version: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if graph.Resources["Alias"] != "A#/properties/choice" {
		t.Fatalf("legacy alias lost inline identity: %v", graph.Resources)
	}
	root := graph.Nodes[graph.Resources["A"]]
	if root.Edges["/properties/escaped~1~0"] == "" {
		t.Fatal("RFC6901 escaped property edge missing")
	}
	choices := graph.Nodes[root.Edges["/properties/choice"]].OneOf
	for _, choice := range choices {
		page, err := c.Page(Query{Item: "example", Version: "1", Resource: "A", Key: "choice", OneOf: choice.Title})
		if err != nil || page.Canonical != graph.Nodes[choice.Node].Canonical {
			t.Fatalf("legacy title selection differs: %+v, %v", choice, err)
		}
	}
	data, err := json.Marshal(graph)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"Query"`) || strings.Contains(string(data), `"description"`) {
		t.Fatal("static routing graph includes non-routing schema data")
	}
	again, err := c.StaticDocument(Query{Item: "example", Version: "1"})
	if err != nil || !reflect.DeepEqual(graph, again) {
		t.Fatal("static graph generation changed catalog state")
	}
}
