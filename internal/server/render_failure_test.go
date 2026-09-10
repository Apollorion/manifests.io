package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheOutdoorProgrammer/manifests.io/internal/observability"
	"github.com/TheOutdoorProgrammer/manifests.io/internal/schema"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestRenderFailureReporting(t *testing.T) {
	// Use the production privacy filter as well as a real embedded renderer.
	for _, key := range []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"} {
		t.Setenv(key, "")
	}
	logger, provider, propagator := slog.Default(), otel.GetTracerProvider(), otel.GetTextMapPropagator()
	t.Cleanup(func() {
		slog.SetDefault(logger)
		otel.SetTracerProvider(provider)
		otel.SetTextMapPropagator(propagator)
	})
	var logs bytes.Buffer
	shutdown, err := observability.SetupWithWriter(t.Context(), "test", &logs)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })
	exporter := tracetest.NewInMemoryExporter()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tracer)
	t.Cleanup(func() { _ = tracer.Shutdown(context.Background()) })

	for _, tc := range []struct {
		name, script, kind string
		canceled, expired  bool
	}{
		{name: "already canceled", canceled: true, kind: "canceled"},
		{name: "canceled during execution", script: `cancelRequest(); while (true) {}`, kind: "canceled"},
		{name: "deadline remains an error", expired: true, kind: "deadline_exceeded"},
		{name: "exception remains an error", script: `throw new Error("private-render-marker");`, kind: "javascript_exception"},
		{name: "stack limit remains an error", script: `function recurse() { return recurse(); } recurse();`, kind: "stack_overflow"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs.Reset()
			exporter.Reset()
			s := testServer(t)
			file := filepath.Join(t.TempDir(), "renderer.js")
			source := `var ManifestsRenderer = {renderPage: function(data) { if (JSON.parse(data).error) {` + tc.script + `} return "<main>Recovered</main>"; }};`
			if err := os.WriteFile(file, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			s.renderer, err = newReactRenderer(file)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			if tc.expired {
				var stop context.CancelFunc
				ctx, stop = context.WithDeadline(ctx, time.Now().Add(-time.Second))
				defer stop()
			}
			for range cap(s.renderer.workers) {
				worker := <-s.renderer.workers
				if err := worker.vm.Set("cancelRequest", cancel); err != nil {
					t.Fatal(err)
				}
				s.renderer.workers <- worker
			}
			request := httptest.NewRequest(http.MethodGet, "/kubernetes/1.34/Pod?path=private-render-marker", nil).WithContext(ctx)
			response := httptest.NewRecorder()
			s.servePage(response, request, http.StatusOK, schema.Page{Error: "private-render-marker"}, true)
			spans := exporter.GetSpans()
			if len(spans) != 1 {
				t.Fatalf("got %d render spans", len(spans))
			}
			found := false
			for _, attr := range spans[0].Attributes {
				if attr.Key == "render.failure" && attr.Value.AsString() == tc.kind {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing failure kind %q: %v", tc.kind, spans[0].Attributes)
			}
			var record map[string]any
			if err := json.Unmarshal(logs.Bytes(), &record); err != nil {
				t.Fatal(err)
			}
			if response.Code != http.StatusServiceUnavailable || record["level"] != "ERROR" || record["msg"] != "React rendering failed" || record["render.failure"] != tc.kind || spans[0].Status.Code != codes.Error {
				t.Fatalf("failure not reported: response=%d record=%v span=%v", response.Code, record, spans[0].Status)
			}
			encoded, err := json.Marshal(spans)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(logs.String()+string(encoded)+response.Body.String(), "private-render-marker") {
				t.Fatal("render failure exposed private data")
			}
			// Exercise both pool slots, including a replacement after a failed VM.
			for range 4 {
				body, err := s.renderer.render(t.Context(), []byte(`{}`))
				if err != nil || string(body) != "<main>Recovered</main>" {
					t.Fatalf("pool did not recover: %s %v", body, err)
				}
			}
		})
	}
}
