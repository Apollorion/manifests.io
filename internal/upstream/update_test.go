package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const openAPI = `{"swagger":"2.0","info":{"title":"Fixture","version":"1"},"paths":{},"definitions":{"Pod":{"type":"object","properties":{"name":{"type":"string"}}}}}`
const crd = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  names:
    kind: Widget
  versions:
    - name: v1
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                name:
                  type: string
`

func corpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for file, content := range map[string]string{"oaspec/kubernetes/1.9.json": openAPI, "ETL/crds/flux-1.0.0/crd.yaml": crd} {
		if err := writeNew(root, file, []byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		name, _ := filepath.Rel(root, path)
		files[name] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func fixtureClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &Client{HTTP: server.Client(), APIBase: server.URL, RawBase: server.URL}
}

func TestDiscoverApplyPreservesVersionsAndRecordsProvenance(t *testing.T) {
	root := corpus(t)
	before := snapshot(t, root)
	sha := strings.Repeat("a", 40)
	requests := 0
	client := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/repos/example/operator/releases/latest":
			_, _ = fmt.Fprint(w, `{"tag_name":"v1.10.0"}`)
		case "/repos/example/operator/commits/v1.10.0":
			_, _ = fmt.Fprintf(w, `{"sha":%q}`, sha)
		case "/example/operator/" + sha + "/crd.yaml":
			_, _ = fmt.Fprint(w, crd)
		default:
			t.Errorf("unexpected request %s", r.URL)
			w.WriteHeader(404)
		}
	})
	sources := []Source{{Product: "flux", Repository: "example/operator", Files: []string{"crd.yaml"}}}
	updates, err := client.Discover(context.Background(), root, sources)
	if err != nil || len(updates) != 1 || updates[0].Commit != sha {
		t.Fatalf("discover: %+v, %v", updates, err)
	}
	if got := snapshot(t, root); !reflect.DeepEqual(got, before) || requests != 2 {
		t.Fatal("discovery changed corpus or downloaded schemas")
	}
	if err := client.Apply(context.Background(), root, updates); err != nil {
		t.Fatal(err)
	}
	after := snapshot(t, root)
	for path, data := range before {
		if after[path] != data {
			t.Fatalf("old version changed: %s", path)
		}
	}
	var record Update
	if err := json.Unmarshal([]byte(after["ETL/crds/flux-1.10.0/upstream.json"]), &record); err != nil || len(record.Files[0].SHA256) != 64 || record.Commit != sha {
		t.Fatalf("provenance: %+v, %v", record, err)
	}
	updates, err = client.Discover(context.Background(), root, sources)
	if err != nil || len(updates) != 0 {
		t.Fatalf("second run is not idempotent: %+v, %v", updates, err)
	}
}

func TestApplyValidatesEntireBatchBeforeWriting(t *testing.T) {
	for _, failure := range []string{"malformed JSON", "missing schemas", "invalid CRD", "non-CRD YAML", "HTTP error", "existing invalid corpus"} {
		t.Run(failure, func(t *testing.T) {
			root := corpus(t)
			if failure == "existing invalid corpus" {
				if err := writeNew(root, "oaspec/kubernetes/1.8.json", []byte(`{}`)); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshot(t, root)
			client := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/valid" {
					_, _ = fmt.Fprint(w, crd)
					return
				}
				switch failure {
				case "malformed JSON":
					_, _ = fmt.Fprint(w, `{broken`)
				case "missing schemas":
					_, _ = fmt.Fprint(w, `{}`)
				case "invalid CRD":
					_, _ = fmt.Fprint(w, "kind: CustomResourceDefinition\nspec: {}")
				case "non-CRD YAML":
					_, _ = fmt.Fprint(w, "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: unexpected")
				case "HTTP error":
					w.WriteHeader(http.StatusBadGateway)
				default:
					_, _ = fmt.Fprint(w, openAPI)
				}
			})
			updates := []Update{{Product: "flux", Version: "2.0.0", Target: "ETL/crds/flux-2.0.0", Files: []File{{Path: "crd.yaml", URL: client.RawBase + "/valid"}}}, {Product: "kubernetes", Version: "1.10", Target: "oaspec/kubernetes/1.10.json", Files: []File{{Path: "swagger.json", URL: client.RawBase + "/bad"}}}}
			if failure == "invalid CRD" || failure == "non-CRD YAML" {
				updates[1] = Update{Product: "flux", Version: "3.0.0", Target: "ETL/crds/flux-3.0.0", Files: []File{{Path: "crd.yaml", URL: client.RawBase + "/bad"}}}
			}
			if err := client.Apply(context.Background(), root, updates); err == nil {
				t.Fatal("invalid update succeeded")
			}
			if got := snapshot(t, root); !reflect.DeepEqual(got, before) {
				t.Fatal("failed batch changed corpus")
			}
			entries, _ := os.ReadDir(root)
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), ".schema-update-") {
					t.Fatal("staging not cleaned")
				}
			}
		})
	}
}

func TestReleaseAndSourceValidation(t *testing.T) {
	for _, tag := range []string{"v1.2.0-rc.1", "v1.2.0+build", "1.02.0", "v1.2", "../main", "main", "v1.2.3/../../x"} {
		if _, err := stableVersion(tag); err == nil {
			t.Errorf("accepted %q", tag)
		}
	}
	for _, tag := range []string{"1.2.3", "v0.0.42", "v1.37.0"} {
		if _, err := stableVersion(tag); err != nil {
			t.Errorf("rejected %q: %v", tag, err)
		}
	}
	for _, source := range []Source{
		{Product: "../flux", Repository: "example/operator", Files: []string{"crd.yaml"}},
		{Product: "flux", Repository: "example/operator", Files: []string{"../crd.yaml"}},
		{Product: "flux", Repository: "example/operator", Directory: "/root"},
		{Product: "flux", Repository: "example/operator", Asset: "../../crd.yaml"},
		{Product: "flux", Repository: "example/operator"},
		{Product: "flux", Repository: "example/operator", Asset: "crd.yaml", Files: []string{"crd.yaml"}},
	} {
		if err := validateSource(source); err == nil {
			t.Errorf("accepted %+v", source)
		}
	}
}

func TestVersionComparisonAndGatewayTracks(t *testing.T) {
	root := corpus(t)
	for _, dir := range []string{"ETL/crds/gatewayapi-1.10.0", "ETL/crds/gatewayapi-1.9.0experimental"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		source  Source
		version string
		newer   bool
	}{
		{Source{Product: "kubernetes"}, "1.10", true},
		{Source{Product: "kubernetes"}, "1.8", false},
		{Source{Product: "kubernetes"}, "1.9", false},
		{Source{Product: "gatewayapi"}, "1.9.1", false},
		{Source{Product: "gatewayapi", Suffix: "experimental"}, "1.10.0experimental", true},
	} {
		got, err := isNewer(root, tc.source, tc.version)
		if err != nil || got != tc.newer {
			t.Errorf("%+v: %v, %v", tc, got, err)
		}
	}
}

func TestDiscoveryRejectsIncompleteOrUnsafeSources(t *testing.T) {
	for _, problem := range []string{"missing asset", "wrong asset host", "draft", "prerelease", "truncated tree", "symlink", "empty tree"} {
		t.Run(problem, func(t *testing.T) {
			client := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/latest"):
					draft, pre := problem == "draft", problem == "prerelease"
					_, _ = fmt.Fprintf(w, `{"tag_name":"v2.0.0","draft":%t,"prerelease":%t,"assets":[{"name":"crd.yaml","browser_download_url":"https://attacker.invalid/crd.yaml"}]}`, draft, pre)
				case strings.Contains(r.URL.Path, "/commits/"):
					_, _ = fmt.Fprintf(w, `{"sha":%q}`, strings.Repeat("a", 40))
				default:
					switch problem {
					case "truncated tree":
						_, _ = fmt.Fprint(w, `{"truncated":true}`)
					case "symlink":
						_, _ = fmt.Fprint(w, `{"tree":[{"path":"crds/escape.yaml","type":"blob","mode":"120000"}]}`)
					default:
						_, _ = fmt.Fprint(w, `{"tree":[]}`)
					}
				}
			})
			source := Source{Product: "flux", Repository: "example/operator", Directory: "crds"}
			if problem == "missing asset" {
				source.Directory = ""
				source.Asset = "missing.yaml"
			}
			if problem == "wrong asset host" {
				source.Directory = ""
				source.Asset = "crd.yaml"
			}
			if _, err := client.Discover(context.Background(), corpus(t), []Source{source}); err == nil {
				t.Fatal("accepted invalid upstream metadata")
			}
		})
	}
}

func TestHTTPBoundsCancellationAndCredentialScoping(t *testing.T) {
	client := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			<-r.Context().Done()
			return
		}
		_, _ = fmt.Fprint(w, strings.Repeat("x", 33))
	})
	if _, err := client.get(context.Background(), client.RawBase, 32); err == nil {
		t.Fatal("accepted oversized body")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := client.get(ctx, client.RawBase+"/slow", 100); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation: %v", err)
	}
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("token leaked to asset host")
		}
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer other.Close()
	client.Token = "fixture-token"
	if _, err := client.get(context.Background(), other.URL, 100); err != nil {
		t.Fatal(err)
	}
}

func TestApplyCannotOverwriteOrEscape(t *testing.T) {
	for _, target := range []string{"ETL/crds/flux-1.0.0", "ETL/crds/../../outside", "/tmp/outside", "oaspec/kubernetes/1.9.json"} {
		root := corpus(t)
		before := snapshot(t, root)
		client := NewClient("")
		if err := client.Apply(context.Background(), root, []Update{{Product: "flux", Version: "1.0.0", Target: target, Files: []File{{Path: "crd.yaml", URL: "https://example.invalid"}}}}); err == nil {
			t.Errorf("accepted %s", target)
		}
		if !reflect.DeepEqual(before, snapshot(t, root)) {
			t.Fatal("changed existing files")
		}
	}
}

func TestRegistryCoversEveryProductAndGatewayTrack(t *testing.T) {
	sources, err := Sources()
	if err != nil {
		t.Fatal(err)
	}
	products := map[string]bool{}
	for _, source := range sources {
		if err := validateSource(source); err != nil {
			t.Fatal(err)
		}
		products[source.Product+source.Suffix] = true
	}
	entries, err := os.ReadDir("../../ETL/crds")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		product, version, _ := strings.Cut(entry.Name(), "-")
		if strings.HasSuffix(version, "experimental") {
			product += "experimental"
		}
		if !products[product] {
			t.Errorf("missing source for %s", entry.Name())
		}
	}
	if !products["kubernetes"] || len(products) != 13 {
		t.Fatalf("unexpected registry: %v", products)
	}
}

func TestApplyRejectsSymlinkCorpusParents(t *testing.T) {
	for _, component := range []string{"ETL", "ETL/crds", "oaspec", "oaspec/kubernetes"} {
		t.Run(component, func(t *testing.T) {
			root := corpus(t)
			outside := filepath.Join(t.TempDir(), "outside")
			if err := os.Rename(filepath.Join(root, component), outside); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(root, component)); err != nil {
				t.Fatal(err)
			}
			before := snapshot(t, outside)
			client := NewClient("")
			update := Update{Product: "flux", Version: "2.0.0", Target: "ETL/crds/flux-2.0.0", Files: []File{{Path: "crd.yaml", URL: "https://example.invalid"}}}
			if err := client.Apply(context.Background(), root, []Update{update}); err == nil {
				t.Fatal("accepted symlink parent")
			}
			if !reflect.DeepEqual(before, snapshot(t, outside)) {
				t.Fatal("modified outside corpus")
			}
		})
	}
}

func TestApplyRejectsDuplicateDownloads(t *testing.T) {
	root := corpus(t)
	before := snapshot(t, root)
	client := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, crd) })
	file := File{Path: "crd.yaml", URL: client.RawBase}
	update := Update{Product: "flux", Version: "2.0.0", Target: "ETL/crds/flux-2.0.0", Files: []File{file, file}}
	if err := client.Apply(context.Background(), root, []Update{update}); err == nil {
		t.Fatal("accepted duplicate filenames")
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("changed corpus")
	}
}

func TestApplyRollsBackInstalledVersionsOnLateCollision(t *testing.T) {
	root := corpus(t)
	before := snapshot(t, root)
	const marker = "another writer owns this provenance"
	client := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kubernetes" {
			if err := writeNew(root, "oaspec/kubernetes/1.10.source", []byte(marker)); err != nil {
				t.Error(err)
				w.WriteHeader(500)
				return
			}
			_, _ = fmt.Fprint(w, openAPI)
		} else {
			_, _ = fmt.Fprint(w, crd)
		}
	})
	updates := []Update{
		{Product: "flux", Version: "2.0.0", Target: "ETL/crds/flux-2.0.0", Files: []File{{Path: "crd.yaml", URL: client.RawBase + "/flux"}}},
		{Product: "kubernetes", Version: "1.10", Target: "oaspec/kubernetes/1.10.json", Files: []File{{Path: "swagger.json", URL: client.RawBase + "/kubernetes"}}},
	}
	if err := client.Apply(context.Background(), root, updates); err == nil {
		t.Fatal("accepted concurrent destination collision")
	}
	before["oaspec/kubernetes/1.10.source"] = marker
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("rollback left new snapshots or removed another writer's file")
	}
}

func TestChartAppVersionUsesSameImmutableCommit(t *testing.T) {
	sha := strings.Repeat("b", 40)
	client := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/example/charts/releases/latest":
			_, _ = fmt.Fprint(w, `{"tag_name":"v0.79.0"}`)
		case "/repos/example/charts/commits/v0.79.0":
			_, _ = fmt.Fprintf(w, `{"sha":%q}`, sha)
		case "/example/charts/" + sha + "/workerpool/Chart.yaml":
			_, _ = fmt.Fprint(w, "version: 0.79.0\nappVersion: v0.0.42\n")
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	source := Source{Product: "spaceliftworkerpool", Repository: "example/charts", VersionFile: "workerpool/Chart.yaml", Files: []string{"workerpool/crd.yaml"}}
	updates, err := client.Discover(context.Background(), corpus(t), []Source{source})
	if err != nil || len(updates) != 1 {
		t.Fatalf("%+v, %v", updates, err)
	}
	update := updates[0]
	if update.Version != "0.0.42" || update.Tag != "v0.79.0" || update.Commit != sha || !strings.Contains(update.Files[0].URL, "/"+sha+"/") {
		t.Fatalf("wrong version provenance: %+v", update)
	}
}

func TestRedirectsStripTokenAndRejectDowngrades(t *testing.T) {
	asset := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("redirect leaked token")
		}
		_, _ = fmt.Fprint(w, "schema")
	}))
	defer asset.Close()
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-token" {
			t.Error("API did not receive token")
		}
		target := asset.URL
		if r.URL.Path == "/downgrade" {
			target = "http://example.invalid"
		}
		http.Redirect(w, r, target, http.StatusFound)
	}))
	defer api.Close()
	client := NewClient("fixture-token")
	client.APIBase = api.URL
	client.HTTP.Transport = api.Client().Transport
	if _, err := client.get(context.Background(), api.URL, 100); err != nil {
		t.Fatal(err)
	}
	if _, err := client.get(context.Background(), api.URL+"/downgrade", 100); err == nil {
		t.Fatal("allowed TLS downgrade")
	}
}
