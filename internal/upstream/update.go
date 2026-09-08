package upstream

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/TheOutdoorProgrammer/manifests.io/internal/schema"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.yaml.in/yaml/v3"
	"golang.org/x/mod/semver"
)

//go:embed sources.json
var registry []byte

const maxDownload = 64 << 20

type Source struct {
	Product      string   `json:"product"`
	Repository   string   `json:"repository"`
	Asset        string   `json:"asset,omitempty"`
	Files        []string `json:"files,omitempty"`
	Directory    string   `json:"directory,omitempty"`
	Suffix       string   `json:"suffix,omitempty"`
	MinorVersion bool     `json:"minorVersion,omitempty"`
	VersionFile  string   `json:"versionFile,omitempty"`
}

type File struct {
	Path   string `json:"path"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256,omitempty"`
}

type Update struct {
	Product    string `json:"product"`
	Version    string `json:"version"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Commit     string `json:"commit,omitempty"`
	Files      []File `json:"files"`
	Target     string `json:"target"`
}

type Client struct {
	HTTP    *http.Client
	Token   string
	APIBase string
	RawBase string
}

func NewClient(token string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 90 * time.Second, CheckRedirect: func(request *http.Request, via []*http.Request) error {
		request.Header.Del("Authorization")
		if len(via) >= 5 || request.URL.Scheme != "https" {
			return errors.New("unsafe or excessive upstream redirects")
		}
		return nil
	}}, Token: token, APIBase: "https://api.github.com", RawBase: "https://raw.githubusercontent.com"}
}

func Sources() ([]Source, error) {
	var sources []Source
	if err := json.Unmarshal(registry, &sources); err != nil {
		return nil, err
	}
	return sources, nil
}

var identifier = regexp.MustCompile(`^[a-z0-9]+$`)
var repository = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

func safePath(value string) bool {
	return value != "." && fs.ValidPath(value) && !strings.ContainsAny(value, "\\\x00")
}

func validateSource(source Source) error {
	if !identifier.MatchString(source.Product) || !repository.MatchString(source.Repository) || (source.Suffix != "" && source.Suffix != "experimental") {
		return errors.New("invalid product, repository or suffix")
	}
	if source.VersionFile != "" && (!safePath(source.VersionFile) || source.Asset != "") {
		return errors.New("invalid chart version source")
	}
	strategies := 0
	if source.Asset != "" {
		strategies++
		if !safePath(source.Asset) || path.Base(source.Asset) != source.Asset {
			return errors.New("invalid asset name")
		}
	}
	if source.Directory != "" {
		strategies++
		if !safePath(source.Directory) {
			return errors.New("invalid source directory")
		}
	}
	if len(source.Files) != 0 {
		strategies++
		for _, file := range source.Files {
			if !safePath(file) {
				return errors.New("invalid source file")
			}
		}
	}
	if strategies != 1 || (source.Product == "kubernetes" && (len(source.Files) != 1 || source.Suffix != "")) || (source.MinorVersion && source.Product != "kubernetes") {
		return errors.New("source requires exactly one download strategy")
	}
	return nil
}

func stableVersion(tag string) (string, error) {
	version := "v" + strings.TrimPrefix(tag, "v")
	if !semver.IsValid(version) || semver.Prerelease(version) != "" || semver.Build(version) != "" || semver.Canonical(version) != version {
		return "", fmt.Errorf("not a stable semantic release: %q", tag)
	}
	return version, nil
}

func (c *Client) get(ctx context.Context, endpoint string, limit int64) ([]byte, error) {
	ctx, span := otel.Tracer("manifests.io/upstream").Start(ctx, "schema.download")
	defer span.End()
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
		return nil, errors.New("invalid upstream URL")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "manifests.io-schema-updater")
	api, err := url.Parse(c.APIBase)
	if err != nil {
		return nil, err
	}
	if u.Scheme == api.Scheme && u.Host == api.Host && c.Token != "" {
		request.Header.Set("Authorization", "Bearer "+c.Token)
	}
	response, err := c.HTTP.Do(request)
	if err != nil {
		span.SetStatus(codes.Error, "request failed")
		return nil, fmt.Errorf("upstream request failed: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	span.SetAttributes(attribute.Int("http.response.status_code", response.StatusCode))
	if response.StatusCode != http.StatusOK {
		span.SetStatus(codes.Error, "upstream status")
		return nil, fmt.Errorf("upstream returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("upstream response exceeds size limit")
	}
	return data, nil
}

func (c *Client) readJSON(ctx context.Context, endpoint string, value any) error {
	data, err := c.get(ctx, endpoint, maxDownload)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

func (c *Client) Discover(ctx context.Context, root string, sources []Source) ([]Update, error) {
	if err := validateCorpusRoot(root); err != nil {
		return nil, err
	}
	var updates []Update
	seen := map[string]bool{}
	for _, source := range sources {
		if err := validateSource(source); err != nil {
			return nil, fmt.Errorf("%s: %w", source.Product, err)
		}
		key := source.Product + source.Suffix
		if seen[key] {
			return nil, fmt.Errorf("duplicate source %s", key)
		}
		seen[key] = true
		update, err := c.discover(ctx, root, source)
		if err != nil {
			return nil, fmt.Errorf("%s%s: %w", source.Product, source.Suffix, err)
		}
		if update != nil {
			updates = append(updates, *update)
		}
	}
	return updates, nil
}

func (c *Client) discover(ctx context.Context, root string, source Source) (*Update, error) {
	ctx, span := otel.Tracer("manifests.io/upstream").Start(ctx, "schema.discover")
	defer span.End()
	span.SetAttributes(attribute.String("schema.product", source.Product))
	slog.InfoContext(ctx, "checking upstream release", "schema.product", source.Product)
	base := c.APIBase + "/repos/" + source.Repository
	var release struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := c.readJSON(ctx, base+"/releases/latest", &release); err != nil {
		return nil, err
	}
	version, err := stableVersion(release.Tag)
	if err != nil {
		return nil, err
	}
	if release.Draft || release.Prerelease {
		return nil, errors.New("latest release is not stable")
	}
	var commit string
	if source.VersionFile != "" {
		commit, err = c.commit(ctx, base, release.Tag)
		if err != nil {
			return nil, err
		}
		data, err := c.get(ctx, c.RawBase+"/"+source.Repository+"/"+commit+"/"+source.VersionFile, 1<<20)
		if err != nil {
			return nil, err
		}
		var chart struct {
			AppVersion string `yaml:"appVersion"`
		}
		if err := yaml.Unmarshal(data, &chart); err != nil {
			return nil, err
		}
		version, err = stableVersion(chart.AppVersion)
		if err != nil {
			return nil, fmt.Errorf("chart appVersion: %w", err)
		}
	}
	if source.MinorVersion {
		version = semver.MajorMinor(version)
	}
	version = strings.TrimPrefix(version, "v") + source.Suffix
	newer, err := isNewer(root, source, version)
	if err != nil || !newer {
		return nil, err
	}
	target := path.Join("ETL/crds", source.Product+"-"+version)
	if source.Product == "kubernetes" {
		target = path.Join("oaspec/kubernetes", version+".json")
	}
	if _, err := os.Lstat(filepath.Join(root, target)); err == nil {
		return nil, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	update := &Update{Product: source.Product, Version: version, Repository: source.Repository, Tag: release.Tag, Target: target}
	if source.Asset != "" {
		for _, asset := range release.Assets {
			if asset.Name == source.Asset {
				expected := "https://github.com/" + source.Repository + "/releases/download/" + url.PathEscape(release.Tag) + "/" + source.Asset
				if asset.URL != expected {
					return nil, errors.New("release asset URL does not match its repository and tag")
				}
				update.Files = append(update.Files, File{Path: source.Asset, URL: asset.URL})
			}
		}
		if len(update.Files) != 1 {
			return nil, fmt.Errorf("release asset %s missing or duplicated", source.Asset)
		}
		return update, nil
	}
	if commit == "" {
		commit, err = c.commit(ctx, base, release.Tag)
		if err != nil {
			return nil, err
		}
	}
	update.Commit = commit
	files := slices.Clone(source.Files)
	if source.Directory != "" {
		var tree struct {
			Truncated bool `json:"truncated"`
			Tree      []struct {
				Path string `json:"path"`
				Type string `json:"type"`
				Mode string `json:"mode"`
			} `json:"tree"`
		}
		if err := c.readJSON(ctx, base+"/git/trees/"+commit+"?recursive=1", &tree); err != nil {
			return nil, err
		}
		if tree.Truncated {
			return nil, errors.New("GitHub source tree is truncated")
		}
		for _, entry := range tree.Tree {
			if strings.HasPrefix(entry.Path, source.Directory+"/") && (path.Ext(entry.Path) == ".yaml" || path.Ext(entry.Path) == ".yml") {
				if !safePath(entry.Path) || entry.Type != "blob" || entry.Mode != "100644" {
					return nil, errors.New("source tree contains an unsafe schema file")
				}
				files = append(files, entry.Path)
			}
		}
	}
	if len(files) == 0 || len(files) > 200 {
		return nil, errors.New("source must contain 1 to 200 schema files")
	}
	slices.Sort(files)
	for _, file := range files {
		update.Files = append(update.Files, File{Path: strings.ReplaceAll(file, "/", "__"), URL: c.RawBase + "/" + source.Repository + "/" + commit + "/" + file})
	}
	return update, nil
}

func (c *Client) commit(ctx context.Context, base, tag string) (string, error) {
	var commit struct {
		SHA string `json:"sha"`
	}
	if err := c.readJSON(ctx, base+"/commits/"+url.PathEscape(tag), &commit); err != nil {
		return "", err
	}
	if len(commit.SHA) != 40 || strings.Trim(commit.SHA, "0123456789abcdef") != "" {
		return "", errors.New("invalid release commit")
	}
	return commit.SHA, nil
}

func isNewer(root string, source Source, version string) (bool, error) {
	directory := filepath.Join(root, "ETL/crds")
	if source.Product == "kubernetes" {
		directory = filepath.Join(root, "oaspec/kubernetes")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		return false, err
	}
	candidate := "v" + strings.TrimSuffix(version, source.Suffix)
	for _, entry := range entries {
		var existing string
		if source.Product == "kubernetes" {
			if entry.IsDir() || path.Ext(entry.Name()) != ".json" {
				continue
			}
			existing = strings.TrimSuffix(entry.Name(), ".json")
		} else {
			if !entry.IsDir() || !strings.HasPrefix(entry.Name(), source.Product+"-") {
				continue
			}
			existing = strings.TrimPrefix(entry.Name(), source.Product+"-")
			if strings.HasSuffix(existing, "experimental") != (source.Suffix == "experimental") {
				continue
			}
			existing = strings.TrimSuffix(existing, source.Suffix)
		}
		if semver.IsValid("v"+existing) && semver.Compare("v"+existing, candidate) >= 0 {
			return false, nil
		}
	}
	return true, nil
}

func (c *Client) Apply(ctx context.Context, root string, updates []Update) error {
	if len(updates) == 0 {
		return nil
	}
	if err := validateCorpusRoot(root); err != nil {
		return err
	}
	ctx, span := otel.Tracer("manifests.io/upstream").Start(ctx, "schema.install")
	defer span.End()
	span.SetAttributes(attribute.Int("schema.updates", len(updates)))
	stage, err := os.MkdirTemp(root, ".schema-update-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	for _, dir := range []string{"oaspec", "ETL/crds"} {
		if err := os.CopyFS(filepath.Join(stage, dir), os.DirFS(filepath.Join(root, dir))); err != nil {
			return err
		}
	}
	var destinations []string
	var downloaded int64
	for index := range updates {
		update := &updates[index]
		expected := "ETL/crds/" + update.Product + "-" + update.Version
		if update.Product == "kubernetes" {
			expected = "oaspec/kubernetes/" + update.Version + ".json"
		}
		if !safePath(update.Target) || update.Target != expected || !identifier.MatchString(update.Product) || len(update.Files) == 0 {
			return errors.New("invalid update destination")
		}
		if _, err := os.Lstat(filepath.Join(root, update.Target)); !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("destination %s already exists or is inaccessible", update.Target)
		}
		for i := range update.Files {
			file := &update.Files[i]
			if !safePath(file.Path) || path.Base(file.Path) != file.Path {
				return errors.New("invalid downloaded filename")
			}
			data, err := c.get(ctx, file.URL, maxDownload)
			if err != nil {
				return fmt.Errorf("%s %s: %w", update.Product, file.Path, err)
			}
			downloaded += int64(len(data))
			if downloaded > 256<<20 {
				return errors.New("schema update batch exceeds size limit")
			}
			file.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
			destination := path.Join(update.Target, file.Path)
			if update.Product == "kubernetes" {
				destination = update.Target
			}
			if err := writeNew(stage, destination, data); err != nil {
				return err
			}
		}
		provenance, err := json.MarshalIndent(update, "", "  ")
		if err != nil {
			return err
		}
		record := path.Join(update.Target, "upstream.json")
		if update.Product == "kubernetes" {
			record = strings.TrimSuffix(update.Target, ".json") + ".source"
			destinations = append(destinations, record)
		}
		if err := writeNew(stage, record, append(provenance, '\n')); err != nil {
			return err
		}
		destinations = append(destinations, update.Target)
	}
	validationCtx, validationSpan := otel.Tracer("manifests.io/upstream").Start(ctx, "schema.validate")
	_, validationErr := schema.Load(stage)
	if validationErr != nil {
		validationSpan.SetStatus(codes.Error, "invalid catalog")
		validationSpan.End()
		span.SetStatus(codes.Error, "invalid catalog")
		return fmt.Errorf("staged catalog validation failed: %w", validationErr)
	}
	slog.InfoContext(validationCtx, "staged catalog validated", "schema.updates", len(updates))
	validationSpan.End()
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("staged catalog validation failed: %w", err)
	}
	var installed []string
	for _, destination := range destinations {
		from, to := filepath.Join(stage, destination), filepath.Join(root, destination)
		err := ctx.Err()
		var info os.FileInfo
		if err == nil {
			info, err = os.Stat(from)
		}
		if err == nil {
			if info.IsDir() {
				err = os.Mkdir(to, 0755)
				if err == nil {
					installed = append(installed, to)
					err = os.CopyFS(to, os.DirFS(from))
				}
			} else {
				err = os.Link(from, to)
				if err == nil {
					installed = append(installed, to)
				}
			}
		}
		if err != nil {
			for _, file := range installed {
				_ = os.RemoveAll(file)
			}
			return err
		}
	}
	slog.InfoContext(ctx, "schema snapshots installed", "schema.updates", len(updates))
	return nil
}

func validateCorpusRoot(root string) error {
	for _, component := range []string{"ETL", "ETL/crds", "oaspec", "oaspec/kubernetes"} {
		info, err := os.Lstat(filepath.Join(root, component))
		if err != nil {
			return err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("corpus directory %s must not be a symlink", component)
		}
	}
	return nil
}

func writeNew(root, name string, data []byte) error {
	file := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return errors.Join(err, f.Close())
}
