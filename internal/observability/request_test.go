package observability

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	logspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	tracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

func preserveTelemetry(t *testing.T) {
	t.Helper()
	logger, provider, propagator, errorHandler, request := slog.Default(), otel.GetTracerProvider(), otel.GetTextMapPropagator(), otel.GetErrorHandler(), exporters.Load()
	t.Cleanup(func() {
		slog.SetDefault(logger)
		otel.SetTracerProvider(provider)
		otel.SetTextMapPropagator(propagator)
		otel.SetErrorHandler(errorHandler)
		exporters.Store(request)
	})
}

func TestRequestsExportBeforeResponseCompletion(t *testing.T) {
	for _, tc := range []struct {
		name, method, body string
		status             int
		panic              bool
	}{
		{name: "large body", method: http.MethodGet, status: 200, body: strings.Repeat("schema", 1<<16)},
		{name: "single byte", method: http.MethodGet, status: 200, body: "x"},
		{name: "empty", method: http.MethodGet, status: 200},
		{name: "head", method: http.MethodHead, status: 200},
		{name: "not modified", method: http.MethodGet, status: 304},
		{name: "no content", method: http.MethodGet, status: 204},
		{name: "panic", method: http.MethodGet, status: 500, body: "Internal server error\n", panic: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			preserveTelemetry(t)
			var traces, logs atomic.Bool
			collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				switch r.URL.Path {
				case "/v1/traces":
					request := new(tracepb.ExportTraceServiceRequest)
					if err := proto.Unmarshal(body, request); err != nil {
						t.Error(err)
					}
					var root, child bool
					for _, resource := range request.ResourceSpans {
						for _, scope := range resource.ScopeSpans {
							for _, span := range scope.Spans {
								root = root || span.Name == "GET /page"
								child = child || span.Name == "react.render"
							}
						}
					}
					if !root || !child {
						t.Errorf("request batch missing root or child: root=%v child=%v", root, child)
					}
					traces.Store(root && child)
				case "/v1/logs":
					request := new(logspb.ExportLogsServiceRequest)
					if err := proto.Unmarshal(body, request); err != nil {
						t.Error(err)
					}
					for _, resource := range request.ResourceLogs {
						for _, scope := range resource.ScopeLogs {
							for _, record := range scope.LogRecords {
								if record.Body.GetStringValue() == "request completed" && len(record.TraceId) == 16 {
									logs.Store(true)
								}
							}
						}
					}
				}
				w.Header().Set("Content-Type", "application/x-protobuf")
			}))
			t.Cleanup(collector.Close)
			for _, key := range []string{"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "OTEL_EXPORTER_OTLP_HEADERS", "OTEL_EXPORTER_OTLP_TRACES_HEADERS", "OTEL_EXPORTER_OTLP_LOGS_HEADERS", "OTEL_SDK_DISABLED", "OTEL_TRACES_EXPORTER", "OTEL_LOGS_EXPORTER"} {
				t.Setenv(key, "")
			}
			t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.URL)
			t.Setenv("OTEL_BSP_SCHEDULE_DELAY", "60000")
			t.Setenv("OTEL_BLRP_SCHEDULE_DELAY", "60000")
			shutdown, err := SetupWithWriter(t.Context(), "test", io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = shutdown(context.Background()) })
			response := &completionObserver{ResponseRecorder: httptest.NewRecorder(), length: len(tc.body), check: func() {
				if !traces.Load() || !logs.Load() {
					t.Error("response completed before exporting server span and completion log")
				}
			}}
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r.Pattern = "GET /page"
				_, span := otel.Tracer("test").Start(r.Context(), "react.render")
				defer span.End()
				if tc.panic {
					panic("synthetic failure")
				}
				w.Header().Set("Content-Length", strconv.Itoa(len(tc.body)))
				w.WriteHeader(tc.status)
				mid := len(tc.body) / 2
				_, _ = io.WriteString(w, tc.body[:mid])
				_, _ = io.WriteString(w, tc.body[mid:])
			})
			Middleware(handler).ServeHTTP(response, httptest.NewRequest(tc.method, "/page", nil))
			if !response.completed || response.Code != tc.status || response.Body.String() != tc.body {
				t.Fatalf("response changed: completed=%v status=%d bytes=%d", response.completed, response.Code, response.Body.Len())
			}
		})
	}
}

type completionObserver struct {
	*httptest.ResponseRecorder
	length    int
	completed bool
	check     func()
}

func (w *completionObserver) WriteHeader(status int) {
	if w.length == 0 {
		w.check()
		w.completed = true
	}
	w.ResponseRecorder.WriteHeader(status)
}

func (w *completionObserver) Write(body []byte) (int, error) {
	if w.Body.Len()+len(body) == w.length {
		w.check()
		w.completed = true
	}
	return w.ResponseRecorder.Write(body)
}

func TestRequestFlushConcurrentBoundedAndIndependentOfCancellation(t *testing.T) {
	preserveTelemetry(t)
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(error) {}))
	started := make(chan struct{}, 2)
	var workers sync.WaitGroup
	flush := func(ctx context.Context) error {
		defer workers.Done()
		if ctx.Err() != nil {
			t.Error("client cancellation canceled telemetry export")
		}
		started <- struct{}{}
		<-ctx.Done()
		return ctx.Err()
	}
	workers.Add(2)
	exporters.Store(&requestExporters{flush: []func(context.Context) error{flush, flush}})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	finished := make(chan struct{})
	go func() {
		flushRequest(ctx)
		close(finished)
	}()
	for range 2 {
		select {
		case <-started:
		case <-time.After(requestFlushTimeout / 2):
			t.Fatal("signals did not flush concurrently")
		}
	}
	select {
	case <-finished:
	case <-time.After(2 * requestFlushTimeout):
		t.Fatal("collector outage blocked response beyond flush deadline")
	}
	workers.Wait()
}

func TestHealthProbesDoNotWaitForTelemetry(t *testing.T) {
	preserveTelemetry(t)
	exporters.Store(&requestExporters{flush: []func(context.Context) error{func(context.Context) error {
		t.Error("probe flushed telemetry")
		return nil
	}}})
	for _, path := range []string{"/healthz", "/readyz"} {
		response := httptest.NewRecorder()
		Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "ok")
		})).ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || response.Body.String() != "ok" {
			t.Fatal("probe response changed")
		}
	}
}

func TestResponseCapturePreservesLengthAndHeaderSemantics(t *testing.T) {
	underlying := httptest.NewRecorder()
	response := &responseCapture{ResponseWriter: underlying}
	response.Header().Set("Content-Length", "4")
	response.Header().Set("X-Before", "original")
	response.WriteHeader(http.StatusCreated)
	response.Header().Set("X-Before", "changed")
	if n, err := response.Write([]byte("oversized")); n != 0 || err != http.ErrContentLength {
		t.Fatalf("excess body accepted: n=%d err=%v", n, err)
	}
	if n, err := response.Write([]byte("body")); n != 4 || err != nil {
		t.Fatalf("valid body rejected: n=%d err=%v", n, err)
	}
	if underlying.Body.String() != "bod" || underlying.Code != http.StatusCreated || underlying.Header().Get("X-Before") != "original" {
		t.Fatal("response did not retain final byte or snapshot headers")
	}
	if n, err := response.Write([]byte("extra")); n != 0 || err != http.ErrContentLength {
		t.Fatalf("excess subsequent body accepted: n=%d err=%v", n, err)
	}
	response.finish()
	if underlying.Body.String() != "body" {
		t.Fatalf("response body = %q", underlying.Body.String())
	}
}

func TestCollectorOutageReleasesResponse(t *testing.T) {
	preserveTelemetry(t)
	var requests atomic.Int32
	release := make(chan struct{})
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		requests.Add(1)
		<-release
	}))
	t.Cleanup(func() {
		close(release)
		collector.Close()
	})
	for _, key := range []string{"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "OTEL_EXPORTER_OTLP_HEADERS", "OTEL_EXPORTER_OTLP_TRACES_HEADERS", "OTEL_EXPORTER_OTLP_LOGS_HEADERS", "OTEL_SDK_DISABLED", "OTEL_TRACES_EXPORTER", "OTEL_LOGS_EXPORTER"} {
		t.Setenv(key, "")
	}
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.URL)
	shutdown, err := SetupWithWriter(t.Context(), "test", io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), requestFlushTimeout)
		defer cancel()
		_ = shutdown(ctx)
	})
	response := httptest.NewRecorder()
	started := time.Now()
	Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "2")
		_, _ = io.WriteString(w, "ok")
	})).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/page", nil))
	if elapsed := time.Since(started); elapsed > 2*requestFlushTimeout {
		t.Errorf("collector outage blocked response for %s", elapsed)
	}
	if requests.Load() != 2 {
		t.Errorf("collector received %d signal requests, want 2", requests.Load())
	}
	if response.Code != http.StatusOK || response.Body.String() != "ok" {
		t.Fatal("collector outage changed response")
	}
}
