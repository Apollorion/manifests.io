package schema

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/getkin/kin-openapi/openapi3"
)

// OpenAPI v2 drops $ref siblings, including field-specific descriptions.
func (d *document) preserveRefDescriptions(raw any, root *openapi3.SchemaRef) error {
	var visit func(any, string, int) error
	visit = func(value any, path string, depth int) error {
		if depth > 256 {
			return fmt.Errorf("description metadata nesting exceeds 256 at %s", path)
		}
		switch value := value.(type) {
		case map[string]any:
			if _, ok := value["$ref"].(string); ok {
				ref, err := navigateRef(root, path)
				if errors.Is(err, ErrBadQuery) {
					return nil
				}
				if err != nil {
					return fmt.Errorf("reference description %s: %w", path, err)
				}
				if ref.Ref != "" {
					description, _ := value["description"].(string)
					d.refDescriptions[ref] = description
					return nil
				}
			}
			for key, child := range value {
				if err := visit(child, path+"/"+escapePointer(key), depth+1); err != nil {
					return err
				}
			}
		case []any:
			for i, child := range value {
				if err := visit(child, path+"/"+strconv.Itoa(i), depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return visit(raw, "", 0)
}
