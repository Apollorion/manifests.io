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
	"testing"

	"github.com/TheOutdoorProgrammer/manifests.io/internal/schema"
)

type fakeCatalog struct{}

func (fakeCatalog) Products() []schema.Product {
	return []schema.Product{{Name: "kubernetes", Versions: []string{"1.34"}}}
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
	s, err := New(fakeCatalog{}, Config{WebDir: dir, RenderDir: dir, PublicDir: dir})
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

func TestPageDataCannotEscapeScript(t *testing.T) {
	w := httptest.NewRecorder()
	testServer(t).ServeHTTP(w, httptest.NewRequest("GET", "/kubernetes/1.34", nil))
	body := w.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") || !strings.Contains(body, `\u003c/script\u003e`) {
		t.Fatalf("unsafe serialization: %s", body)
	}
	if !strings.Contains(body, `data-dynamic="true"`) || !strings.Contains(body, `rel="canonical"`) || strings.Contains(body, "<!--page-") {
		t.Fatal("incomplete template")
	}
}

func TestPrerenderCachingAndDynamicQueries(t *testing.T) {
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
	r := httptest.NewRequest("GET", "/kubernetes/1.34", nil)
	r.Header.Set("If-None-Match", w.Header().Get("ETag"))
	cached := httptest.NewRecorder()
	s.ServeHTTP(cached, r)
	if cached.Code != http.StatusNotModified || cached.Body.Len() != 0 {
		t.Fatal("conditional request did not return 304")
	}
	dynamic := httptest.NewRecorder()
	s.ServeHTTP(dynamic, httptest.NewRequest("GET", "/kubernetes/1.34?path=/properties/spec", nil))
	if !strings.Contains(dynamic.Body.String(), "Server rendered fields") || !strings.Contains(dynamic.Body.String(), "data-dynamic") {
		t.Fatal("contextual query lost its selected schema HTML or fresh page data")
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

type canonicalCatalog struct {
	fakeCatalog
	page schema.Page
}

func (c canonicalCatalog) Page(schema.Query) (schema.Page, error) {
	return c.page, nil
}

func TestNamedReferencesRedirectToTheirCanonicalResource(t *testing.T) {
	const deployment = "/kubernetes/1.34/io.k8s.api.apps.v1.Deployment"
	const podSpec = "/kubernetes/1.34/io.k8s.api.core.v1.PodSpec"
	const context = "?path=/properties/spec/properties/template/properties/spec"
	for _, tc := range []struct {
		name      string
		method    string
		url       string
		canonical string
		status    int
	}{
		{"nested reference", "GET", deployment + context, podSpec, http.StatusPermanentRedirect},
		{"nested reference head", "HEAD", deployment + context, podSpec, http.StatusPermanentRedirect},
		{"legacy linked reference", "GET", deployment + context + "&linked=Workload", podSpec, http.StatusPermanentRedirect},
		{"canonical resource", "GET", podSpec, podSpec, http.StatusOK},
		{"canonical linked resource", "GET", podSpec + "?linked=Workload", podSpec, http.StatusOK},
		{"inline schema", "GET", deployment + "?path=/properties/status", deployment + "?path=%2Fproperties%2Fstatus", http.StatusOK},
		{"resource listing", "GET", "/kubernetes/1.34", podSpec, http.StatusOK},
		{"API reference", "GET", "/api/page?item=kubernetes&version=1.34&resource=io.k8s.api.apps.v1.Deployment&path=/properties/spec/properties/template/properties/spec", podSpec, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := testServer(t)
			s.catalog = canonicalCatalog{page: schema.Page{Canonical: tc.canonical, Title: "PodSpec"}}
			w := httptest.NewRecorder()
			s.ServeHTTP(w, httptest.NewRequest(tc.method, tc.url, nil))
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.status, w.Body)
			}
			if tc.status == http.StatusPermanentRedirect {
				if location := w.Header().Get("Location"); location != tc.canonical {
					t.Fatalf("location=%q want=%q", location, tc.canonical)
				}
			} else if w.Header().Get("Location") != "" {
				t.Fatal("non-redirect response includes Location")
			}
			if tc.method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD redirect returned a body")
			}
			if strings.HasPrefix(tc.url, "/api/") {
				var page schema.Page
				if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil || page.Canonical != podSpec {
					t.Fatalf("API did not return canonical page JSON: %s", w.Body)
				}
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
