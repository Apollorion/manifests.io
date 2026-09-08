package schema

import "testing"

func TestDefaultKubernetesVersionUsesSemanticOrder(t *testing.T) {
	c := &Catalog{
		products: []Product{
			{Name: "flux", Versions: []string{"99.0.0"}},
			{Name: "kubernetes", Versions: []string{"1.9", "1.100", "1.99", "1.101.0-rc.1", "invalid"}},
		},
		documents: map[string]*document{"kubernetes/1.100": {}, "kubernetes/1.9": {}},
	}
	page, err := c.Page(Query{})
	if err != nil {
		t.Fatal(err)
	}
	if page.Version != "1.100" || page.Canonical != "/kubernetes/1.100" {
		t.Fatalf("default page = %+v", page)
	}
	if page.Catalog[1].DefaultVersion != "1.100" {
		t.Fatalf("catalog default = %q", page.Catalog[1].DefaultVersion)
	}
	explicit, err := c.Page(Query{Item: "kubernetes", Version: "1.9"})
	if err != nil || explicit.Version != "1.9" {
		t.Fatalf("explicit version changed: %+v, %v", explicit, err)
	}
}

func TestDefaultKubernetesVersionRequiresAnAvailableStableVersion(t *testing.T) {
	for _, products := range [][]Product{nil, {{Name: "flux", Versions: []string{"2.0.0"}}}, {{Name: "kubernetes", Versions: []string{"invalid", "1.100.0-rc.1"}}}} {
		if query := DefaultQuery(products); query.Version != "" {
			t.Fatalf("invented a default version: %+v", query)
		}
	}
}
