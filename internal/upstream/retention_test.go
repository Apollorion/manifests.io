package upstream

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func addVersion(t *testing.T, root, product, version string) {
	t.Helper()
	target := "ETL/crds/" + product + "-" + version + "/crd.yaml"
	provenance := "ETL/crds/" + product + "-" + version + "/upstream.json"
	data := crd
	if product == "kubernetes" {
		target = "oaspec/kubernetes/" + version + ".json"
		provenance = "oaspec/kubernetes/" + version + ".source"
		data = openAPI
	}
	for name, body := range map[string]string{target: data, provenance: "original provenance " + version} {
		if err := writeNew(root, name, []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPlanRetentionCountsProductsAndGatewayTracksIndependently(t *testing.T) {
	root := corpus(t)
	for _, version := range []string{"1.7", "1.8", "1.10", "1.11", "1.12"} {
		addVersion(t, root, "kubernetes", version)
	}
	for _, version := range []string{"1.0.2", "1.0.9", "1.0.10", "1.1.0", "2.0.0"} {
		addVersion(t, root, "flux", version)
	}
	for minor := 0; minor < 7; minor++ {
		addVersion(t, root, "gatewayapi", fmt.Sprintf("1.%d.0", minor))
	}
	for minor := 0; minor < 6; minor++ {
		addVersion(t, root, "gatewayapi", fmt.Sprintf("1.%d.0experimental", minor))
	}
	before := snapshot(t, root)
	retired, err := PlanRetention(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	var targets []string
	for _, version := range retired {
		targets = append(targets, version.Target)
	}
	want := []string{"ETL/crds/flux-1.0.0", "ETL/crds/gatewayapi-1.0.0", "ETL/crds/gatewayapi-1.0.0experimental", "ETL/crds/gatewayapi-1.1.0", "oaspec/kubernetes/1.7.json"}
	if !reflect.DeepEqual(targets, want) {
		t.Fatalf("retired %v; want %v", targets, want)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("dry run changed corpus")
	}
	for _, version := range retired {
		for _, file := range version.Files {
			if len(file.SHA256) != 64 || before[file.Path] == "" {
				t.Fatalf("incomplete retirement checksum: %+v", file)
			}
		}
		if version.Product == "kubernetes" && len(version.Files) != 2 {
			t.Fatal("missing Kubernetes provenance retirement")
		}
	}
}

func TestApplyPrunesWithoutNewReleasesAndPreservesKeptFiles(t *testing.T) {
	root := corpus(t)
	for _, version := range []string{"1.4", "1.5", "1.6", "1.7", "1.8"} {
		addVersion(t, root, "kubernetes", version)
	}
	before := snapshot(t, root)
	retired, err := PlanRetention(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(retired) != 1 || retired[0].Version != "1.4" {
		t.Fatalf("wrong retention: %+v", retired)
	}
	if err := NewClient("").Apply(context.Background(), root, nil); err != nil {
		t.Fatal(err)
	}
	for _, version := range retired {
		for _, file := range version.Files {
			delete(before, file.Path)
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("retention changed kept schema or provenance bytes")
	}
	again, err := PlanRetention(root, nil)
	if err != nil || len(again) != 0 {
		t.Fatalf("retention not idempotent: %+v %v", again, err)
	}
}

func TestRetentionIncludesNewestAdditionBeforeCounting(t *testing.T) {
	root := corpus(t)
	for _, version := range []string{"1.5", "1.6", "1.7", "1.8"} {
		addVersion(t, root, "kubernetes", version)
	}
	client := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, openAPI) })
	update := Update{Product: "kubernetes", Version: "1.10", Target: "oaspec/kubernetes/1.10.json", Files: []File{{Path: "swagger.json", URL: client.RawBase}}}
	retired, err := PlanRetention(root, []Update{update})
	if err != nil || len(retired) != 1 || retired[0].Version != "1.5" {
		t.Fatalf("wrong plan: %+v %v", retired, err)
	}
	if err := client.ApplyPlan(context.Background(), root, Report{Updates: []Update{update}, Retired: retired}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, update.Target)); err != nil {
		t.Fatal("newest addition was not kept")
	}
	if _, err := os.Stat(filepath.Join(root, "oaspec/kubernetes/1.5.source")); !os.IsNotExist(err) {
		t.Fatal("old provenance not retired")
	}
	update.Version, update.Target = "1.1", "oaspec/kubernetes/1.1.json"
	if _, err := PlanRetention(root, []Update{update}); err == nil {
		t.Fatal("accepted stale addition that would immediately be retired")
	}
}

func TestRetentionRejectsMalformedVersionsAndSymlinks(t *testing.T) {
	for _, scenario := range []string{"invalid version", "symlink version", "symlink schema", "symlink provenance"} {
		t.Run(scenario, func(t *testing.T) {
			root := corpus(t)
			switch scenario {
			case "invalid version":
				if err := os.Mkdir(filepath.Join(root, "ETL/crds/flux-garbage"), 0755); err != nil {
					t.Fatal(err)
				}
			case "symlink version":
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "ETL/crds/flux-2.0.0")); err != nil {
					t.Fatal(err)
				}
			case "symlink schema":
				if err := os.Symlink("crd.yaml", filepath.Join(root, "ETL/crds/flux-1.0.0/linked.yaml")); err != nil {
					t.Fatal(err)
				}
			case "symlink provenance":
				if err := os.Symlink("1.9.json", filepath.Join(root, "oaspec/kubernetes/1.9.source")); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := PlanRetention(root, nil); err == nil {
				t.Fatal("accepted malformed corpus")
			}
		})
	}
}

func TestRetentionRejectsTamperedRetirementAndChangedFiles(t *testing.T) {
	for _, scenario := range []string{"changed file", "forged target", "missing file", "extra retirement"} {
		t.Run(scenario, func(t *testing.T) {
			root := corpus(t)
			for _, version := range []string{"1.4", "1.5", "1.6", "1.7", "1.8"} {
				addVersion(t, root, "kubernetes", version)
			}
			retired, err := PlanRetention(root, nil)
			if err != nil {
				t.Fatal(err)
			}
			switch scenario {
			case "changed file":
				if err := os.WriteFile(filepath.Join(root, "oaspec/kubernetes/1.4.source"), []byte("changed"), 0644); err != nil {
					t.Fatal(err)
				}
			case "forged target":
				retired[0].Target = "oaspec/kubernetes/../../outside"
			case "missing file":
				retired[0].Files = retired[0].Files[:1]
			case "extra retirement":
				retired = append(retired, Retirement{Product: "kubernetes", Version: "1.9", Target: "oaspec/kubernetes/1.9.json"})
			}
			before := snapshot(t, root)
			if err := NewClient("").ApplyPlan(context.Background(), root, Report{Retired: retired}); err == nil {
				t.Fatal("accepted stale or forged retirement")
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("changed corpus on invalid retirement")
			}
		})
	}
}

func TestRetentionRollsBackRetirementsAndAdditionsOnLateCollision(t *testing.T) {
	root := corpus(t)
	for _, version := range []string{"1.4", "1.5", "1.6", "1.7", "1.8"} {
		addVersion(t, root, "kubernetes", version)
	}
	before := snapshot(t, root)
	client := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/kubernetes" {
			if err := writeNew(root, "oaspec/kubernetes/1.10.source", []byte("concurrent owner")); err != nil {
				t.Error(err)
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
		t.Fatal("accepted late collision")
	}
	before["oaspec/kubernetes/1.10.source"] = "concurrent owner"
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("rollback failed to restore retirements or remove additions")
	}
}

type cancellationContext struct {
	context.Context
	check func() bool
}

func (c cancellationContext) Err() error {
	if c.check() {
		return context.Canceled
	}
	return nil
}

func TestRetentionRollsBackCancellationAfterMutation(t *testing.T) {
	for _, after := range []string{"retirement", "addition"} {
		t.Run(after, func(t *testing.T) {
			root := corpus(t)
			for _, version := range []string{"1.4", "1.5", "1.6", "1.7", "1.8"} {
				addVersion(t, root, "kubernetes", version)
			}
			before := snapshot(t, root)
			retired, err := PlanRetention(root, nil)
			if err != nil {
				t.Fatal(err)
			}
			stage := t.TempDir()
			if err := writeNew(stage, "ETL/crds/flux-2.0.0/crd.yaml", []byte(crd)); err != nil {
				t.Fatal(err)
			}
			ctx := cancellationContext{Context: context.Background(), check: func() bool {
				if after == "retirement" {
					_, err := os.Stat(filepath.Join(root, retired[0].Target))
					return os.IsNotExist(err)
				}
				_, err := os.Stat(filepath.Join(root, "ETL/crds/flux-2.0.0"))
				return err == nil
			}}
			keep, err := installPlan(ctx, root, stage, []string{"ETL/crds/flux-2.0.0"}, retired)
			if keep || !errors.Is(err, context.Canceled) {
				t.Fatalf("unexpected cancellation: %v %v", keep, err)
			}
			if !reflect.DeepEqual(before, snapshot(t, root)) {
				t.Fatal("cancellation did not restore original corpus")
			}
		})
	}
}

func TestRetentionManifestContainsEveryRemovedCRDFile(t *testing.T) {
	root := corpus(t)
	if err := writeNew(root, "ETL/crds/flux-1.0.0/nested/license.txt", []byte("license")); err != nil {
		t.Fatal(err)
	}
	for minor := 1; minor < 6; minor++ {
		addVersion(t, root, "flux", fmt.Sprintf("1.%d.0", minor))
	}
	retired, err := PlanRetention(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(retired) != 1 {
		t.Fatalf("wrong retirements: %+v", retired)
	}
	paths := []string{}
	for _, file := range retired[0].Files {
		paths = append(paths, file.Path)
	}
	if !slices.Contains(paths, "ETL/crds/flux-1.0.0/nested/license.txt") || !strings.HasSuffix(retired[0].Target, "flux-1.0.0") {
		t.Fatalf("incomplete removed file manifest: %+v", retired)
	}
}

func TestRetentionPreservesBackupWhenRollbackIsObstructed(t *testing.T) {
	root := corpus(t)
	for _, version := range []string{"1.4", "1.5", "1.6", "1.7", "1.8"} {
		addVersion(t, root, "kubernetes", version)
	}
	retired, err := PlanRetention(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	stage := t.TempDir()
	ctx := cancellationContext{Context: context.Background(), check: func() bool {
		if _, err := os.Stat(filepath.Join(root, retired[0].Target)); os.IsNotExist(err) {
			if err := writeNew(root, retired[0].Target, []byte("another writer")); err != nil {
				t.Error(err)
			}
			return true
		}
		return false
	}}
	keep, err := installPlan(ctx, root, stage, nil, retired)
	if !keep || err == nil || !strings.Contains(err.Error(), "rollback incomplete") {
		t.Fatalf("lost recovery indication: %v %v", keep, err)
	}
	data, err := os.ReadFile(filepath.Join(stage, "retired", retired[0].Target))
	if err != nil || string(data) != openAPI {
		t.Fatalf("original snapshot not preserved: %v", err)
	}
	data, err = os.ReadFile(filepath.Join(root, retired[0].Target))
	if err != nil || string(data) != "another writer" {
		t.Fatal("rollback overwrote concurrent file")
	}
}

func TestRetentionDetectsChangesDuringDownload(t *testing.T) {
	root := corpus(t)
	for _, version := range []string{"1.4", "1.5", "1.6", "1.7", "1.8"} {
		addVersion(t, root, "kubernetes", version)
	}
	before := snapshot(t, root)
	const target = "oaspec/kubernetes/1.4.source"
	client := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
		if err := os.WriteFile(filepath.Join(root, target), []byte("changed while downloading"), 0644); err != nil {
			t.Error(err)
		}
		_, _ = fmt.Fprint(w, crd)
	})
	update := Update{Product: "flux", Version: "2.0.0", Target: "ETL/crds/flux-2.0.0", Files: []File{{Path: "crd.yaml", URL: client.RawBase}}}
	if err := client.Apply(context.Background(), root, []Update{update}); err == nil {
		t.Fatal("accepted a changed retirement after staging")
	}
	before[target] = "changed while downloading"
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("mutated corpus after retirement hash changed")
	}
}
