package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
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
		"/healthz", "/readyz", "/missing", "/kubernetes/1.34/missing",
		"/api/page?item=missing&version=1", "/api/page?item=kubernetes&item=flux&version=1.34",
		"/assets/private.js.map", "/sitemap-missing.xml",
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
