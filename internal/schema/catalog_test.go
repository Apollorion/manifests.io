package schema

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

var corpus = sync.OnceValues(func() (*Catalog, error) { return Load("../..") })

// Captured before removing generated definitions to detect broken legacy URLs.
var legacyCorpus = map[string]struct {
	count int
	hash  string
}{
	"certmanager/1.7":                {144, "e240662a680a501186804576b9ddc78aa0d29d46924a27a359e38e6381c9aaa8"},
	"certmanager/1.14":               {171, "64e109d8e63b9fe86e01ae4b35739f6aa19c492f54369fe85118bb1f219fd582"},
	"cosign policy-controller/0.5.7": {31, "dd503b801ea520c1d5b4ad30d316e76a23a7cd44ca3f72a94366012e383eff11"},
	"flagger/1.19.0":                 {55, "dd9d6ad2acac7942b859136767b264ce1ccf12c7c7c4e86f87b51287e7e838ce"},
	"flux/0.27.3":                    {208, "164ab8cc04bfae038cb482f12f18cc98423b97565203f76d9c0d36fba22c2465"},
	"flux/0.31.2":                    {247, "135cf45272c1ad29d794944d04b63bd53fd60298bb228415395f7712bff77942"},
	"flux/2.0.1":                     {289, "d89849171e7913e2fc853d2070b7098e585723682466f25847efed405faf9eff"},
	"gateway api/1.0.0 standard":     {48, "190a87696c7a6c3ce8db6b2a34a777b35d33a1b49b103500a8f249fd91d39ca6"},
	"gateway api/1.0.0 experimental": {85, "4ada8b9bca93803757e39934808cf4dbb9d8ac769c5c5cf1623509d93e449096"},
	"gateway api/1.1.0 standard":     {62, "fc213d11e684b61450170be587212e955010228412e56865fa05e3c5133d8717"},
	"gateway api/1.1.0 experimental": {101, "e8ce2671b5d2dbeef93ec2b0d75c33ebe91656f1c9c7b1575b64c3f7f7cff7c2"},
	"gateway api/1.1.1 standard":     {62, "fc213d11e684b61450170be587212e955010228412e56865fa05e3c5133d8717"},
	"gateway api/1.1.1 experimental": {101, "e8ce2671b5d2dbeef93ec2b0d75c33ebe91656f1c9c7b1575b64c3f7f7cff7c2"},
	"gateway api/1.2.0 standard":     {55, "2f84dd5e1ef228274d4dc65a3d1e457b197ea14af283767b5d565e1f91523ece"},
	"gateway api/1.2.0 experimental": {95, "d0d49e01c50bd4ea947aaab4e15cf2c0664304bba8eee740046192c29fb0296a"},
	"istio/1.13.3":                   {150, "df7fa9bc10973097fdfdafb6f5f0f3b9b4845240e10870da46618ea62a2fde19"},
	"kubernetes/1.29":                {577, "88e055f640e0a9ee9b1b3aa01a10383bed38107063e18a27299fbe8abeb2a0bb"},
	"kubernetes/1.30":                {626, "cc397693092869dbd1618b37ae93155e291d7d54dffda403d6d27938e12cee97"},
	"kubernetes/1.31":                {635, "2bc164baae41ec4c708ec2bd9814341f71767ed325eb7c15b5c18c2d3d956846"},
	"kubernetes/1.32":                {638, "e3a464e712622d9b0325f2241bba2b4999a7d9b9bf2c39fc6712f1b58f0656d6"},
	"kubernetes/1.33":                {707, "0d2a248dcbc6ce78ea5fa606590b1781396330375129a714be6fb4c5536278a8"},
	"kubernetes/1.34":                {730, "fd86cb1c571dd14e5190d9f2673a1bf7f090aceaffae746d4113eecd2fb0f71b"},
	"opa gatekeeper/3.14.0":          {235, "14cd06c6817dba9c937bde198c1babbc4f0392806dba1cf813b0001d44839b7d"},
	"prometheus operator/0.71.2":     {553, "0eb32fd411848206c0806d10a1061dab73e399478c03de47dbbfd922d2948a6b"},
	"rancher/2.8":                    {82, "1c84ca1b956056c47d915c40eef886008284474b0747942f7750d5e1c385a30a"},
	"spacelift operator/0.1.0":       {56, "349d72ec158c157a8e1bf20e39488edb7ca30460a2b141f59d7692c969e46e2f"},
	"spacelift workerpool/0.0.21":    {192, "c50605e00ae7d6b302151e0be3e30fc0cfb058bb3b6eeb49d143794c78e38a97"},
	"spacelift workerpool/0.0.41":    {239, "94b556d9a7c0792b227963cc4659ad71c6c515b144dfe66c650e36528380e70c"},
}

func TestCorpusAndLegacyRoutes(t *testing.T) {
	c, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Products()) != 12 {
		t.Fatalf("products: %d", len(c.Products()))
	}
	count := 0
	for key := range legacyCorpus {
		if c.documents[key] == nil {
			t.Errorf("missing historical version %s", key)
		}
	}
	for _, product := range c.Products() {
		for _, version := range product.Versions {
			q := Query{Item: product.Name, Version: version}
			page, err := c.Page(q)
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Resources) == 0 {
				t.Errorf("empty index %v", q)
			}
			actual := make(map[string]bool)
			doc := c.documents[product.Name+"/"+version]
			for name := range doc.schemas {
				actual[name] = true
			}
			for name := range doc.aliases {
				actual[name] = true
			}
			newHash := sha256.Sum256([]byte(strings.Join(sortedKeys(actual), "\n")))
			expected, ok := legacyCorpus[product.Name+"/"+version]
			if ok && (len(actual) != expected.count || fmt.Sprintf("%x", newHash) != expected.hash) {
				t.Errorf("legacy URLs changed for %v", q)
			}
			for name := range actual {
				q.Resource = name
				if _, err := c.Page(q); err != nil {
					t.Errorf("legacy %v: %v", q, err)
				}
				count++
			}
		}
	}
	t.Logf("validated %d legacy definition URLs", count)
}

func TestFiniteCanonicalRoutes(t *testing.T) {
	c, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	routes := c.Routes()
	seen := map[string]string{}
	for _, q := range routes {
		p, err := c.Page(q)
		if err != nil {
			t.Fatalf("%v: %v", q, err)
		}
		if seen[p.Canonical] != "" {
			t.Fatalf("duplicate canonical %s from %v and %s", p.Canonical, q, seen[p.Canonical])
		}
		seen[p.Canonical] = q.String()
	}
	q := Query{Item: "kubernetes", Version: "1.34", Resource: "io.k8s.api.core.v1.Pod", Pointer: "/properties/spec/properties/containers/items"}
	p, err := c.Page(q)
	if err != nil {
		t.Fatal(err)
	}
	if p.Canonical != "/kubernetes/1.34/io.k8s.api.core.v1.Container" {
		t.Fatalf("ref boundary canonical: %s", p.Canonical)
	}
	for _, p := range c.Products() {
		for _, version := range p.Versions {
			d := c.documents[p.Name+"/"+version]
			for alias := range d.aliases {
				page, err := c.Page(Query{Item: p.Name, Version: version, Resource: alias})
				if err != nil {
					t.Fatal(err)
				}
				if seen[page.Canonical] == "" {
					t.Errorf("legacy alias not prerendered: %s", alias)
				}
			}
		}
	}
	t.Logf("validated %d finite canonical routes", len(routes))
}

func TestNavigation(t *testing.T) {
	c, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	q := Query{Item: "kubernetes", Version: "1.34", Resource: "io.k8s.api.core.v1.Pod"}
	p, err := c.Page(q)
	if err != nil {
		t.Fatal(err)
	}
	var spec Row
	for _, row := range p.Resources {
		if row.Name == "spec" {
			spec = row
		}
	}
	if spec.Href != "/kubernetes/1.34/io.k8s.api.core.v1.PodSpec?path=Pod.spec" || spec.Type != "PodSpec" {
		t.Fatalf("spec row: %+v", spec)
	}
	q.Pointer = "/properties/spec/properties/containers/items"
	p, err = c.Page(q)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range p.Resources {
		if row.Name == "name" {
			found = row.Required
		}
	}
	if !found {
		t.Fatal("container name must be required")
	}
	for _, path := range []string{"oops", "/properties", "/properties/~2", "/oneOf/-1"} {
		q.Pointer = path
		if _, err := c.Page(q); !errors.Is(err, ErrBadQuery) {
			t.Errorf("path %q: %v", path, err)
		}
	}
	q.Pointer = "/properties/nonexistent"
	if _, err := c.Page(q); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	q = Query{Item: "gateway api", Version: "1.2.0 standard", Resource: "io.k8s.networking.gateway.v1.Gateway", Pointer: "/properties/spec/properties/addresses"}
	p, err = c.Page(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Variants) != 2 {
		t.Fatal("array item alternatives missing")
	}
}

func TestReferenceNavigationUsesTargetURLs(t *testing.T) {
	c, err := corpus()
	if err != nil {
		t.Fatal(err)
	}
	q := Query{Item: "kubernetes", Version: "1.34", Resource: "io.k8s.api.apps.v1.Deployment"}
	context := "Deployment"
	for _, step := range []struct{ field, target string }{
		{"spec", "io.k8s.api.apps.v1.DeploymentSpec"},
		{"template", "io.k8s.api.core.v1.PodTemplateSpec"},
		{"spec", "io.k8s.api.core.v1.PodSpec"},
		{"containers", "io.k8s.api.core.v1.Container"},
	} {
		page, err := c.Page(q)
		if err != nil {
			t.Fatal(err)
		}
		var href string
		for _, row := range page.Resources {
			if row.Name == step.field {
				href = row.Href
			}
		}
		context += "." + step.field
		if want := "/kubernetes/1.34/" + step.target + "?path=" + context; href != want {
			t.Fatalf("%s.%s links to %q, want %q", q.Resource, step.field, href, want)
		}
		q = queryFromHref(t, href)
		target, err := c.Page(q)
		if err != nil || target.Title != context || target.Path != context || target.Pointer != "" {
			t.Fatalf("target lost traversal context: page=%+v error=%v", target, err)
		}
	}
	page, err := c.Page(Query{Item: "kubernetes", Version: "1.34", Resource: "io.k8s.api.core.v1.PodSpec", Path: "Deployment.spec.template.spec"})
	if err != nil {
		t.Fatal(err)
	}
	if page.Resource != "io.k8s.api.core.v1.PodSpec" || page.Pointer != "" || page.Title != "Deployment.spec.template.spec" {
		t.Fatalf("contextual URL did not resolve to PodSpec: resource=%s path=%s title=%s", page.Resource, page.Path, page.Title)
	}
}

func TestOpenAPIReaders(t *testing.T) {
	for _, data := range []string{
		`{"swagger":"2.0","definitions":{"Thing":{"type":"object","properties":{"self":{"$ref":"#/definitions/Thing"}}}}}`,
		`{"openapi":"3.0.3","components":{"schemas":{"Thing":{"type":"object","properties":{"self":{"$ref":"#/components/schemas/Thing"}}}}}}`,
	} {
		d, err := readOpenAPI([]byte(data))
		if err != nil {
			t.Fatal(err)
		}
		if d.schemas["Thing"].Value.Properties["self"].Value != d.schemas["Thing"].Value {
			t.Fatal("cycle not resolved")
		}
	}
	for _, ref := range []string{"https://example.com/schema.json", "file:///etc/passwd", "../../secret.json"} {
		data := `{"openapi":"3.0.3","components":{"schemas":{"Thing":{"$ref":"` + ref + `"}}}}`
		if _, err := readOpenAPI([]byte(data)); err == nil {
			t.Errorf("accepted external ref %s", ref)
		}
	}
}
