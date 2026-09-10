package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/TheOutdoorProgrammer/manifests.io/internal/schema"
)

var loadCorpus = sync.OnceValues(func() (*schema.Catalog, error) {
	return schema.Load("../..")
})

func corpusCatalog(t testing.TB) *schema.Catalog {
	t.Helper()
	catalog, err := loadCorpus()
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

type fakeCatalog struct{}

func (fakeCatalog) Products() []schema.Product {
	return []schema.Product{{Name: "kubernetes", Versions: []string{"1.34"}}}
}

func (fakeCatalog) Definitions(q schema.Query) ([]schema.Definition, error) {
	if q.Item != "kubernetes" || q.Version != "1.34" {
		return nil, schema.ErrNotFound
	}
	return []schema.Definition{}, nil
}

func (fakeCatalog) Routes() []schema.Query {
	return []schema.Query{{Item: "kubernetes", Version: "1.34"}}
}

func (fakeCatalog) Page(q schema.Query) (schema.Page, error) {
	if q.Item != "kubernetes" || q.Version != "1.34" || q.Resource == "missing" {
		return schema.Page{}, schema.ErrNotFound
	}
	return schema.Page{Item: q.Item, Version: q.Version, Resource: q.Resource, Title: "Pod", Description: "</script><script>alert(1)</script>", Canonical: "/kubernetes/1.34", Catalog: fakeCatalog{}.Products()}, nil
}

func testServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(`<html><head><!--page-head--></head><body><div id="root"><!--app-html--></div><!--page-data--></body></html>`), 0600); err != nil {
		t.Fatal(err)
	}
	rendererFile := filepath.Join(dir, "renderer.js")
	if err := os.WriteFile(rendererFile, []byte(`var ManifestsRenderer = {renderPage: function(data) { var page = JSON.parse(data); return "<main>Fresh rendered fields</main>"; }};`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := New(fakeCatalog{}, Config{WebDir: dir, RenderDir: dir, PublicDir: dir, RendererFile: rendererFile})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestHTTPContract(t *testing.T) {
	s := testServer(t)
	for _, tc := range []struct {
		method string
		url    string
		status int
	}{
		{"GET", "/", 307}, {"GET", "/healthz", 200}, {"GET", "/readyz", 200},
		{"GET", "/api/catalog", 200}, {"GET", "/api/page?item=kubernetes&version=1.34", 200},
		{"GET", "/api/page?item=kubernetes&item=flux&version=1.34", 400},
		{"GET", "/api/page?item=kubernetes&version=1.34&path=%ZZ", 400},
		{"GET", "/api/page?item=kubernetes", 400}, {"GET", "/api/page?item=missing&version=1.34", 404},
		{"GET", "/api/missing", 404}, {"GET", "/kubernetes/1.34/missing", 404},
		{"GET", "/kubernetes/1.34/Pod/extra", 404}, {"GET", "/assets/../index.html", 404},
		{"GET", "/assets/", 404}, {"POST", "/api/page", 405}, {"HEAD", "/kubernetes/1.34", 200},
	} {
		t.Run(tc.method+tc.url, func(t *testing.T) {
			w := httptest.NewRecorder()
			s.ServeHTTP(w, httptest.NewRequest(tc.method, tc.url, nil))
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.status, w.Body)
			}
			if tc.method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD returned a body")
			}
			if w.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatal("security headers missing")
			}
		})
	}
}

func TestBrowserSourceMapsStayPrivate(t *testing.T) {
	s := testServer(t)
	assets := filepath.Join(s.config.WebDir, "assets")
	if err := os.Mkdir(assets, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "index.js.map"), []byte(`{"sourcesContent":["private source"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{"GET", "HEAD"} {
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(method, "/assets/index.js.map", nil))
		if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "private source") {
			t.Fatalf("%s exposed a browser source map: status=%d", method, w.Code)
		}
	}
}

func TestPageDataCannotEscapeScript(t *testing.T) {
	w := httptest.NewRecorder()
	testServer(t).ServeHTTP(w, httptest.NewRequest("GET", "/kubernetes/1.34", nil))
	body := w.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") || !strings.Contains(body, `\u003c/script\u003e`) {
		t.Fatalf("unsafe serialization: %s", body)
	}
	if !strings.Contains(body, "Fresh rendered fields") || !strings.Contains(body, `rel="canonical"`) || strings.Contains(body, "<!--page-") {
		t.Fatal("incomplete template")
	}
}

func TestPrerenderSharedAcrossTraversalQueries(t *testing.T) {
	s := testServer(t)
	file := filepath.Join(s.config.RenderDir, RenderFilename("/kubernetes/1.34"))
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte(`<head><!--page-head--></head><div id="root"><main>Server rendered fields</main></div><!--page-data-->`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, compressed.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest("GET", "/kubernetes/1.34", nil))
	if !strings.Contains(w.Body.String(), "Server rendered fields") {
		t.Fatal("prerendered content absent")
	}
	if strings.Contains(w.Body.String(), " inert") {
		t.Fatal("canonical page navigation was disabled")
	}
	r := httptest.NewRequest("GET", "/kubernetes/1.34", nil)
	r.Header.Set("If-None-Match", w.Header().Get("ETag"))
	cached := httptest.NewRecorder()
	s.ServeHTTP(cached, r)
	if cached.Code != http.StatusNotModified || cached.Body.Len() != 0 {
		t.Fatal("conditional request did not return 304")
	}
	dynamic := httptest.NewRecorder()
	s.ServeHTTP(dynamic, httptest.NewRequest("GET", "/kubernetes/1.34?path=Deployment.spec.template.spec", nil))
	if dynamic.Body.String() != w.Body.String() || dynamic.Header().Get("ETag") != w.Header().Get("ETag") {
		t.Fatal("traversal query changed the shared HTML or ETag")
	}
}

func TestErrorsRemainMachineReadable(t *testing.T) {
	w := httptest.NewRecorder()
	testServer(t).ServeHTTP(w, httptest.NewRequest("GET", "/api/page?item=missing&version=1", nil))
	var page schema.Page
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || page.Error == "" || len(page.Catalog) == 0 {
		t.Fatalf("error lacks recovery context: %s", w.Body)
	}
}

func TestTraversalSharesHTMLAndAPIData(t *testing.T) {
	catalog := corpusCatalog(t)
	s := testServer(t)
	s.catalog = catalog
	version := schema.DefaultQuery(catalog.Products()).Version
	resource := "io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.JSONSchemaProps"
	base := "/kubernetes/" + version + "/" + resource
	canonical := httptest.NewRecorder()
	s.ServeHTTP(canonical, httptest.NewRequest(http.MethodGet, base, nil))
	apiURL := "/api/page?item=kubernetes&version=" + version + "&resource=" + resource
	canonicalAPI := httptest.NewRecorder()
	s.ServeHTTP(canonicalAPI, httptest.NewRequest(http.MethodGet, apiURL, nil))
	for _, query := range []string{
		"?path=First.allOf&trail=%7B%22" + resource + "%23%22%3A2%7D",
		"?linked=Second.anyOf",
		"?path=First.allOf&trail=invalid",
	} {
		response := httptest.NewRecorder()
		s.ServeHTTP(response, httptest.NewRequest(http.MethodGet, base+query, nil))
		if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), canonical.Body.Bytes()) || response.Header().Get("ETag") != canonical.Header().Get("ETag") {
			t.Fatal("visitor traversal changed shared HTML")
		}
		api := httptest.NewRecorder()
		s.ServeHTTP(api, httptest.NewRequest(http.MethodGet, apiURL+"&"+strings.TrimPrefix(query, "?"), nil))
		var page schema.Page
		if api.Code != http.StatusOK || !bytes.Equal(api.Body.Bytes(), canonicalAPI.Body.Bytes()) || json.Unmarshal(api.Body.Bytes(), &page) != nil || page.Path != "" || page.Trail != "" {
			t.Fatal("visitor traversal changed shared API data")
		}
		if len(page.Cycles) == 0 {
			t.Fatal("shared API lacks browser cycle metadata")
		}
	}
	spec := httptest.NewRecorder()
	s.ServeHTTP(spec, httptest.NewRequest(http.MethodGet, base+"?pointer=%2Fproperties%2Fdescription&path=First.description", nil))
	if spec.Code != http.StatusOK || bytes.Equal(spec.Body.Bytes(), canonical.Body.Bytes()) {
		t.Fatal("inline schema selector was ignored")
	}
	specAPI := httptest.NewRecorder()
	s.ServeHTTP(specAPI, httptest.NewRequest(http.MethodGet, apiURL+"&pointer=%2Fproperties%2Fdescription&path=First.description", nil))
	if specAPI.Code != http.StatusOK || bytes.Equal(specAPI.Body.Bytes(), canonicalAPI.Body.Bytes()) {
		t.Fatal("API inline schema selector was ignored")
	}
}

func TestDefinitionsIgnoreTraversalContext(t *testing.T) {
	s := testServer(t)
	s.catalog = corpusCatalog(t)
	base := "/api/definitions?item=kubernetes&version=" + schema.DefaultQuery(s.catalog.Products()).Version
	canonical := httptest.NewRecorder()
	s.ServeHTTP(canonical, httptest.NewRequest(http.MethodGet, base, nil))
	for _, query := range []string{"&path=Deployment.spec", "&linked=Pod.spec&trail=invalid"} {
		response := httptest.NewRecorder()
		s.ServeHTTP(response, httptest.NewRequest(http.MethodGet, base+query, nil))
		if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), canonical.Body.Bytes()) {
			t.Fatal("visitor traversal changed definitions API data")
		}
	}
}

func TestDefinitionsHTTPContract(t *testing.T) {
	catalog := corpusCatalog(t)
	s := testServer(t)
	s.catalog = catalog
	version := schema.DefaultQuery(catalog.Products()).Version
	for _, tc := range []struct {
		method string
		query  string
		status int
	}{
		{"GET", "item=kubernetes&version=" + version, 200},
		{"HEAD", "item=kubernetes&version=" + version, 200},
		{"GET", "item=missing&version=1.34", 404},
		{"GET", "item=kubernetes&version=missing", 404},
		{"HEAD", "item=kubernetes&version=missing", 404},
		{"GET", "item=kubernetes", 400},
		{"GET", "item=kubernetes&item=flux&version=1.34", 400},
		{"GET", "item=%ZZ&version=1.34", 400},
		{"GET", "item=kubernetes%2F1.34&version=1.34", 400},
		{"GET", "item=kubernetes&version=1.34&resource=Pod", 400},
		{"GET", "item=kubernetes&version=1.34&pointer=%2Fproperties%2Fspec", 400},
		{"POST", "item=kubernetes&version=1.34", 405},
	} {
		t.Run(tc.method+tc.query, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, "/api/definitions?"+tc.query, nil)
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.status, w.Body)
			}
			if tc.status != 405 && r.Pattern != "/api/definitions" {
				t.Fatalf("missing low-cardinality route: %q", r.Pattern)
			}
			if tc.method == "HEAD" {
				if w.Body.Len() != 0 {
					t.Fatal("HEAD returned a body")
				}
				return
			}
			if tc.status == 200 {
				var definitions []schema.Definition
				if err := json.Unmarshal(w.Body.Bytes(), &definitions); err != nil {
					t.Fatal(err)
				}
				found := false
				for _, definition := range definitions {
					if definition.Resource == "io.k8s.api.core.v1.ContainerStatus" {
						found = definition.Name == "ContainerStatus" && definition.Href == "/kubernetes/"+version+"/io.k8s.api.core.v1.ContainerStatus"
					}
				}
				if !found {
					t.Fatal("nested ContainerStatus definition missing from endpoint")
				}
			} else if tc.status != 405 {
				var page schema.Page
				if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || page.Error == "" || len(page.Catalog) == 0 {
					t.Fatalf("error lacks recovery context: %s", w.Body)
				}
			}
		})
	}
}

type canonicalCatalog struct {
	fakeCatalog
	page schema.Page
}

func (c canonicalCatalog) Page(schema.Query) (schema.Page, error) {
	return c.page, nil
}

func TestDocumentationKeepsNavigationContextWithoutRedirecting(t *testing.T) {
	const deployment = "/kubernetes/1.34/io.k8s.api.apps.v1.Deployment"
	const podSpec = "/kubernetes/1.34/io.k8s.api.core.v1.PodSpec"
	const context = "?path=Deployment.spec.template.spec"
	for _, tc := range []struct {
		name      string
		method    string
		url       string
		canonical string
	}{
		{"reference context", "GET", podSpec + context, podSpec},
		{"reference context head", "HEAD", podSpec + context, podSpec},
		{"different canonical resource", "GET", deployment + "?pointer=/properties/spec/properties/template/properties/spec", podSpec},
		{"different canonical resource head", "HEAD", deployment + "?pointer=/properties/spec/properties/template/properties/spec", podSpec},
		{"legacy linked reference", "GET", podSpec + "?linked=Deployment.spec.template.spec", podSpec},
		{"canonical resource", "GET", podSpec, podSpec},
		{"inline schema", "GET", deployment + "?pointer=/properties/status&path=Deployment.status", deployment + "?pointer=%2Fproperties%2Fstatus"},
		{"resource listing", "GET", "/kubernetes/1.34", "/kubernetes/1.34"},
		{"API reference", "GET", "/api/page?item=kubernetes&version=1.34&resource=io.k8s.api.core.v1.PodSpec&path=Deployment.spec.template.spec", podSpec},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testServer(t)
			s.catalog = canonicalCatalog{page: schema.Page{Canonical: tc.canonical, Title: "PodSpec", Path: "Deployment.spec.template.spec"}}
			w := httptest.NewRecorder()
			s.ServeHTTP(w, httptest.NewRequest(tc.method, tc.url, nil))
			if w.Code != http.StatusOK {
				t.Fatalf("status=%d want=200 body=%s", w.Code, w.Body)
			}
			if w.Header().Get("Location") != "" {
				t.Fatal("documentation response includes a redirect Location")
			}
			if tc.method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD response returned a body")
			}
			if strings.HasPrefix(tc.url, "/api/") {
				var page schema.Page
				if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || page.Canonical != podSpec || page.Path != "Deployment.spec.template.spec" {
					t.Fatalf("API did not return page JSON with navigation context: %s", w.Body)
				}
			}
		})
	}
}

func TestParseNavigationPathAndPointer(t *testing.T) {
	const resource = "io.k8s.api.core.v1.PodSpec"
	const route = "/kubernetes/1.34/" + resource
	for _, tc := range []struct {
		url     string
		path    string
		pointer string
	}{
		{route + "?path=Deployment.spec.template.spec", "Deployment.spec.template.spec", ""},
		{route + "?path=Deployment.spec.template.spec.containers&pointer=%2Fproperties%2Fcontainers", "Deployment.spec.template.spec.containers", "/properties/containers"},
		{route + "?linked=Deployment.spec.template.spec", "Deployment.spec.template.spec", ""},
		{route + "?path=Pod.spec&linked=Deployment.spec.template.spec", "Pod.spec", ""},
		{"/api/page?item=kubernetes&version=1.34&resource=" + resource + "&path=Pod.spec&pointer=%2Fproperties%2Fcontainers", "Pod.spec", "/properties/containers"},
	} {
		t.Run(tc.url, func(t *testing.T) {
			query, err := parseQuery(httptest.NewRequest("GET", tc.url, nil))
			if err != nil {
				t.Fatal(err)
			}
			if query.Item != "kubernetes" || query.Version != "1.34" || query.Resource != resource || query.Path != tc.path || query.Pointer != tc.pointer {
				t.Fatalf("unexpected navigation query: %+v", query)
			}
		})
	}
}

func TestRejectInvalidSiteOrigin(t *testing.T) {
	s := testServer(t)
	for _, origin := range []string{"javascript:alert(1)", "https://user:pass@example.com", "https://example.com/?x=1", "https://example.com/path"} {
		config := s.config
		config.SiteURL = origin
		if _, err := New(fakeCatalog{}, config); err == nil {
			t.Fatalf("accepted invalid site URL %q", origin)
		}
	}
}
