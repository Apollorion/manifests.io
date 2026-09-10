package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/TheOutdoorProgrammer/manifests.io/internal/schema"
)

func TestAssetCachePolicy(t *testing.T) {
	s := testServer(t)
	if err := os.Mkdir(filepath.Join(s.config.WebDir, "assets"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ path, policy string }{
		{"/favicon.svg", "public, max-age=0, s-maxage=604800, must-revalidate"},
		{"/assets/index-abcd1234.js", "public, max-age=31536000, immutable"},
	} {
		if err := os.WriteFile(filepath.Join(s.config.WebDir, tc.path), []byte("asset"), 0600); err != nil {
			t.Fatal(err)
		}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != tc.policy {
			t.Fatalf("%s: status=%d cache=%q", tc.path, w.Code, w.Header().Get("Cache-Control"))
		}
	}
}

func TestSharedCachePolicy(t *testing.T) {
	s := testServer(t)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, target := range []string{
			"/", "/kubernetes/1.34", "/kubernetes/1.34/Pod?path=Deployment.spec&trail=Pod:1",
			"/api/catalog", "/api/page?item=kubernetes&version=1.34", "/api/definitions?item=kubernetes&version=1.34",
			"/robots.txt", "/sitemap.xml", "/sitemap-1.xml",
		} {
			t.Run(method+target, func(t *testing.T) {
				w := httptest.NewRecorder()
				s.ServeHTTP(w, httptest.NewRequest(method, target, nil))
				if got := w.Header().Get("Cache-Control"); got != "public, max-age=0, s-maxage=604800, must-revalidate" {
					t.Fatalf("cache policy=%q, status=%d", got, w.Code)
				}
				if etag := w.Header().Get("ETag"); etag != "" {
					r := httptest.NewRequest(method, target, nil)
					r.Header.Set("If-None-Match", etag)
					conditional := httptest.NewRecorder()
					s.ServeHTTP(conditional, r)
					if conditional.Code != http.StatusNotModified || conditional.Header().Get("Cache-Control") != w.Header().Get("Cache-Control") {
						t.Fatal("conditional response lost shared cache policy")
					}
				}
			})
		}
	}
}

func TestHealthAndFailuresAreNotCached(t *testing.T) {
	s := testServer(t)
	for _, target := range []string{
		"/healthz", "/readyz", "/missing", "/kubernetes/1.34/Pod/extra",
		"/api/page?item=kubernetes&item=flux&version=1.34", "/api/missing",
		"/assets/private.js.map", "/assets/missing.js", "/favicon-missing.svg", "/sitemap-missing.xml",
	} {
		t.Run(target, func(t *testing.T) {
			w := httptest.NewRecorder()
			s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
			if got := w.Header().Get("Cache-Control"); got != "no-store" {
				t.Fatalf("health or failure cache policy=%q, status=%d", got, w.Code)
			}
		})
	}
}

func TestMissingSchemasAreCached(t *testing.T) {
	s := testServer(t)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, target := range []string{
			"/kubernetes/1.34/missing", "/kubernetes/missing", "/missing/1",
			"/api/page?item=missing&version=1", "/api/page?item=kubernetes&version=1.34&resource=missing",
			"/api/definitions?item=missing&version=1",
		} {
			t.Run(method+target, func(t *testing.T) {
				w := httptest.NewRecorder()
				s.ServeHTTP(w, httptest.NewRequest(method, target, nil))
				if w.Code != http.StatusNotFound || w.Header().Get("Cache-Control") != "public, max-age=0, s-maxage=604800, must-revalidate" {
					t.Fatalf("status=%d cache=%q", w.Code, w.Header().Get("Cache-Control"))
				}
				if method == http.MethodHead && w.Body.Len() != 0 {
					t.Fatal("HEAD returned an error body")
				}
			})
		}
	}
}

func TestSchemaSelectorCachePolicy(t *testing.T) {
	s := testServer(t)
	s.catalog = corpusCatalog(t)
	version := schema.DefaultQuery(s.catalog.Products()).Version
	for _, base := range []string{
		"/kubernetes/" + version + "/io.k8s.api.core.v1.Pod?",
		"/api/page?item=kubernetes&version=" + version + "&resource=io.k8s.api.core.v1.Pod&",
	} {
		for _, tc := range []struct {
			pointer string
			status  int
			policy  string
		}{
			{"/properties/not-a-field", http.StatusNotFound, "public, max-age=0, s-maxage=604800, must-revalidate"},
			{"/properties", http.StatusBadRequest, "no-store"},
			{"/properties/bad~2escape", http.StatusBadRequest, "no-store"},
		} {
			w := httptest.NewRecorder()
			s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, base+"pointer="+url.QueryEscape(tc.pointer), nil))
			if w.Code != tc.status || w.Header().Get("Cache-Control") != tc.policy {
				t.Fatalf("%s%s: status=%d cache=%q", base, tc.pointer, w.Code, w.Header().Get("Cache-Control"))
			}
		}
	}
}

type unavailableCatalog struct{ fakeCatalog }

func (unavailableCatalog) Page(schema.Query) (schema.Page, error) {
	return schema.Page{}, errors.New("catalog unavailable")
}

func TestTransientFailuresDoNotInheritSchemaCachePolicy(t *testing.T) {
	for _, target := range []string{"/kubernetes/1.34/Pod", "/api/page?item=kubernetes&version=1.34&resource=Pod"} {
		s := testServer(t)
		s.catalog = unavailableCatalog{}
		w := httptest.NewRecorder()
		s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
		if w.Code != http.StatusInternalServerError || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("transient catalog failure: status=%d cache=%q", w.Code, w.Header().Get("Cache-Control"))
		}
	}
	s := testServer(t)
	file := filepath.Join(t.TempDir(), "renderer.js")
	if err := os.WriteFile(file, []byte(`var ManifestsRenderer = {renderPage: function() { throw new Error("render failed"); }};`), 0600); err != nil {
		t.Fatal(err)
	}
	var err error
	s.renderer, err = newReactRenderer(file)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/kubernetes/1.34/missing", nil))
	if w.Code != http.StatusServiceUnavailable || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("missing schema render failure: status=%d cache=%q", w.Code, w.Header().Get("Cache-Control"))
	}
}
