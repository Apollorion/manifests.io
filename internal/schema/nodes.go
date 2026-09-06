package schema

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

func (d *document) indexNodes() error {
	d.nodes = make(map[*openapi3.Schema]location)
	for _, name := range sortedKeys(d.schemas) {
		if ref := d.schemas[name]; ref != nil && ref.Value != nil {
			if _, exists := d.nodes[ref.Value]; !exists {
				d.nodes[ref.Value] = location{resource: name}
			}
		}
	}
	seen := make(map[*openapi3.Schema]bool)
	var visit func(*openapi3.SchemaRef, location, int) error
	visit = func(ref *openapi3.SchemaRef, loc location, depth int) error {
		if ref == nil || ref.Value == nil || seen[ref.Value] {
			return nil
		}
		if depth > 128 {
			return fmt.Errorf("schema nesting exceeds 128 at %s%s", loc.resource, loc.path)
		}
		s := ref.Value
		seen[s] = true
		if canonical, exists := d.nodes[s]; exists {
			loc = canonical
		} else {
			d.nodes[s] = loc
		}
		child := func(ref *openapi3.SchemaRef, path string) error {
			return visit(ref, location{loc.resource, loc.path + path}, depth+1)
		}
		for _, container := range []struct {
			name    string
			schemas openapi3.Schemas
		}{
			{"properties", s.Properties}, {"patternProperties", s.PatternProperties}, {"$defs", s.Defs}, {"dependentSchemas", s.DependentSchemas},
		} {
			for _, name := range sortedKeys(container.schemas) {
				if err := child(container.schemas[name], "/"+container.name+"/"+escapePointer(name)); err != nil {
					return err
				}
			}
		}
		for _, set := range []struct {
			name string
			refs openapi3.SchemaRefs
		}{{"oneOf", s.OneOf}, {"anyOf", s.AnyOf}, {"allOf", s.AllOf}} {
			for i, ref := range set.refs {
				if err := child(ref, "/"+set.name+"/"+strconv.Itoa(i)); err != nil {
					return err
				}
			}
		}
		for _, single := range []struct {
			name string
			ref  *openapi3.SchemaRef
		}{{"items", s.Items}, {"additionalProperties", s.AdditionalProperties.Schema}, {"not", s.Not}} {
			if err := child(single.ref, "/"+single.name); err != nil {
				return err
			}
		}
		return nil
	}
	for _, name := range sortedKeys(d.schemas) {
		if err := visit(d.schemas[name], location{resource: name}, 0); err != nil {
			return err
		}
	}
	d.routes = make([]location, 0, len(d.nodes))
	for _, loc := range d.nodes {
		d.routes = append(d.routes, loc)
	}
	slices.SortFunc(d.routes, func(a, b location) int {
		if c := strings.Compare(a.resource, b.resource); c != 0 {
			return c
		}
		return strings.Compare(a.path, b.path)
	})
	return nil
}
