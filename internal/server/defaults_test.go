package server

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/TheOutdoorProgrammer/manifests.io/internal/schema"
)

type futureCatalog struct{ fakeCatalog }

func (futureCatalog) Products() []schema.Product {
	return []schema.Product{{Name: "kubernetes", Versions: []string{"1.9", "1.100", "1.99"}}}
}

func (futureCatalog) Page(query schema.Query) (schema.Page, error) {
	if query.Item == "kubernetes" && query.Version == "1.9" && query.Resource == "" {
		return schema.Page{Item: query.Item, Version: query.Version}, nil
	}
	return schema.Page{}, schema.ErrNotFound
}

func TestRootAndErrorRecoveryFollowLatestKubernetes(t *testing.T) {
	s := testServer(t)
	s.catalog = futureCatalog{}
	for _, method := range []string{"GET", "HEAD"} {
		response := httptest.NewRecorder()
		s.ServeHTTP(response, httptest.NewRequest(method, "/", nil))
		if response.Code != 307 || response.Header().Get("Location") != "/kubernetes/1.100" {
			t.Fatalf("root %s: %d %s", method, response.Code, response.Header().Get("Location"))
		}
	}
	for _, tc := range []struct{ url, version string }{
		{"/api/page?item=missing&version=missing", "1.100"},
		{"/api/page?item=kubernetes&version=1.9&resource=missing", "1.9"},
	} {
		response := httptest.NewRecorder()
		s.ServeHTTP(response, httptest.NewRequest("GET", tc.url, nil))
		var page schema.Page
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if response.Code != 404 || page.Item != "kubernetes" || page.Version != tc.version {
			t.Fatalf("%s: %d %+v", tc.url, response.Code, page)
		}
	}
}
