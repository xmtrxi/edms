package observability

import (
	"context"
	"os"
	"strconv"

	"github.com/labstack/echo/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "github.com/xmtrxi/edms"

// TracingEnabled reports whether an OTLP endpoint is configured through the
// standard OTEL_EXPORTER_OTLP_* variables.
func TracingEnabled() bool {
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" || os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != ""
}

// SetupTracing exports traces over OTLP/HTTP when an endpoint is configured.
// The exporter reads the standard OTEL_* variables, including headers and
// sampling (OTEL_TRACES_SAMPLER). The returned function flushes on shutdown.
func SetupTracing(ctx context.Context, version string) (func(context.Context) error, error) {
	if !TracingEnabled() {
		return func(context.Context) error { return nil }, nil
	}
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	name := os.Getenv("OTEL_SERVICE_NAME")
	if name == "" {
		name = "edms-api"
	}
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL,
		semconv.ServiceName(name), semconv.ServiceVersion(version)))
	if err != nil {
		return nil, err
	}
	provider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter), sdktrace.WithResource(res))
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return provider.Shutdown, nil
}

// TracingMiddleware starts a server span per request, continuing a trace the
// caller started. Without SetupTracing it records nothing.
func TracingMiddleware() echo.MiddlewareFunc {
	tracer := otel.Tracer(tracerName)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			req := c.Request()
			ctx := otel.GetTextMapPropagator().Extract(req.Context(), propagation.HeaderCarrier(req.Header))
			route := c.Path()
			if route == "" {
				route = req.URL.Path
			}
			ctx, span := tracer.Start(ctx, req.Method+" "+route,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					semconv.HTTPRequestMethodKey.String(req.Method),
					semconv.HTTPRoute(route),
					semconv.URLPath(req.URL.Path),
					attribute.String("client.address", c.RealIP()),
				))
			defer span.End()
			c.SetRequest(req.WithContext(ctx))

			err := next(c)
			status := statusOf(c, err)
			span.SetAttributes(semconv.HTTPResponseStatusCode(status))
			if status >= 500 {
				span.SetStatus(codes.Error, strconv.Itoa(status))
			}
			if err != nil {
				span.RecordError(err)
			}
			return err
		}
	}
}

// StartSpan begins an internal span, for background work such as tasks.
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, name, trace.WithAttributes(attrs...))
}
