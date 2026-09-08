package upstream

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestWorkflowArtifactRestoreRejectsUnsafeArchivesBeforeExtraction(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal("python3 is required to verify the schema-update workflow")
	}
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
	for _, scenario := range []string{"valid", "traversal", "absolute", "code path", "symlink", "hardlink", "duplicate", "existing file", "symlink parent"} {
		t.Run(scenario, func(t *testing.T) {
			root, artifact := t.TempDir(), t.TempDir()
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
