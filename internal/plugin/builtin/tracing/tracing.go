// Package tracing provides an OpenTelemetry distributed tracing plugin.
// It implements both TransportPlugin (to start spans on PreHTTP) and
// ObservabilityPlugin (to end spans with final attributes via OnTrace).
//
// Configuration (all optional):
//
//	endpoint:     OTLP HTTP endpoint, e.g. "localhost:4318"
//	              If empty, the plugin is a no-op with zero overhead.
//	service_name: Service name in traces (default: "butter")
//	insecure:     Disable TLS for the exporter connection (default: true)
package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/temikus/butter/internal/plugin"
)

// spanMetaKey is the key used to stash the active span in RequestContext.Metadata
// so that OnTrace can end it with the final request attributes.
const spanMetaKey = "_otel_span"

// Plugin implements TransportPlugin and ObservabilityPlugin to add
// distributed tracing to every request handled by Butter.
type Plugin struct {
	sdk    *sdktrace.TracerProvider // non-nil only when OTLP is configured
	tracer trace.Tracer             // non-nil only when sdk != nil
}

// New creates a new tracing plugin. Call Init before use.
func New() *Plugin { return &Plugin{} }

func (p *Plugin) Name() string { return "tracing" }

// Init sets up the OTLP HTTP exporter if an endpoint is provided.
// When endpoint is empty the plugin becomes a strict no-op.
func (p *Plugin) Init(cfg map[string]any) error {
	endpoint, _ := cfg["endpoint"].(string)
	if endpoint == "" {
		return nil
	}

	serviceName := "butter"
	if sn, ok := cfg["service_name"].(string); ok && sn != "" {
		serviceName = sn
	}

	insecure := true
	if ins, ok := cfg["insecure"].(bool); ok {
		insecure = ins
	}

	ctx := context.Background()
	opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(endpoint)}
	if insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exp, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return fmt.Errorf("tracing: create OTLP HTTP exporter: %w", err)
	}

	res := resource.NewSchemaless(attribute.String("service.name", serviceName))
	p.sdk = sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	p.tracer = p.sdk.Tracer("github.com/temikus/butter")
	return nil
}

// Close flushes pending spans and shuts down the exporter.
func (p *Plugin) Close() error {
	if p.sdk != nil {
		return p.sdk.Shutdown(context.Background())
	}
	return nil
}

// PreHTTP starts a server span for the incoming request and stashes it in
// pctx.Metadata so that OnTrace can end it after the full lifecycle completes.
func (p *Plugin) PreHTTP(pctx *plugin.RequestContext) error {
	if p.tracer == nil {
		return nil
	}
	spanName := "llm.request"
	if pctx.Request != nil {
		spanName = pctx.Request.URL.Path
	}
	ctx := context.Background()
	if pctx.Request != nil {
		ctx = pctx.Request.Context()
	}
	_, span := p.tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer))
	pctx.Metadata[spanMetaKey] = span
	return nil
}

func (p *Plugin) PostHTTP(_ *plugin.RequestContext) error { return nil }

func (p *Plugin) StreamChunk(_ *plugin.RequestContext, chunk []byte) ([]byte, error) {
	return chunk, nil
}

// OnTrace ends the span with the final request attributes and status.
// It is called asynchronously after the response is sent (via EmitTrace's
// goroutine), so span.End() never blocks the client response path.
func (p *Plugin) OnTrace(t *plugin.RequestTrace) {
	span, ok := t.Metadata[spanMetaKey].(trace.Span)
	if !ok || span == nil {
		return
	}

	span.SetAttributes(
		attribute.String("butter.provider", t.Provider),
		attribute.String("butter.model", t.Model),
		attribute.Int("http.response.status_code", t.StatusCode),
		attribute.Int64("butter.duration_ms", t.Duration.Milliseconds()),
	)
	if streaming, ok := t.Metadata["streaming"].(bool); ok && streaming {
		span.SetAttributes(attribute.Bool("butter.streaming", true))
	}
	if cacheVal, ok := t.Metadata["cache"].(string); ok {
		span.SetAttributes(attribute.String("butter.cache", cacheVal))
	}

	if t.Error != nil {
		span.RecordError(t.Error)
		span.SetStatus(codes.Error, t.Error.Error())
	} else if t.StatusCode >= 400 {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", t.StatusCode))
	} else {
		span.SetStatus(codes.Ok, "")
	}
	span.End()
}
