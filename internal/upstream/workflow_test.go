package upstream

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func workflowRestoreScript(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../.github/workflows/update-schemas.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct{ Steps []struct{ Name, Run string } }
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	var script string
	for _, step := range workflow.Jobs["pull-request"].Steps {
		if step.Name == "Restore verified snapshots without overwriting existing versions" {
			script = strings.TrimSuffix(strings.TrimPrefix(step.Run, "python3 - <<'PY'\n"), "PY\n")
		}
	}
	if script == "" {
		t.Fatal("workflow restore script missing")
	}
	return script
}

func TestWorkflowArtifactRestoreRejectsUnsafeArchivesBeforeExtraction(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal("python3 is required to verify the schema-update workflow")
	}
	script := workflowRestoreScript(t)
	for _, scenario := range []string{"valid", "traversal", "absolute", "code path", "symlink", "hardlink", "duplicate", "existing file", "symlink parent"} {
		t.Run(scenario, func(t *testing.T) {
			root, artifact := t.TempDir(), t.TempDir()
			if err := os.WriteFile(filepath.Join(artifact, "schema-updates.json"), []byte(`{"updates":[],"retired":[]}`), 0644); err != nil {
				t.Fatal(err)
			}
			var archive bytes.Buffer
			compressed := gzip.NewWriter(&archive)
			writer := tar.NewWriter(compressed)
			add := func(name string, kind byte) {
				header := &tar.Header{Name: name, Mode: 0644, Typeflag: kind}
				if kind == tar.TypeReg {
					header.Size = 3
				} else {
					header.Linkname = "/tmp/outside"
				}
				if err := writer.WriteHeader(header); err != nil {
					t.Fatal(err)
				}
				if kind == tar.TypeReg {
					if _, err := writer.Write([]byte("new")); err != nil {
						t.Fatal(err)
					}
				}
			}
			add("oaspec/kubernetes/1.10.json", tar.TypeReg)
			second := "ETL/crds/flux-2.0.0/crd.yaml"
			kind := byte(tar.TypeReg)
			switch scenario {
			case "traversal":
				second = "ETL/crds/../../escape.yaml"
			case "absolute":
				second = "/tmp/escape.yaml"
			case "code path":
				second = ".github/workflows/escape.yaml"
			case "symlink":
				kind = tar.TypeSymlink
			case "hardlink":
				kind = tar.TypeLink
			case "duplicate":
				second = "oaspec/kubernetes/1.10.json"
			case "existing file":
				if err := writeNew(root, second, []byte("old")); err != nil {
					t.Fatal(err)
				}
			case "symlink parent":
				if err := os.Mkdir(filepath.Join(root, "ETL"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "ETL/crds")); err != nil {
					t.Fatal(err)
				}
			}
			add(second, kind)
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if err := compressed.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(artifact, "schema-updates.tar.gz"), archive.Bytes(), 0644); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(python, "-c", script)
			command.Dir = root
			command.Env = append(os.Environ(), "RUNNER_TEMP="+artifact)
			output, err := command.CombinedOutput()
			if scenario == "valid" {
				if err != nil {
					t.Fatalf("valid artifact rejected: %v: %s", err, output)
				}
				if value, err := os.ReadFile(filepath.Join(root, second)); err != nil || string(value) != "new" {
					t.Fatalf("file not restored: %s %v", value, err)
				}
			} else {
				if err == nil {
					t.Fatal("unsafe artifact accepted")
				}
				if _, err := os.Stat(filepath.Join(root, "oaspec/kubernetes/1.10.json")); !os.IsNotExist(err) {
					t.Fatal("restored early member before validating complete artifact")
				}
				if scenario == "existing file" {
					if value, err := os.ReadFile(filepath.Join(root, second)); err != nil || string(value) != "old" {
						t.Fatal("overwrote existing file")
					}
				}
			}
		})
	}
}

func TestWorkflowRetirementsAreValidatedBeforeAnyChanges(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	script := workflowRestoreScript(t)
	for _, scenario := range []string{"valid", "retirement only", "kubernetes", "missing provenance", "changed file", "code path", "incomplete snapshot", "symlink parent", "symlink file", "duplicate retirement", "mixed target", "addition inside retirement"} {
		t.Run(scenario, func(t *testing.T) {
			root, artifact := t.TempDir(), t.TempDir()
			product, version, target := "flux", "1.0.0", "ETL/crds/flux-1.0.0"
			old := target + "/crd.yaml"
			if scenario == "kubernetes" || scenario == "missing provenance" {
				product, version, target = "kubernetes", "1.9", "oaspec/kubernetes/1.9.json"
				old = target
			}
			const fresh = "oaspec/kubernetes/1.10.json"
			if err := writeNew(root, old, []byte("old")); err != nil {
				t.Fatal(err)
			}
			checksum := fmt.Sprintf("%x", sha256.Sum256([]byte("old")))
			file := map[string]string{"path": old, "sha256": checksum}
			retirement := map[string]any{"product": product, "version": version, "target": target, "files": []any{file}}
			retired := []any{retirement}
			if product == "kubernetes" {
				if err := writeNew(root, "oaspec/kubernetes/1.9.source", []byte("proof")); err != nil {
					t.Fatal(err)
				}
				if scenario == "kubernetes" {
					retirement["files"] = []any{file, map[string]string{"path": "oaspec/kubernetes/1.9.source", "sha256": fmt.Sprintf("%x", sha256.Sum256([]byte("proof")))}}
				}
			}
			switch scenario {
			case "changed file":
				file["sha256"] = strings.Repeat("0", 64)
			case "code path":
				file["path"] = "go.mod"
			case "incomplete snapshot":
				if err := writeNew(root, target+"/upstream.json", []byte("{}")); err != nil {
					t.Fatal(err)
				}
			case "symlink parent":
				if err := os.Rename(filepath.Join(root, target), filepath.Join(root, "backup")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(root, "backup"), filepath.Join(root, target)); err != nil {
					t.Fatal(err)
				}
			case "symlink file":
				if err := os.Rename(filepath.Join(root, old), filepath.Join(root, "backup")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(root, "backup"), filepath.Join(root, old)); err != nil {
					t.Fatal(err)
				}
			case "duplicate retirement":
				retired = append(retired, retirement)
			case "mixed target":
				retirement["target"] = "ETL/crds/flux-2.0.0"
			}
			report, err := json.Marshal(map[string]any{"updates": []any{}, "retired": retired})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(artifact, "schema-updates.json"), report, 0644); err != nil {
				t.Fatal(err)
			}
			var archive bytes.Buffer
			compressed := gzip.NewWriter(&archive)
			writer := tar.NewWriter(compressed)
			if scenario != "retirement only" {
				names := []string{fresh}
				if scenario == "addition inside retirement" {
					names = append(names, target+"/new.yaml")
				}
				for _, name := range names {
					if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0644, Typeflag: tar.TypeReg, Size: 3}); err != nil {
						t.Fatal(err)
					}
					if _, err := writer.Write([]byte("new")); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			if err := compressed.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(artifact, "schema-updates.tar.gz"), archive.Bytes(), 0644); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(python, "-c", script)
			command.Dir = root
			command.Env = append(os.Environ(), "RUNNER_TEMP="+artifact)
			output, err := command.CombinedOutput()
			if scenario == "valid" || scenario == "retirement only" || scenario == "kubernetes" {
				if err != nil {
					t.Fatalf("valid retirement rejected: %v: %s", err, output)
				}
				if _, err := os.Stat(filepath.Join(root, target)); !os.IsNotExist(err) {
					t.Fatal("retired directory remains")
				}
				if scenario == "kubernetes" {
					if _, err := os.Stat(filepath.Join(root, "oaspec/kubernetes/1.9.source")); !os.IsNotExist(err) {
						t.Fatal("retired provenance remains")
					}
				}
				if scenario == "valid" {
					if value, err := os.ReadFile(filepath.Join(root, fresh)); err != nil || string(value) != "new" {
						t.Fatal("new snapshot missing")
					}
				}
			} else {
				if err == nil {
					t.Fatal("unsafe retirement accepted")
				}
				if _, err := os.Stat(filepath.Join(root, fresh)); !os.IsNotExist(err) {
					t.Fatal("restored addition before validating retirements")
				}
				if value, err := os.ReadFile(filepath.Join(root, old)); err != nil || string(value) != "old" {
					t.Fatal("removed original before validating retirements")
				}
			}
		})
	}
}
