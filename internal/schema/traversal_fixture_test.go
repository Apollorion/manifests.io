package schema

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"testing"
)

func TestTraversalFixtures(t *testing.T) {
	type fixture struct {
		Name      string `json:"name"`
		Search    string `json:"search"`
		Canonical Page   `json:"canonical"`
		Expected  Page   `json:"expected"`
	}
	var fixtures []fixture
	add := func(name string, c *Catalog, q Query) Page {
		t.Helper()
		expected, err := c.Page(q)
		if err != nil {
			t.Fatal(err)
		}
		base := q
		base.Path, base.Trail = "", ""
		canonical, err := c.Page(base)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := url.Parse(Href(q))
		if err != nil {
			t.Fatal(err)
		}
		fixtures = append(fixtures, fixture{Name: name, Search: parsed.RawQuery, Canonical: canonical, Expected: expected})
		return expected
	}
	for _, tc := range circularSchemaCases {
		c := cycleCatalog(t, tc.schemas)
		q := Query{Item: "example", Version: "1", Resource: "A", Path: "Workload.schema"}
		for step := 0; step <= tc.clicks; step++ {
			p := add(tc.name+"-"+strconv.Itoa(step), c, q)
			if step == tc.clicks {
				break
			}
			if tc.variant {
				q = queryFromHref(t, p.Variants[0].Href)
			} else {
				q = queryFromHref(t, p.Resources[0].Href)
			}
		}
	}
	c := cycleCatalog(t, `{"A":{"properties":{"value":{"type":"string"},"choice":{"oneOf":[{"title":"First","type":"object","properties":{"field":{"type":"string"}}},{"title":"Second","type":"integer"}]}}}}`)
	add("inline-scalar", c, Query{Item: "example", Version: "1", Resource: "A", Pointer: "/properties/value", Path: "Workload.value"})
	add("legacy-oneof-object", c, Query{Item: "example", Version: "1", Resource: "A", Key: "choice", OneOf: "First", Path: "Workload"})
	add("legacy-oneof-leaf", c, Query{Item: "example", Version: "1", Resource: "A", Key: "choice", OneOf: "Second", Path: "Workload"})
	data, err := json.MarshalIndent(fixtures, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("MANIFESTS_PRINT_TRAVERSAL_FIXTURES") == "1" {
		fmt.Println(string(data))
		return
	}
	stored, err := os.ReadFile("../../frontend/src/fixtures/traversal.json")
	if err != nil {
		t.Fatal(err)
	}
	var current []fixture
	if err := json.Unmarshal(stored, &current); err != nil {
		t.Fatal(err)
	}
	normalized, err := json.MarshalIndent(current, "", "  ")
	if err != nil || string(normalized) != string(data) {
		t.Fatal("browser traversal fixtures differ from the catalog; regenerate with MANIFESTS_PRINT_TRAVERSAL_FIXTURES=1 go test -v ./internal/schema -run '^TestTraversalFixtures$'")
	}
}
