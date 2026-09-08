package upstream

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/mod/semver"
)

const RetainedVersions = 5

type RetiredFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Retirement struct {
	Product string        `json:"product"`
	Version string        `json:"version"`
	Target  string        `json:"target"`
	Files   []RetiredFile `json:"files"`
}

type Report struct {
	Updates []Update     `json:"updates"`
	Retired []Retirement `json:"retired"`
}

type storedVersion struct {
	product, version, target string
	added                    bool
}

func retentionVersion(product, version string) (string, string, error) {
	track := product
	if product == "gatewayapi" && strings.HasSuffix(version, "experimental") {
		track += "/experimental"
		version = strings.TrimSuffix(version, "experimental")
	}
	if !identifier.MatchString(product) || !semver.IsValid("v"+version) || semver.Prerelease("v"+version) != "" || semver.Build("v"+version) != "" || strings.Count(version, ".") < 1 {
		return "", "", fmt.Errorf("invalid stored version %s %s", product, version)
	}
	return track, "v" + version, nil
}

func PlanRetention(root string, updates []Update) ([]Retirement, error) {
	if err := validateCorpusRoot(root); err != nil {
		return nil, err
	}
	groups := map[string][]storedVersion{}
	targets := map[string]bool{}
	add := func(version storedVersion) error {
		track, _, err := retentionVersion(version.product, version.version)
		if err != nil {
			return err
		}
		if targets[version.target] {
			return fmt.Errorf("duplicate version destination %s", version.target)
		}
		targets[version.target] = true
		groups[track] = append(groups[track], version)
		return nil
	}
	for _, directory := range []string{"oaspec/kubernetes", "ETL/crds"} {
		entries, err := os.ReadDir(filepath.Join(root, directory))
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("symlink in corpus: %s", entry.Name())
			}
			version := storedVersion{target: path.Join(directory, entry.Name())}
			if directory == "oaspec/kubernetes" {
				if entry.IsDir() || path.Ext(entry.Name()) != ".json" {
					continue
				}
				version.product, version.version = "kubernetes", strings.TrimSuffix(entry.Name(), ".json")
			} else {
				if !entry.IsDir() {
					continue
				}
				var ok bool
				version.product, version.version, ok = strings.Cut(entry.Name(), "-")
				if !ok {
					return nil, fmt.Errorf("invalid CRD version directory %s", entry.Name())
				}
			}
			if _, err := retirementFiles(root, version.target, version.product); err != nil {
				return nil, err
			}
			if err := add(version); err != nil {
				return nil, err
			}
		}
	}
	for _, update := range updates {
		expected := "ETL/crds/" + update.Product + "-" + update.Version
		if update.Product == "kubernetes" {
			expected = "oaspec/kubernetes/" + update.Version + ".json"
		}
		if update.Target != expected || !safePath(expected) {
			return nil, errors.New("invalid update destination")
		}
		if err := add(storedVersion{product: update.Product, version: update.Version, target: update.Target, added: true}); err != nil {
			return nil, err
		}
	}
	retired := []Retirement{}
	for _, versions := range groups {
		slices.SortFunc(versions, func(a, b storedVersion) int {
			_, left, _ := retentionVersion(a.product, a.version)
			_, right, _ := retentionVersion(b.product, b.version)
			if comparison := semver.Compare(right, left); comparison != 0 {
				return comparison
			}
			return strings.Compare(b.version, a.version)
		})
		for _, version := range versions[min(RetainedVersions, len(versions)):] {
			if version.added {
				return nil, fmt.Errorf("new update %s is outside retained versions", version.target)
			}
			files, err := retirementFiles(root, version.target, version.product)
			if err != nil {
				return nil, err
			}
			retired = append(retired, Retirement{Product: version.product, Version: version.version, Target: version.target, Files: files})
		}
	}
	slices.SortFunc(retired, func(a, b Retirement) int { return strings.Compare(a.Target, b.Target) })
	return retired, nil
}

func retirementFiles(root, target, product string) ([]RetiredFile, error) {
	files := []RetiredFile{}
	add := func(name string) error {
		info, err := os.Lstat(filepath.Join(root, name))
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("non-regular corpus file %s", name)
		}
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return err
		}
		files = append(files, RetiredFile{Path: name, SHA256: fmt.Sprintf("%x", sha256.Sum256(data))})
		return nil
	}
	if product == "kubernetes" {
		if err := add(target); err != nil {
			return nil, err
		}
		record := strings.TrimSuffix(target, ".json") + ".source"
		if _, err := os.Lstat(filepath.Join(root, record)); err == nil {
			if err := add(record); err != nil {
				return nil, err
			}
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	} else {
		err := filepath.WalkDir(filepath.Join(root, target), func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("symlink in corpus: %s", name)
			}
			if entry.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(root, name)
			if err != nil {
				return err
			}
			return add(filepath.ToSlash(relative))
		})
		if err != nil {
			return nil, err
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("empty stored version %s", target)
	}
	slices.SortFunc(files, func(a, b RetiredFile) int { return strings.Compare(a.Path, b.Path) })
	return files, nil
}

func verifyRetirements(root string, retired []Retirement) error {
	if err := validateCorpusRoot(root); err != nil {
		return err
	}
	for _, version := range retired {
		files, err := retirementFiles(root, version.Target, version.Product)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(files, version.Files) {
			return fmt.Errorf("retired version changed during update: %s", version.Target)
		}
	}
	return nil
}

func retirementTargets(retired []Retirement) []string {
	var targets []string
	for _, version := range retired {
		if version.Product == "kubernetes" {
			for _, file := range version.Files {
				targets = append(targets, file.Path)
			}
		} else {
			targets = append(targets, version.Target)
		}
	}
	return targets
}

func installPlan(ctx context.Context, root, stage string, destinations []string, retired []Retirement) (bool, error) {
	if err := verifyRetirements(root, retired); err != nil {
		return false, err
	}
	ctx, span := otel.Tracer("manifests.io/upstream").Start(ctx, "schema.install")
	defer span.End()
	span.SetAttributes(attribute.String("schema.operation", "prune"), attribute.Int("schema.updates", len(retired)))
	var installed, moved []string
	backup := filepath.Join(stage, "retired")
	rollback := func(cause error) (bool, error) {
		var failures []error
		for _, target := range installed {
			failures = append(failures, os.RemoveAll(filepath.Join(root, target)))
		}
		for _, target := range slices.Backward(moved) {
			original := filepath.Join(root, target)
			if _, err := os.Lstat(original); !errors.Is(err, fs.ErrNotExist) {
				failures = append(failures, fmt.Errorf("cannot restore occupied target %s", target))
				continue
			}
			failures = append(failures, os.Rename(filepath.Join(backup, target), original))
		}
		if failure := errors.Join(failures...); failure != nil {
			return true, errors.Join(cause, fmt.Errorf("rollback incomplete; retired files preserved at %s: %w", backup, failure))
		}
		return false, cause
	}
	for _, target := range retirementTargets(retired) {
		if err := ctx.Err(); err != nil {
			return rollback(err)
		}
		to := filepath.Join(backup, target)
		if err := os.MkdirAll(filepath.Dir(to), 0755); err != nil {
			return rollback(err)
		}
		if err := os.Rename(filepath.Join(root, target), to); err != nil {
			return rollback(err)
		}
		moved = append(moved, target)
	}
	for _, destination := range destinations {
		if err := ctx.Err(); err != nil {
			return rollback(err)
		}
		from, to := filepath.Join(stage, destination), filepath.Join(root, destination)
		info, err := os.Stat(from)
		if err != nil {
			return rollback(err)
		}
		if info.IsDir() {
			if err := os.Mkdir(to, 0755); err != nil {
				return rollback(err)
			}
			installed = append(installed, destination)
			if err := os.CopyFS(to, os.DirFS(from)); err != nil {
				return rollback(err)
			}
		} else {
			if err := os.Link(from, to); err != nil {
				return rollback(err)
			}
			installed = append(installed, destination)
		}
	}
	if err := ctx.Err(); err != nil {
		return rollback(err)
	}
	slog.InfoContext(ctx, "old schema snapshots retired", "schema.operation", "prune", "schema.updates", len(retired))
	return false, nil
}
