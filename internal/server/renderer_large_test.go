package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/TheOutdoorProgrammer/manifests.io/internal/schema"
)

func prometheusRenderFixture(t testing.TB) (*schema.Catalog, *Server) {
	t.Helper()
	const bundle = "../../frontend/dist-render/renderer.js"
	if _, err := os.Stat(bundle); os.IsNotExist(err) {
		t.Skip("run npm --prefix frontend run build for React integration")
	}
	catalog, err := schema.Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(catalog, Config{WebDir: "../../frontend/dist", RenderDir: t.TempDir(), PublicDir: "../../public", RendererFile: bundle})
	if err != nil {
		t.Fatal(err)
	}
	return catalog, s
}

func prometheusSpecQuery(version string) schema.Query {
	return schema.Query{Item: "prometheus operator", Version: version, Resource: "com.coreos.monitoring.v1.Prometheus", Path: "Prometheus.spec.scrapeClasses.tlsConfig.cert.secret.spec.initContainers.lifecycle.preStop.exec.spec", Pointer: "/properties/spec"}
}

func prometheusVersions(t testing.TB, catalog *schema.Catalog) []string {
	t.Helper()
	for _, product := range catalog.Products() {
		if product.Name == "prometheus operator" && len(product.Versions) > 0 {
			return product.Versions
		}
	}
	t.Fatal("Prometheus schemas missing from corpus")
	return nil
}

func TestLargeContextualRenderer(t *testing.T) {
	catalog, s := prometheusRenderFixture(t)
	for _, version := range prometheusVersions(t, catalog) {
		t.Run(version, func(t *testing.T) {
			q := prometheusSpecQuery(version)
			page, err := catalog.Page(q)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(page)
			if err != nil {
				t.Fatal(err)
			}
			expected := nodeRenderedPage(t, data)
			for range 2 {
				var before, after runtime.MemStats
				runtime.ReadMemStats(&before)
				html, err := s.renderer.render(t.Context(), data)
				runtime.ReadMemStats(&after)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(html, expected) {
					t.Fatal("large contextual output differs from the unmodified Node renderer")
				}
				allocated := after.TotalAlloc - before.TotalAlloc
				t.Logf("rows=%d html=%d allocated=%d", len(page.Resources), len(html), allocated)
				// Leave GC headroom while rejecting the original 600+ MiB copying cost.
				if allocated > 64<<20 {
					t.Errorf("render allocated %d bytes, budget is 64 MiB", allocated)
				}
			}
			response := httptest.NewRecorder()
			s.ServeHTTP(response, httptest.NewRequest(http.MethodGet, schema.Href(q), nil))
			q.Path, q.Trail = "", ""
			canonical, err := catalog.Page(q)
			if err != nil {
				t.Fatal(err)
			}
			canonicalData, err := json.Marshal(canonical)
			if err != nil {
				t.Fatal(err)
			}
			expected = nodeRenderedPage(t, canonicalData)
			if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), expected) {
				t.Fatalf("shared schema HTTP response is incomplete: status=%d", response.Code)
			}
		})
	}
}

// Race instrumentation distorts this production-deadline benchmark.
func BenchmarkLargeContextualBurst(b *testing.B) {
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)
	catalog, s := prometheusRenderFixture(b)
	versions := prometheusVersions(b, catalog)
	q := prometheusSpecQuery(versions[len(versions)-1])
	canonical := q
	canonical.Path, canonical.Trail = "", ""
	page, err := catalog.Page(canonical)
	if err != nil {
		b.Fatal(err)
	}
	data, err := json.Marshal(page)
	if err != nil {
		b.Fatal(err)
	}
	expected, err := s.renderer.render(b.Context(), data)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for range b.N {
		const requests = 40
		responses := make([]*httptest.ResponseRecorder, requests)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := range requests {
			wg.Go(func() {
				<-start
				response := httptest.NewRecorder()
				s.ServeHTTP(response, httptest.NewRequest(http.MethodGet, schema.Href(q), nil))
				responses[i] = response
			})
		}
		close(start)
		wg.Wait()
		failures := 0
		for _, response := range responses {
			if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), expected) {
				failures++
			}
		}
		if failures != 0 {
			b.Fatalf("%d of %d contextual requests failed or returned incomplete HTML", failures, requests)
		}
	}
}
