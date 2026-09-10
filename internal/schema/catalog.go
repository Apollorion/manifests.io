package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"go.yaml.in/yaml/v3"
	"golang.org/x/mod/semver"
)

var ErrNotFound = errors.New("schema not found")
var ErrBadQuery = errors.New("invalid schema query")

type Product struct {
	Name           string   `json:"name"`
	Versions       []string `json:"versions"`
	DefaultVersion string   `json:"defaultVersion,omitempty"`
}

type Definition struct {
	Name     string `json:"name"`
	Resource string `json:"resource"`
	Href     string `json:"href"`
}

type Query struct {
	Item     string `json:"item"`
	Version  string `json:"version"`
	Resource string `json:"resource,omitempty"`
	Path     string `json:"path,omitempty"`
	Pointer  string `json:"pointer,omitempty"`
	Trail    string `json:"trail,omitempty"`
	OneOf    string `json:"oneOf,omitempty"`
	Key      string `json:"key,omitempty"`
	visits   map[*openapi3.Schema]int
}

type Link struct {
	Label    string `json:"label"`
	Href     string `json:"href"`
	Circular bool   `json:"circular,omitempty"`
}

type Row struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Required    bool     `json:"required,omitempty"`
	Href        string   `json:"href,omitempty"`
	Circular    bool     `json:"circular,omitempty"`
	Constraints []string `json:"constraints,omitempty"`
	Variants    []Link   `json:"variants,omitempty"`
}

type Page struct {
	Error         string    `json:"error,omitempty"`
	Item          string    `json:"item"`
	Version       string    `json:"version"`
	Resource      string    `json:"resource,omitempty"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	Path          string    `json:"path,omitempty"`
	Pointer       string    `json:"pointer,omitempty"`
	Trail         string    `json:"trail,omitempty"`
	Cycles        []string  `json:"cycles,omitempty"`
	Leaf          bool      `json:"leaf,omitempty"`
	Resources     []Row     `json:"resources"`
	OtherVersions []Link    `json:"otherVersions"`
	Breadcrumbs   []Link    `json:"breadcrumbs"`
	Variants      []Link    `json:"variants"`
	Catalog       []Product `json:"catalog"`
	Canonical     string    `json:"canonical"`
}

type location struct{ resource, path string }
type gvk struct{ Group, Version, Kind string }
type document struct {
	schemas         openapi3.Schemas
	aliases         map[string]location
	gvks            map[string][]gvk
	nodes           map[*openapi3.Schema]location
	cyclic          map[string]*openapi3.Schema
	routes          []location
	refDescriptions map[*openapi3.SchemaRef]string
}

type Catalog struct {
	products  []Product
	documents map[string]*document
}

var productAliases = map[string]string{
	"cosignpolicycontroller": "cosign policy-controller",
	"gatewayapi":             "gateway api",
	"opagatekeeper":          "opa gatekeeper",
	"prometheusoperator":     "prometheus operator",
	"spaceliftoperator":      "spacelift operator",
	"spaceliftworkerpool":    "spacelift workerpool",
}

func Load(root string) (*Catalog, error) {
	c := &Catalog{documents: make(map[string]*document)}
	versions := make(map[string][]string)
	add := func(name, version string, doc *document) error {
		if err := doc.indexNodes(); err != nil {
			return err
		}
		key := name + "/" + version
		if _, exists := c.documents[key]; exists {
			return fmt.Errorf("duplicate product version %s", key)
		}
		c.documents[key] = doc
		versions[name] = append(versions[name], version)
		return nil
	}
	entries, err := os.ReadDir(filepath.Join(root, "oaspec", "kubernetes"))
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, "oaspec", "kubernetes", entry.Name()))
		if err != nil {
			return nil, err
		}
		d, err := readOpenAPI(data)
		if err != nil {
			return nil, fmt.Errorf("kubernetes %s: %w", entry.Name(), err)
		}
		if err := add("kubernetes", strings.TrimSuffix(entry.Name(), ".json"), d); err != nil {
			return nil, err
		}
	}
	entries, err = os.ReadDir(filepath.Join(root, "ETL", "crds"))
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name, version, ok := strings.Cut(entry.Name(), "-")
		if !ok || version == "" {
			return nil, fmt.Errorf("CRD directory %q must be product-version", entry.Name())
		}
		if alias := productAliases[name]; alias != "" {
			name = alias
		}
		if name == "gateway api" {
			if strings.HasSuffix(version, "experimental") {
				version = strings.TrimSuffix(version, "experimental") + " experimental"
			} else {
				version += " standard"
			}
		}
		d, err := readCRDs(filepath.Join(root, "ETL", "crds", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if err := add(name, version, d); err != nil {
			return nil, err
		}
	}
	for _, name := range sortedKeys(versions) {
		slices.SortFunc(versions[name], compareVersions)
		c.products = append(c.products, Product{Name: name, Versions: versions[name]})
	}
	return c, nil
}

func compareVersions(a, b string) int {
	left, right := strings.Split(strings.Fields(a)[0], "."), strings.Split(strings.Fields(b)[0], ".")
	for i := 0; i < min(len(left), len(right)); i++ {
		x, ex := strconv.Atoi(left[i])
		y, ey := strconv.Atoi(right[i])
		if ex == nil && ey == nil && x != y {
			if x < y {
				return -1
			}
			return 1
		}
		if ex != nil || ey != nil {
			return strings.Compare(a, b)
		}
	}
	if len(left) != len(right) {
		return len(left) - len(right)
	}
	if strings.HasSuffix(a, " standard") && strings.HasSuffix(b, " experimental") {
		return -1
	}
	if strings.HasSuffix(b, " standard") && strings.HasSuffix(a, " experimental") {
		return 1
	}
	return strings.Compare(a, b)
}

func (c *Catalog) Products() []Product {
	result := make([]Product, len(c.products))
	defaultQuery := DefaultQuery(c.products)
	for i, p := range c.products {
		result[i] = Product{Name: p.Name, Versions: slices.Clone(p.Versions)}
		if p.Name == defaultQuery.Item {
			result[i].DefaultVersion = defaultQuery.Version
		}
	}
	return result
}

func DefaultQuery(products []Product) Query {
	query := Query{Item: "kubernetes"}
	for _, product := range products {
		if product.Name != query.Item {
			continue
		}
		for _, version := range product.Versions {
			candidate := "v" + strings.TrimPrefix(version, "v")
			if !semver.IsValid(candidate) || semver.Prerelease(candidate) != "" {
				continue
			}
			if query.Version == "" || semver.Compare(candidate, "v"+strings.TrimPrefix(query.Version, "v")) > 0 {
				query.Version = version
			}
		}
	}
	return query
}

func (c *Catalog) Definitions(q Query) ([]Definition, error) {
	if q.Item == "" || q.Version == "" || strings.ContainsAny(q.Item+q.Version, "/\\\x00") || q.Resource != "" || q.Path != "" || q.Pointer != "" || q.Trail != "" || q.OneOf != "" || q.Key != "" {
		return nil, ErrBadQuery
	}
	d := c.documents[q.Item+"/"+q.Version]
	if d == nil {
		return nil, ErrNotFound
	}
	definitions := make([]Definition, 0, len(d.schemas)+len(d.aliases))
	for resource := range d.schemas {
		link := q
		link.Resource = resource
		definitions = append(definitions, Definition{Name: shortName(resource), Resource: resource, Href: Href(link)})
	}
	for resource, loc := range d.aliases {
		if d.schemas[resource] != nil {
			continue
		}
		link := q
		link.Resource, link.Pointer = loc.resource, loc.path
		definitions = append(definitions, Definition{Name: shortName(resource), Resource: resource, Href: Href(link)})
	}
	slices.SortFunc(definitions, func(a, b Definition) int {
		if order := strings.Compare(a.Name, b.Name); order != 0 {
			return order
		}
		return strings.Compare(a.Resource, b.Resource)
	})
	return definitions, nil
}

func (c *Catalog) Routes() []Query {
	var result []Query
	for _, p := range c.products {
		for _, version := range p.Versions {
			q := Query{Item: p.Name, Version: version}
			result = append(result, q)
			for _, loc := range c.documents[p.Name+"/"+version].routes {
				q.Resource, q.Pointer = loc.resource, loc.path
				result = append(result, q)
			}
		}
	}
	return result
}

func readOpenAPI(data []byte) (*document, error) {
	var header struct {
		OpenAPI     string         `json:"openapi"`
		Definitions map[string]any `json:"definitions"`
		Components  struct {
			Schemas map[string]any `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, err
	}
	var spec *openapi3.T
	var err error
	loader := openapi3.NewLoader()
	if header.OpenAPI != "" {
		spec, err = loader.LoadFromData(data)
	} else {
		var v2 openapi2.T
		if err = json.Unmarshal(data, &v2); err == nil {
			spec, err = openapi2conv.ToV3WithLoader(&v2, loader, nil)
		}
	}
	if err != nil {
		return nil, err
	}
	if spec.Components == nil || len(spec.Components.Schemas) == 0 {
		return nil, errors.New("document contains no schemas")
	}
	d := newDocument(spec.Components.Schemas)
	rawSchemas := header.Definitions
	if header.OpenAPI != "" {
		rawSchemas = header.Components.Schemas
	}
	for name, raw := range rawSchemas {
		if err := d.preserveRefDescriptions(raw, d.schemas[name]); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	for name, ref := range d.schemas {
		if ref.Value == nil {
			return nil, fmt.Errorf("unresolved schema %q", name)
		}
		if ext := ref.Value.Extensions["x-kubernetes-group-version-kind"]; ext != nil {
			data, err := json.Marshal(ext)
			if err != nil {
				return nil, err
			}
			var values []gvk
			if err := json.Unmarshal(data, &values); err != nil {
				return nil, err
			}
			d.gvks[name] = values
		}
	}
	return d, nil
}

func newDocument(schemas openapi3.Schemas) *document {
	return &document{schemas: schemas, aliases: make(map[string]location), gvks: make(map[string][]gvk), refDescriptions: make(map[*openapi3.SchemaRef]string)}
}

func readCRDs(dir string) (*document, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	d := newDocument(make(openapi3.Schemas))
	for _, entry := range entries {
		if entry.IsDir() || (filepath.Ext(entry.Name()) != ".yaml" && filepath.Ext(entry.Name()) != ".yml") {
			continue
		}
		if err := readCRDFile(filepath.Join(dir, entry.Name()), d); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
	}
	if len(d.schemas) == 0 {
		return nil, errors.New("directory contains no CRD schemas")
	}
	spec := &openapi3.T{OpenAPI: "3.0.3", Components: &openapi3.Components{Schemas: d.schemas}}
	if err := openapi3.NewLoader().ResolveRefsIn(spec, nil); err != nil {
		return nil, err
	}
	return d, nil
}

func readCRDFile(path string, d *document) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	decoder := yaml.NewDecoder(f)
	for {
		var raw map[string]any
		if err := decoder.Decode(&raw); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if err := addCRD(raw, d); err != nil {
			return err
		}
	}
}

func addCRD(raw map[string]any, d *document) error {
	if items, ok := raw["items"].([]any); ok {
		for _, item := range items {
			object, ok := item.(map[string]any)
			if !ok {
				return errors.New("list item is not an object")
			}
			if err := addCRD(object, d); err != nil {
				return err
			}
		}
	}
	if raw["kind"] != "CustomResourceDefinition" {
		return nil
	}
	spec, ok := raw["spec"].(map[string]any)
	if !ok {
		return errors.New("CRD spec is missing")
	}
	names, _ := spec["names"].(map[string]any)
	kind, _ := names["kind"].(string)
	group, _ := spec["group"].(string)
	if kind == "" || group == "" {
		return errors.New("CRD group or kind is missing")
	}
	versions, _ := spec["versions"].([]any)
	if len(versions) == 0 {
		if v, ok := spec["version"].(string); ok {
			versions = []any{map[string]any{"name": v}}
		}
	}
	if len(versions) == 0 {
		return errors.New("CRD versions are missing")
	}
	for _, value := range versions {
		version, ok := value.(map[string]any)
		if !ok {
			return errors.New("CRD version is not an object")
		}
		name, _ := version["name"].(string)
		if name == "" {
			return errors.New("CRD version name is missing")
		}
		wrapper, _ := version["schema"].(map[string]any)
		schema, _ := wrapper["openAPIV3Schema"].(map[string]any)
		if schema == nil {
			validation, _ := spec["validation"].(map[string]any)
			schema, _ = validation["openAPIV3Schema"].(map[string]any)
		}
		if schema == nil {
			return fmt.Errorf("CRD %s %s lacks openAPIV3Schema", kind, name)
		}
		data, err := json.Marshal(schema)
		if err != nil {
			return err
		}
		var ref openapi3.SchemaRef
		if err := json.Unmarshal(data, &ref); err != nil {
			return err
		}
		parts := strings.Split(group, ".")
		slices.Reverse(parts)
		resource := strings.Join(parts, ".") + "." + name + "." + kind
		if _, exists := d.schemas[resource]; exists {
			return fmt.Errorf("duplicate CRD schema %s", resource)
		}
		d.schemas[resource] = &ref
		if err := d.preserveRefDescriptions(schema, &ref); err != nil {
			return err
		}
		d.gvks[resource] = []gvk{{group, name, kind}}
		legacyAliases(d, resource, location{resource: resource}, schema, 0)
	}
	return nil
}

func legacyAliases(d *document, alias string, loc location, raw map[string]any, depth int) {
	if depth > 128 {
		return
	}
	children, ok := raw["properties"].(map[string]any)
	segment := "/properties/"
	if !ok {
		children, _ = raw["items"].(map[string]any)
		segment = "/items/"
	}
	for _, key := range sortedKeys(children) {
		child, ok := children[key].(map[string]any)
		if !ok {
			continue
		}
		_, props := child["properties"]
		_, items := child["items"]
		if !props && !items {
			continue
		}
		name := alias + capitalize(key)
		next := location{loc.resource, loc.path + segment + escapePointer(key)}
		d.aliases[name] = next
		legacyAliases(d, name, next, child, depth+1)
	}
}

func capitalize(s string) string {
	runes := []rune(strings.ToLower(s))
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}
	return string(runes)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func escapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}
