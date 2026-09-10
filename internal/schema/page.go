package schema

import (
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

func (c *Catalog) Page(q Query) (Page, error) {
	if q.Item == "" && q.Version == "" {
		defaultQuery := DefaultQuery(c.products)
		q.Item, q.Version = defaultQuery.Item, defaultQuery.Version
	}
	p := Page{Item: q.Item, Version: q.Version, Resource: q.Resource, Catalog: c.Products(), Resources: []Row{}, OtherVersions: []Link{}, Breadcrumbs: []Link{}, Variants: []Link{}}
	if len(q.Pointer) > 8192 || len(q.Path) > 8192 || len(q.Resource) > 2048 {
		return p, ErrBadQuery
	}
	d := c.documents[q.Item+"/"+q.Version]
	if d == nil {
		return p, ErrNotFound
	}
	base := Query{Item: q.Item, Version: q.Version}
	p.Breadcrumbs = append(p.Breadcrumbs, Link{Label: q.Item + " " + q.Version, Href: Href(base)})
	if q.Resource == "" {
		if q.Pointer != "" || q.Path != "" || q.Trail != "" || q.OneOf != "" || q.Key != "" {
			return p, ErrBadQuery
		}
		p.Title = q.Item + " " + q.Version
		p.Canonical = Href(base)
		seen := make(map[string]bool)
		for _, name := range sortedKeys(d.gvks) {
			for _, g := range d.gvks[name] {
				if strings.HasSuffix(g.Kind, "List") || seen[g.Kind] {
					continue
				}
				seen[g.Kind] = true
				link := base
				link.Resource = name
				p.Resources = append(p.Resources, Row{Name: g.Kind, Type: g.Kind, Description: d.schemas[name].Value.Description, Href: Href(link)})
			}
		}
		slices.SortFunc(p.Resources, func(a, b Row) int { return strings.Compare(a.Name, b.Name) })
		return p, nil
	}
	if alias, ok := d.aliases[q.Resource]; ok && d.schemas[q.Resource] == nil {
		q.Resource = alias.resource
		q.Pointer = alias.path + q.Pointer
	}
	ref := d.schemas[q.Resource]
	if ref == nil || ref.Value == nil {
		return p, ErrNotFound
	}
	selected, err := navigate(ref.Value, q.Pointer)
	if err != nil {
		return p, err
	}
	if q.OneOf != "" || q.Key != "" {
		if q.Key == "" {
			return p, ErrBadQuery
		}
		property := selected.Properties[q.Key]
		if property == nil || property.Value == nil {
			return p, ErrNotFound
		}
		found := -1
		for i, candidate := range property.Value.OneOf {
			if candidate.Value != nil && candidate.Value.Title == q.OneOf {
				found = i
				break
			}
		}
		if found < 0 {
			return p, ErrNotFound
		}
		q.Pointer += "/properties/" + escapePointer(q.Key) + "/oneOf/" + strconv.Itoa(found)
		if q.Path != "" {
			q.Path += "." + q.Key
		}
		selected = property.Value.OneOf[found].Value
		q.OneOf, q.Key = "", ""
	}
	q = d.canonicalQuery(selected, q)
	p.Cycles = sortedKeys(d.cyclic)
	p.Resource, p.Pointer, p.Path = q.Resource, q.Pointer, q.Path
	p.Trail = q.Trail
	if err := d.trackVisits(&q, selected); err != nil {
		return p, err
	}
	p.Title = shortName(q.Resource) + displayPath(q.Pointer)
	if q.Path != "" {
		p.Title = q.Path
	}
	p.Description = selected.Description
	canonical := q
	canonical.Path = ""
	canonical.Trail = ""
	p.Canonical = Href(canonical)
	q.Path = p.Title
	root := base
	root.Resource = q.Resource
	p.Breadcrumbs = append(p.Breadcrumbs, Link{Label: shortName(q.Resource), Href: Href(root)})
	if p.Title != shortName(q.Resource) {
		current := q
		current.Trail = p.Trail
		p.Breadcrumbs = append(p.Breadcrumbs, Link{Label: p.Title, Href: Href(current)})
	}
	kinds := make(map[string]bool)
	for _, current := range d.gvks[q.Resource] {
		kinds[current.Kind] = true
	}
	if len(kinds) != 0 {
		for _, name := range sortedKeys(d.gvks) {
			for _, g := range d.gvks[name] {
				if !kinds[g.Kind] {
					continue
				}
				link := base
				link.Resource = name
				p.OtherVersions = append(p.OtherVersions, Link{Label: strings.TrimPrefix(g.Group+"/"+g.Version+"/"+g.Kind, "/"), Href: Href(link)})
			}
		}
	}
	p.Variants = d.variants(selected, q)
	rowSchema, rowQuery := selected, q
	if selected.Items != nil && selected.Items.Value != nil {
		rowSchema = selected.Items.Value
		rowQuery.Pointer += "/items"
		p.Variants = append(p.Variants, d.variants(rowSchema, rowQuery)...)
	}
	for _, name := range sortedKeys(rowSchema.Properties) {
		child := rowSchema.Properties[name]
		childQ := rowQuery
		childQ.Pointer += "/properties/" + escapePointer(name)
		childQ.Path += "." + name
		row := d.buildRow(name, child, childQ)
		row.Required = slices.Contains(rowSchema.Required, name)
		p.Resources = append(p.Resources, row)
	}
	if additional := rowSchema.AdditionalProperties.Schema; additional != nil {
		childQ := rowQuery
		childQ.Pointer += "/additionalProperties"
		childQ.Path += ".[key]"
		p.Resources = append(p.Resources, d.buildRow("[key]", additional, childQ))
	} else if rowSchema.AdditionalProperties.Has != nil && *rowSchema.AdditionalProperties.Has {
		p.Resources = append(p.Resources, Row{Name: "[key]", Type: "any"})
	}
	if len(p.Resources) == 0 {
		p.Leaf = true
		row := d.buildRow(p.Title, &openapi3.SchemaRef{Value: rowSchema}, rowQuery)
		row.Href = ""
		row.Circular = false
		p.Resources = append(p.Resources, row)
	}
	return p, nil
}

func Href(q Query) string {
	path := "/" + url.PathEscape(q.Item) + "/" + url.PathEscape(q.Version)
	if q.Resource != "" {
		path += "/" + url.PathEscape(q.Resource)
	}
	values := url.Values{}
	if q.Path != "" {
		values.Set("path", q.Path)
	}
	if q.Pointer != "" {
		values.Set("pointer", q.Pointer)
	}
	if q.Trail != "" {
		values.Set("trail", q.Trail)
	}
	if q.OneOf != "" {
		values.Set("oneOf", q.OneOf)
	}
	if q.Key != "" {
		values.Set("key", q.Key)
	}
	if len(values) > 0 {
		path += "?" + values.Encode()
	}
	return path
}

func navigate(schema *openapi3.Schema, pointer string) (*openapi3.Schema, error) {
	ref, err := navigateRef(&openapi3.SchemaRef{Value: schema}, pointer)
	if err != nil {
		return nil, err
	}
	if ref.Value == nil {
		return nil, ErrNotFound
	}
	return ref.Value, nil
}

func navigateRef(root *openapi3.SchemaRef, pointer string) (*openapi3.SchemaRef, error) {
	if pointer == "" {
		return root, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, ErrBadQuery
	}
	parts := strings.Split(pointer[1:], "/")
	if len(parts) > 128 {
		return nil, ErrBadQuery
	}
	schema := root.Value
	for i := 0; i < len(parts); i++ {
		if schema == nil {
			return nil, ErrNotFound
		}
		var ref *openapi3.SchemaRef
		switch parts[i] {
		case "properties", "patternProperties", "$defs", "dependentSchemas":
			container := parts[i]
			i++
			if i == len(parts) {
				return nil, ErrBadQuery
			}
			key, err := unescapePointer(parts[i])
			if err != nil {
				return nil, err
			}
			switch container {
			case "properties":
				ref = schema.Properties[key]
			case "patternProperties":
				ref = schema.PatternProperties[key]
			case "$defs":
				ref = schema.Defs[key]
			case "dependentSchemas":
				ref = schema.DependentSchemas[key]
			}
		case "items":
			ref = schema.Items
		case "additionalProperties":
			ref = schema.AdditionalProperties.Schema
		case "not":
			ref = schema.Not
		case "oneOf", "anyOf", "allOf":
			var refs openapi3.SchemaRefs
			switch parts[i] {
			case "oneOf":
				refs = schema.OneOf
			case "anyOf":
				refs = schema.AnyOf
			case "allOf":
				refs = schema.AllOf
			}
			i++
			if i == len(parts) {
				return nil, ErrBadQuery
			}
			index, err := strconv.Atoi(parts[i])
			if err != nil || index < 0 {
				return nil, ErrBadQuery
			}
			if index >= len(refs) {
				return nil, ErrNotFound
			}
			ref = refs[index]
		default:
			return nil, ErrBadQuery
		}
		if ref == nil {
			return nil, ErrNotFound
		}
		schema = ref.Value
		root = ref
	}
	return root, nil
}

func unescapePointer(s string) (string, error) {
	for i := 0; i < len(s); i++ {
		if s[i] == '~' {
			i++
			if i == len(s) || (s[i] != '0' && s[i] != '1') {
				return "", ErrBadQuery
			}
		}
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~"), nil
}

func displayPath(path string) string {
	parts := strings.Split(path, "/")
	var result string
	for i := 1; i < len(parts); i++ {
		switch parts[i] {
		case "properties":
			i++
			if i < len(parts) {
				key, _ := unescapePointer(parts[i])
				result += "." + key
			}
		case "items":
			result += "[]"
		case "additionalProperties":
			result += "[key]"
		case "oneOf", "allOf", "anyOf":
			kind := parts[i]
			i++
			if i < len(parts) {
				index, _ := strconv.Atoi(parts[i])
				result += " (" + kind + " " + strconv.Itoa(index+1) + ")"
			}
		}
	}
	return result
}

func shortName(s string) string {
	if i := strings.LastIndexAny(s, "./"); i >= 0 {
		return s[i+1:]
	}
	return s
}

func (d *document) buildRow(name string, ref *openapi3.SchemaRef, q Query) Row {
	row := Row{Name: name, Type: schemaType(ref, make(map[*openapi3.Schema]bool), 0)}
	if ref == nil || ref.Value == nil {
		return row
	}
	s := ref.Value
	row.Description = s.Description
	if description, ok := d.refDescriptions[ref]; ok {
		row.Description = description
	}
	row.Constraints = constraints(s)
	target := s
	q = d.canonicalQuery(s, q)
	row.Variants = d.variants(s, q)
	if s.Items != nil && s.Items.Value != nil {
		itemsQuery := q
		itemsQuery.Pointer += "/items"
		row.Variants = append(row.Variants, d.variants(s.Items.Value, itemsQuery)...)
		if s.Items.Ref != "" {
			q = d.canonicalQuery(s.Items.Value, itemsQuery)
			target = s.Items.Value
		}
	}
	if ref.Ref != "" || len(s.Properties) > 0 || s.Items != nil || s.AdditionalProperties.Schema != nil || len(row.Variants) > 0 {
		link := d.navigationLink(name, target, q)
		row.Href, row.Circular = link.Href, link.Circular
	}
	return row
}

func schemaType(ref *openapi3.SchemaRef, seen map[*openapi3.Schema]bool, depth int) string {
	if ref == nil || ref.Value == nil {
		return "unknown"
	}
	s := ref.Value
	if ref.Ref != "" {
		return shortName(ref.Ref)
	}
	if seen[s] || depth >= 16 {
		return "object"
	}
	seen[s] = true
	defer delete(seen, s)
	if s.Extensions["x-kubernetes-int-or-string"] == true || s.Format == "int-or-string" {
		return "integer | string"
	}
	if s.Items != nil {
		typeName := schemaType(s.Items, seen, depth+1)
		if strings.Contains(typeName, " | ") {
			typeName = "(" + typeName + ")"
		}
		return typeName + "[]"
	}
	for _, refs := range []openapi3.SchemaRefs{s.OneOf, s.AnyOf} {
		if len(refs) > 0 {
			var types []string
			for _, r := range refs {
				value := schemaType(r, seen, depth+1)
				if !slices.Contains(types, value) {
					types = append(types, value)
				}
			}
			return strings.Join(types, " | ")
		}
	}
	if s.AdditionalProperties.Schema != nil {
		return "map[string]" + schemaType(s.AdditionalProperties.Schema, seen, depth+1)
	}
	if s.AdditionalProperties.Has != nil && *s.AdditionalProperties.Has {
		return "map[string]any"
	}
	if s.Type != nil && len(*s.Type) > 0 {
		return strings.Join(*s.Type, " | ")
	}
	if len(s.Properties) > 0 || len(s.AllOf) > 0 {
		return "object"
	}
	return "any"
}

func (d *document) canonicalQuery(s *openapi3.Schema, q Query) Query {
	if loc, ok := d.nodes[s]; ok {
		q.Resource, q.Pointer = loc.resource, loc.path
	}
	return q
}

func (d *document) variants(s *openapi3.Schema, q Query) []Link {
	var links []Link
	for _, set := range []struct {
		name string
		refs openapi3.SchemaRefs
	}{{"oneOf", s.OneOf}, {"anyOf", s.AnyOf}, {"allOf", s.AllOf}} {
		for i, ref := range set.refs {
			label := set.name + " " + strconv.Itoa(i+1)
			if ref.Value != nil && ref.Value.Title != "" {
				label = ref.Value.Title
			}
			child := q
			child.Pointer += "/" + set.name + "/" + strconv.Itoa(i)
			links = append(links, d.navigationLink(label, ref.Value, child))
		}
	}
	return links
}

func constraints(s *openapi3.Schema) []string {
	var values []string
	add := func(name string, v any) { data, _ := json.Marshal(v); values = append(values, name+": "+string(data)) }
	if s.Format != "" {
		values = append(values, "format: "+s.Format)
	}
	if len(s.Enum) > 0 {
		add("enum", s.Enum)
	}
	if s.Default != nil {
		add("default", s.Default)
	}
	if s.Min != nil {
		add("minimum", *s.Min)
	}
	if s.Max != nil {
		add("maximum", *s.Max)
	}
	if s.ExclusiveMin.IsTrue() || s.ExclusiveMin.Value != nil {
		add("exclusiveMinimum", s.ExclusiveMin)
	}
	if s.ExclusiveMax.IsTrue() || s.ExclusiveMax.Value != nil {
		add("exclusiveMaximum", s.ExclusiveMax)
	}
	if s.MultipleOf != nil {
		add("multipleOf", *s.MultipleOf)
	}
	if s.MinLength > 0 {
		add("minLength", s.MinLength)
	}
	if s.MaxLength != nil {
		add("maxLength", *s.MaxLength)
	}
	if s.MinItems > 0 {
		add("minItems", s.MinItems)
	}
	if s.MaxItems != nil {
		add("maxItems", *s.MaxItems)
	}
	if s.MinProps > 0 {
		add("minProperties", s.MinProps)
	}
	if s.MaxProps != nil {
		add("maxProperties", *s.MaxProps)
	}
	if s.AdditionalProperties.Has != nil && !*s.AdditionalProperties.Has {
		add("additionalProperties", false)
	}
	if s.UniqueItems {
		values = append(values, "unique items")
	}
	if s.Pattern != "" {
		values = append(values, "pattern: "+s.Pattern)
	}
	if s.Nullable {
		values = append(values, "nullable")
	}
	if s.ReadOnly {
		values = append(values, "read-only")
	}
	if s.WriteOnly {
		values = append(values, "write-only")
	}
	if s.Deprecated {
		values = append(values, "deprecated")
	}
	for _, key := range sortedKeys(s.Extensions) {
		if strings.HasPrefix(key, "x-kubernetes-") {
			add(key, s.Extensions[key])
		}
	}
	return values
}

func (q Query) String() string {
	return fmt.Sprintf("%s %s %s%s (%s)", q.Item, q.Version, q.Resource, q.Pointer, q.Path)
}
