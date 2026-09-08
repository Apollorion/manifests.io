package server

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/TheOutdoorProgrammer/manifests.io/internal/schema"
)

func sitemapLocations(t *testing.T, body []byte, root, entry string) []string {
	t.Helper()
	var document struct {
		XMLName xml.Name
		URLs    []struct {
			Location string `xml:"loc"`
		} `xml:"url"`
		Sitemaps []struct {
			Location string `xml:"loc"`
		} `xml:"sitemap"`
	}
	if err := xml.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	if document.XMLName.Local != root || document.XMLName.Space != sitemapNamespace {
		t.Fatalf("invalid sitemap root: %+v", document.XMLName)
	}
	var locations []string
	if entry == "url" {
		for _, item := range document.URLs {
			locations = append(locations, item.Location)
		}
	} else {
		for _, item := range document.Sitemaps {
			locations = append(locations, item.Location)
		}
	}
	return locations
}

func TestCrawlerDocumentsCoverOnlyFiniteCanonicalRoutes(t *testing.T) {
	catalog, err := schema.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	const site = "https://docs.example.test"
	routes := catalog.Routes()
	files, err := buildCrawlerFiles(site, routes)
	if err != nil {
		t.Fatal(err)
	}
	index := sitemapLocations(t, files["/sitemap.xml"].body, "sitemapindex", "sitemap")
	seen := make(map[string]bool)
	var locations []string
	for _, chunk := range index {
		if !strings.HasPrefix(chunk, site+"/sitemap-") {
			t.Fatalf("unexpected chunk URL %q", chunk)
		}
		body := files[strings.TrimPrefix(chunk, site)].body
		if len(body) > sitemapMaxBytes {
			t.Fatal("sitemap exceeds byte limit")
		}
		entries := sitemapLocations(t, body, "urlset", "url")
		if len(entries) > sitemapMaxEntries {
			t.Fatal("sitemap exceeds entry limit")
		}
		for _, location := range entries {
			u, err := url.Parse(location)
			if err != nil {
				t.Fatal(err)
			}
			if u.Scheme+"://"+u.Host != site || u.Fragment != "" || len(location) >= 2048 {
				t.Fatalf("invalid sitemap URL %q", location)
			}
			for key := range u.Query() {
				if key != "pointer" {
					t.Fatalf("traversal context leaked into sitemap: %s", location)
				}
			}
			if seen[location] {
				t.Fatalf("duplicate sitemap URL %s", location)
			}
			seen[location] = true
			locations = append(locations, location)
		}
	}
	if !slices.IsSorted(locations) {
		t.Fatal("sitemap URLs are not deterministic")
	}
	for _, route := range routes {
		page, err := catalog.Page(route)
		if err != nil {
			t.Fatal(err)
		}
		location := site + page.Canonical
		if len(location) < 2048 && !seen[location] {
			t.Fatalf("canonical page missing: %s", location)
		}
		delete(seen, location)
	}
	if len(seen) != 0 {
		t.Fatalf("sitemap includes %d noncanonical URLs", len(seen))
	}
	for _, expected := range []string{
		"/kubernetes/1.34/io.k8s.api.core.v1.ContainerStatus",
		"/kubernetes/1.34/io.k8s.apiextensions-apiserver.pkg.apis.apiextensions.v1.JSONSchemaProps",
		"/certmanager/1.14/io.cert-manager.v1.Certificate?pointer=%2Fproperties%2Fspec",
	} {
		if !slices.Contains(locations, site+expected) {
			t.Errorf("missing crawler destination %s", expected)
		}
	}
	t.Logf("sitemap covers %d canonical URLs in %d chunks", len(locations), len(index))
}

func TestSitemapSplitsAtProtocolLimits(t *testing.T) {
	locations := make([]string, sitemapMaxEntries+1)
	for i := range locations {
		locations[i] = fmt.Sprintf("https://example.test/schema/%d", i)
	}
	documents, err := sitemapDocuments("urlset", "url", locations, sitemapMaxEntries, sitemapMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(documents) != 2 {
		t.Fatalf("expected two chunks, got %d", len(documents))
	}
	if len(sitemapLocations(t, documents[0], "urlset", "url")) != sitemapMaxEntries || len(sitemapLocations(t, documents[1], "urlset", "url")) != 1 {
		t.Fatal("wrong entry boundary")
	}
	const escaped = "https://example.test/a?x=1&y=<tag>\"'"
	one, err := sitemapDocuments("urlset", "url", []string{escaped}, sitemapMaxEntries, sitemapMaxBytes)
	if err != nil {
		t.Fatal(err)
	}
	documents, err = sitemapDocuments("urlset", "url", []string{escaped, escaped}, sitemapMaxEntries, len(one[0]))
	if err != nil || len(documents) != 2 {
		t.Fatalf("byte boundary did not split: %v", err)
	}
	for _, document := range documents {
		if len(document) != len(one[0]) || sitemapLocations(t, document, "urlset", "url")[0] != escaped {
			t.Fatal("size or XML entity escaping changed")
		}
	}
	if _, err := sitemapDocuments("urlset", "url", []string{escaped}, sitemapMaxEntries, len(one[0])-1); err == nil {
		t.Fatal("oversized entry accepted")
	}
}

func TestSitemapDeduplicatesAndDropsTraversalContext(t *testing.T) {
	route := schema.Query{Item: "gateway api", Version: "1.2.0 standard", Resource: "example.v1.Thing", Pointer: "/properties/a&b"}
	contextual := route
	contextual.Path, contextual.Trail = "Thing.spec", "history"
	files, err := buildCrawlerFiles("https://example.test", []schema.Query{route, contextual})
	if err != nil {
		t.Fatal(err)
	}
	locations := sitemapLocations(t, files["/sitemap-1.xml"].body, "urlset", "url")
	if len(locations) != 1 || locations[0] != "https://example.test"+schema.Href(route) {
		t.Fatalf("unexpected canonical locations: %v", locations)
	}
}

func TestCrawlerHTTPContract(t *testing.T) {
	s := testServer(t)
	for _, path := range []string{"/robots.txt", "/sitemap.xml", "/sitemap-1.xml"} {
		var etag string
		for _, method := range []string{"GET", "HEAD"} {
			r := httptest.NewRequest(method, "https://host-injection.invalid"+path, nil)
			r.Header.Set("X-Forwarded-Host", "forwarded-injection.invalid")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != 200 {
				t.Fatalf("%s %s: status %d", method, path, w.Code)
			}
			if w.Header().Get("X-Content-Type-Options") != "nosniff" || w.Header().Get("Content-Length") == "" || w.Header().Get("Cache-Control") != "public, max-age=0, must-revalidate" {
				t.Fatalf("crawler headers missing: %v", w.Header())
			}
			if strings.Contains(w.Body.String(), "injection.invalid") {
				t.Fatal("request host leaked into crawler documents")
			}
			if method == "GET" {
				etag = w.Header().Get("ETag")
				if etag == "" || !strings.Contains(w.Body.String(), "https://www.manifests.io") {
					t.Fatal("configured site or ETag missing")
				}
				if path == "/robots.txt" && w.Body.String() != "User-agent: *\nDisallow:\n\nSitemap: https://www.manifests.io/sitemap.xml\n" {
					t.Fatalf("unexpected robots policy: %s", w.Body)
				}
			} else if w.Body.Len() != 0 || w.Header().Get("ETag") != etag {
				t.Fatal("HEAD differs from GET")
			}
			if path == "/sitemap-1.xml" && r.Pattern != "/sitemap-{chunk}.xml" {
				t.Fatalf("unbounded crawler route %s", r.Pattern)
			}
		}
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("If-None-Match", etag)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != http.StatusNotModified || w.Body.Len() != 0 {
			t.Fatal("crawler conditional request failed")
		}
	}
	for _, path := range []string{"/sitemap-0.xml", "/sitemap-2.xml", "/sitemap-01.xml", "/sitemap-99999999999999999999999999.xml", "/sitemap-../robots.xml"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", path, nil)
		s.ServeHTTP(w, r)
		if w.Code != 404 || r.Pattern != "/sitemap-{chunk}.xml" {
			t.Fatalf("invalid sitemap path %s: %d, %s", path, w.Code, r.Pattern)
		}
	}
}
