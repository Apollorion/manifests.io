package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/TheOutdoorProgrammer/manifests.io/internal/schema"
)

func TestReactRendererBundle(t *testing.T) {
	filename := "../../frontend/dist-render/renderer.js"
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		t.Skip("run npm --prefix frontend run build for React integration")
	}
	r, err := newReactRenderer(filename)
	if err != nil {
		t.Fatal(err)
	}
	p := schema.Page{Item: "kubernetes", Version: "1.34", Resource: "Pod", Title: "Deployment.spec", Canonical: "/kubernetes/1.34/Pod", Catalog: []schema.Product{{Name: "kubernetes", Versions: []string{"1.34", "1.33"}}}, Resources: []schema.Row{{Name: "self", Circular: true}, {Name: "other", Href: "/kubernetes/1.34/Other?path=Deployment.spec.other"}, {Name: "unsafe", Description: "<img src=x onerror=alert(1)>"}}}
	data, _ := json.Marshal(p)
	html, err := r.render(context.Background(), data)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Deployment.spec", "Circular reference", "&lt;img", "Deployment.spec.other"} {
		if !strings.Contains(string(html), want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(string(html), "<img src=x") {
		t.Fatal("description was executable HTML")
	}
	if !bytes.Equal(html, nodeRenderedPage(t, data)) {
		t.Fatal("embedded React output differs from the build renderer")
	}
}

func nodeRenderedPage(t testing.TB, data []byte) []byte {
	t.Helper()
	command := exec.CommandContext(t.Context(), "node", "--input-type=module", "-e", `import {render} from './frontend/dist-server/entry-server.js'; let data=''; for await (const chunk of process.stdin) data+=chunk; process.stdout.write(render(JSON.parse(data)));`)
	command.Dir = "../.."
	command.Stdin = bytes.NewReader(data)
	expected, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Node reference renderer: %s %v", expected, err)
	}
	return expected
}

func TestRendererCancellationAndConcurrentIsolation(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "renderer.js")
	if err := os.WriteFile(filename, []byte(`var count=0;var ManifestsRenderer={renderPage:function(value){count++;if(value==="loop"){while(true){}}return value+":"+count;}};`), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := newReactRenderer(filename)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := r.render(ctx, []byte("loop")); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("renderer did not preserve deadline: %v", err)
	}
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			result, err := r.render(context.Background(), []byte("next"))
			if err != nil || !strings.HasPrefix(string(result), "next:") {
				t.Errorf("worker did not recover: %s %v", result, err)
			}
		})
	}
	wg.Wait()
}
