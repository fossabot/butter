package metrics

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/temikus/butter/internal/plugin"
)

// Plugin implements an ObservabilityPlugin that records request metrics
// using OpenTelemetry instruments and exposes them via a Prometheus endpoint.
type Plugin struct {
	meterProvider *sdkmetric.MeterProvider
	handler       http.Handler

	requestTotal    metric.Int64Counter
	requestDuration metric.Float64Histogram
	requestErrors   metric.Int64Counter
}

// New creates a metrics plugin.
func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) Name() string { return "metrics" }

func (p *Plugin) Init(cfg map[string]any) error {
	exporter, err := prometheus.New()
	if err != nil {
		return err
	}

	p.meterProvider = sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)

	meter := p.meterProvider.Meter("github.com/temikus/butter")

	p.requestTotal, err = meter.Int64Counter(
		"butter.request.total",
		metric.WithDescription("Total number of requests processed"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	p.requestDuration, err = meter.Float64Histogram(
		"butter.request.duration",
		metric.WithDescription("Request duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	p.requestErrors, err = meter.Int64Counter(
		"butter.request.errors",
		metric.WithDescription("Total number of failed requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return err
	}

	p.handler = promhttp.Handler()
	return nil
}

func (p *Plugin) Close() error {
	if p.meterProvider != nil {
		return p.meterProvider.Shutdown(context.Background())
	}
	return nil
}

// OnTrace records metrics from a completed request trace.
func (p *Plugin) OnTrace(trace *plugin.RequestTrace) {
	ctx := context.Background()

	streaming := false
	if v, ok := trace.Metadata["streaming"].(bool); ok {
		streaming = v
	}

	attrs := metric.WithAttributes(
		attribute.String("provider", trace.Provider),
		attribute.String("model", trace.Model),
		attribute.Int("http.status_code", trace.StatusCode),
		attribute.Bool("streaming", streaming),
	)

	p.requestTotal.Add(ctx, 1, attrs)
	p.requestDuration.Record(ctx, trace.Duration.Seconds(), attrs)

	if trace.Error != nil || trace.StatusCode >= 400 {
		p.requestErrors.Add(ctx, 1, attrs)
	}
}

// Handler returns the Prometheus HTTP handler for the /metrics endpoint.
func (p *Plugin) Handler() http.Handler {
	return p.handler
}
