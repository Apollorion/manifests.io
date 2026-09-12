package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/TheOutdoorProgrammer/manifests.io/internal/schema"
)

type staticFakeCatalog struct{ fakeCatalog }

func (staticFakeCatalog) StaticDocument(q schema.Query) (schema.StaticDocument, error) {
	return schema.StaticDocument{Item: q.Item, Version: q.Version, Resources: map[string]string{}, Nodes: map[string]schema.StaticNode{}}, nil
}

func TestStaticExportCompletesMetadataAndErrorPages(t *testing.T) {
	var output bytes.Buffer
	if err := ExportStatic(t.Context(), staticFakeCatalog{}, "https://docs.example/", &output); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	var kinds []string
	pages, complete := 0, false
	for {
		var record struct {
			Kind   string          `json:"kind"`
			Head   string          `json:"head"`
			Data   json.RawMessage `json:"data"`
			Status int             `json:"status"`
		}
		if err := decoder.Decode(&record); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, record.Kind)
		if record.Kind == "page" {
			pages++
			var data string
			if err := json.Unmarshal(record.Data, &data); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(record.Head, `rel="canonical" href="https://docs.example/`) || !strings.Contains(record.Head, `property="og:image"`) {
				t.Fatal("static page lacks complete SEO metadata")
			}
			if strings.Contains(data, "</script>") {
				t.Fatal("static embedded JSON can escape its script element")
			}
			var page schema.Page
			if err := json.Unmarshal([]byte(data), &page); err != nil {
				t.Fatal(err)
			}
			if record.Status != 200 && (page.Error == "" || !strings.Contains(record.Head, `name="robots" content="noindex"`)) {
				t.Fatal("static error page lacks recovery data or noindex")
			}
		}
		complete = record.Kind == "complete"
	}
	if pages != 5 || !complete || kinds[0] != "root" {
		t.Fatalf("incomplete export: pages=%d kinds=%v", pages, kinds)
	}
}

func TestStaticExportRejectsInvalidOrigins(t *testing.T) {
	for _, site := range []string{"javascript:alert(1)", "https://user:pass@example.com", "https://example.com/path", "https://example.com?query=1"} {
		if err := ExportStatic(t.Context(), staticFakeCatalog{}, site, io.Discard); err == nil {
			t.Fatalf("accepted invalid static origin %q", site)
		}
	}
}
