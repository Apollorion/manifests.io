package schema

import (
	"encoding/json"
	"slices"

	"github.com/getkin/kin-openapi/openapi3"
)

const maxSchemaVisits = 3

func (loc location) identity() string { return loc.resource + "#" + loc.path }

// Tarjan's components distinguish recursive schemas from shared, acyclic types.
func (d *document) indexCycles(edges map[*openapi3.Schema][]*openapi3.Schema) {
	d.cyclic = make(map[string]*openapi3.Schema)
	indices, low := make(map[*openapi3.Schema]int), make(map[*openapi3.Schema]int)
	onStack := make(map[*openapi3.Schema]bool)
	var stack []*openapi3.Schema
	next := 0
	var visit func(*openapi3.Schema)
	visit = func(s *openapi3.Schema) {
		next++
		indices[s], low[s] = next, next
		stack = append(stack, s)
		onStack[s] = true
		for _, child := range edges[s] {
			if indices[child] == 0 {
				visit(child)
				low[s] = min(low[s], low[child])
			} else if onStack[child] {
				low[s] = min(low[s], indices[child])
			}
		}
		if low[s] != indices[s] {
			return
		}
		var component []*openapi3.Schema
		for {
			child := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[child] = false
			component = append(component, child)
			if child == s {
				break
			}
		}
		if len(component) > 1 || slices.Contains(edges[s], s) {
			for _, child := range component {
				d.cyclic[d.nodes[child].identity()] = child
			}
		}
	}
	for s := range d.nodes {
		if indices[s] == 0 {
			visit(s)
		}
	}
}

func (d *document) trackVisits(q *Query, selected *openapi3.Schema) error {
	counts := make(map[string]int)
	if q.Trail != "" {
		if len(q.Trail) > 4096 || json.Unmarshal([]byte(q.Trail), &counts) != nil || counts == nil || len(counts) > 128 {
			return ErrBadQuery
		}
	}
	q.visits = make(map[*openapi3.Schema]int)
	for id, count := range counts {
		if count < 1 || count > maxSchemaVisits {
			return ErrBadQuery
		}
		if s := d.cyclic[id]; s != nil {
			q.visits[s] = count
		} else {
			delete(counts, id)
		}
	}
	id := d.nodes[selected].identity()
	if d.cyclic[id] == selected {
		if q.visits[selected] >= maxSchemaVisits {
			return ErrBadQuery
		}
		q.visits[selected]++
		counts[id] = q.visits[selected]
	}
	q.Trail = ""
	if len(counts) > 0 {
		data, err := json.Marshal(counts)
		if err != nil {
			return err
		}
		q.Trail = string(data)
	}
	return nil
}

func (d *document) navigationLink(label string, target *openapi3.Schema, q Query) Link {
	if q.visits[target] >= maxSchemaVisits {
		return Link{Label: label, Circular: true}
	}
	return Link{Label: label, Href: Href(d.canonicalQuery(target, q))}
}
