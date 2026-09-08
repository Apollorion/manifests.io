package observability

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func Setup(ctx context.Context, version string) (func(context.Context) error, error) {
	return SetupWithWriter(ctx, version, os.Stdout)
}

func SetupWithWriter(ctx context.Context, version string, output io.Writer) (func(context.Context) error, error) {
	local := slog.NewJSONHandler(output, nil)
	slog.SetDefault(slog.New(correlatedHandler{Handler: local}))
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	// Exporter errors can contain credentials in their endpoint URLs.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(error) { slog.New(local).Warn("telemetry export failed") }))
	res := resource.NewSchemaless(attribute.String("service.name", "manifests.io"), attribute.String("service.version", version))
	var shutdowns []func(context.Context) error
	shutdown := func(ctx context.Context) error {
		var failures []error
		for _, stop := range shutdowns {
			failures = append(failures, stop(ctx))
		}
		return errors.Join(failures...)
	}
	if enabled("TRACES") {
		exporter, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, errors.New("initialize trace exporter")
		}
		provider := sdktrace.NewTracerProvider(sdktrace.WithResource(res), sdktrace.WithBatcher(privateExporter{SpanExporter: exporter}))
		otel.SetTracerProvider(provider)
		shutdowns = append(shutdowns, provider.Shutdown)
	} else {
		provider := sdktrace.NewTracerProvider(sdktrace.WithResource(res), sdktrace.WithSampler(sdktrace.NeverSample()))
		otel.SetTracerProvider(provider)
		shutdowns = append(shutdowns, provider.Shutdown)
	}
	if enabled("LOGS") {
		exporter, err := otlploghttp.New(ctx)
		if err != nil {
			_ = shutdown(ctx)
			return nil, errors.New("initialize log exporter")
		}
		provider := sdklog.NewLoggerProvider(sdklog.WithResource(res), sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)))
		slog.SetDefault(slog.New(correlatedHandler{Handler: slog.NewMultiHandler(local, otelslog.NewHandler("manifests.io", otelslog.WithLoggerProvider(provider)))}))
		shutdowns = append(shutdowns, provider.Shutdown)
	}
	return shutdown, nil
}

func enabled(signal string) bool {
	return os.Getenv("OTEL_SDK_DISABLED") != "true" && os.Getenv("OTEL_"+signal+"_EXPORTER") != "none" && (os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" || os.Getenv("OTEL_EXPORTER_OTLP_"+signal+"_ENDPOINT") != "")
}

type correlatedHandler struct{ slog.Handler }

func (h correlatedHandler) Handle(ctx context.Context, record slog.Record) error {
	safe := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		if safeLogKey(attr.Key) {
			safe.AddAttrs(attr)
		}
		return true
	})
	if span := trace.SpanContextFromContext(ctx); span.IsValid() {
		safe.AddAttrs(slog.String("trace_id", span.TraceID().String()), slog.String("span_id", span.SpanID().String()))
	}
	return h.Handler.Handle(ctx, safe)
}

func (h correlatedHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	var safe []slog.Attr
	for _, attr := range attrs {
		if safeLogKey(attr.Key) {
			safe = append(safe, attr)
		}
	}
	return correlatedHandler{Handler: h.Handler.WithAttrs(safe)}
}

func (h correlatedHandler) WithGroup(name string) slog.Handler {
	return correlatedHandler{Handler: h.Handler.WithGroup(name)}
}

func safeLogKey(key string) bool {
	switch key {
	case "http.request.method", "http.route", "http.response.status_code", "duration_ms", "version", "port", "documents", "definitions", "schema.product", "schema.operation", "schema.updates", "render.failure":
		return true
	}
	return false
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := otel.Tracer("manifests.io/http").Start(ctx, "HTTP request", trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()
		r = r.WithContext(ctx)
		response := &responseCapture{ResponseWriter: w}
		defer func() {
			route := r.Pattern
			if route == "" {
				route = "unmatched"
			}
			status := response.status
			if status == 0 {
				status = http.StatusOK
			}
			if failure := recover(); failure != nil {
				status = http.StatusInternalServerError
				slog.ErrorContext(ctx, "request panicked", "http.route", route)
				if response.status == 0 {
					http.Error(response, "Internal server error", status)
				}
			}
			method := r.Method
			if !strings.Contains("|GET|HEAD|POST|PUT|PATCH|DELETE|OPTIONS|CONNECT|TRACE|", "|"+method+"|") {
				method = "OTHER"
			}
			span.SetName(route)
			span.SetAttributes(attribute.String("http.request.method", method), attribute.String("http.route", route), attribute.Int("http.response.status_code", status))
			if status >= 500 {
				span.SetStatus(codes.Error, "")
			}
			slog.InfoContext(ctx, "request completed", "http.request.method", method, "http.route", route, "http.response.status_code", status, "duration_ms", time.Since(started).Milliseconds())
		}()
		next.ServeHTTP(response, r)
	})
}

type responseCapture struct {
	http.ResponseWriter
	status int
}

func (w *responseCapture) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *responseCapture) WriteHeader(status int) {
	if status >= 100 && status < 200 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *responseCapture) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

type privateExporter struct{ sdktrace.SpanExporter }

func (e privateExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	safe := make([]sdktrace.ReadOnlySpan, len(spans))
	for i, span := range spans {
		safe[i] = privateSpan{ReadOnlySpan: span}
	}
	return e.SpanExporter.ExportSpans(ctx, safe)
}

type privateSpan struct{ sdktrace.ReadOnlySpan }

func (s privateSpan) Name() string {
	switch s.ReadOnlySpan.Name() {
	case "catalog.load", "schema.update", "schema.discover", "schema.download", "schema.validate", "schema.install", "react.render":
		return s.ReadOnlySpan.Name()
	}
	for _, attr := range s.Attributes() {
		if attr.Key == "http.route" {
			return attr.Value.AsString()
		}
	}
	return "service operation"
}

func (s privateSpan) Attributes() []attribute.KeyValue {
	var safe []attribute.KeyValue
	for _, attr := range s.ReadOnlySpan.Attributes() {
		switch attr.Key {
		case "http.request.method", "http.route", "http.response.status_code", "schema.product", "schema.operation", "schema.updates", "render.failure":
			safe = append(safe, attr)
		}
	}
	return safe
}

func (s privateSpan) Events() []sdktrace.Event {
	var events []sdktrace.Event
	for _, event := range s.ReadOnlySpan.Events() {
		name := "service event"
		if event.Name == "exception" {
			name = "exception"
		}
		events = append(events, sdktrace.Event{Name: name, Time: event.Time})
	}
	return events
}

func (s privateSpan) Links() []sdktrace.Link { return nil }

func (s privateSpan) Status() sdktrace.Status {
	return sdktrace.Status{Code: s.ReadOnlySpan.Status().Code}
}
