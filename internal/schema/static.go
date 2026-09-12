package schema

type StaticVariant struct {
	Title string `json:"title"`
	Node  string `json:"node"`
}

type StaticNode struct {
	Query     Query             `json:"-"`
	Canonical string            `json:"canonical"`
	Edges     map[string]string `json:"edges,omitempty"`
	OneOf     []StaticVariant   `json:"oneOf,omitempty"`
}

type StaticDocument struct {
	Item      string                `json:"item"`
	Version   string                `json:"version"`
	Resources map[string]string     `json:"resources"`
	Nodes     map[string]StaticNode `json:"nodes"`
}

func (c *Catalog) StaticDocument(q Query) (StaticDocument, error) {
	graph := StaticDocument{Item: q.Item, Version: q.Version, Resources: make(map[string]string), Nodes: make(map[string]StaticNode)}
	d := c.documents[q.Item+"/"+q.Version]
	if d == nil {
		return graph, ErrNotFound
	}
	for name, ref := range d.schemas {
		if ref != nil && ref.Value != nil {
			graph.Resources[name] = d.nodes[ref.Value].identity()
		}
	}
	for name, loc := range d.aliases {
		if _, exists := graph.Resources[name]; exists {
			continue
		}
		selected, err := navigate(d.schemas[loc.resource].Value, loc.path)
		if err != nil {
			return graph, err
		}
		graph.Resources[name] = d.nodes[selected].identity()
	}
	for selected, loc := range d.nodes {
		query := Query{Item: q.Item, Version: q.Version, Resource: loc.resource, Pointer: loc.path}
		node := StaticNode{Query: query, Canonical: Href(query), Edges: make(map[string]string)}
		for _, child := range schemaChildren(selected) {
			if child.ref != nil && child.ref.Value != nil {
				node.Edges[child.pointer] = d.nodes[child.ref.Value].identity()
			}
		}
		for _, ref := range selected.OneOf {
			if ref != nil && ref.Value != nil {
				node.OneOf = append(node.OneOf, StaticVariant{Title: ref.Value.Title, Node: d.nodes[ref.Value].identity()})
			}
		}
		graph.Nodes[loc.identity()] = node
	}
	return graph, nil
}
