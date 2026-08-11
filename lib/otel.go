package lib

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
)

const otelEndpoint = "api.honeycomb.io:443"

type OpenTelemetry struct {
	resource       *resource.Resource
	traceExporter  *otlptrace.Exporter
	traceProvider  *trace.TracerProvider
	metricExporter *otlpmetricgrpc.Exporter
	metricProvider *metric.MeterProvider
	loggerProvider *log.LoggerProvider
	logProcessor   *log.BatchProcessor
	otelHandler    slog.Handler
}

func NewOpenTelemetry(config Config) (OpenTelemetry, error) {
	var err error
	ot := OpenTelemetry{}
	ot.resource, err = makeOtelResource(config)
	if err != nil {
		return ot, fmt.Errorf("make resource: %v", err)
	}
	err = ot.initTracer(config)
	if err != nil {
		return ot, fmt.Errorf("init tracer: %v", err)
	}
	err = ot.initMeter(config)
	if err != nil {
		return ot, fmt.Errorf("init meter: %v", err)
	}
	err = ot.initLogger(config)
	if err != nil {
		return ot, fmt.Errorf("init logger: %v", err)
	}
	return ot, nil
}

func (ot OpenTelemetry) WrapLogHandler(logHandler slog.Handler) slog.Handler {
	return ChainHandlers(ot.otelHandler, logHandler)
}

func (ot OpenTelemetry) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_ = ot.traceProvider.Shutdown(ctx)
	_ = ot.loggerProvider.Shutdown(ctx)
	_ = ot.logProcessor.Shutdown(ctx)
	_ = ot.traceExporter.Shutdown(ctx)
	_ = ot.metricExporter.Shutdown(ctx)
}

func makeOtelResource(config Config) (*resource.Resource, error) {
	r := resource.NewWithAttributes(semconv.SchemaURL,
		semconv.ServiceName("firefly-api"),
		semconv.ServiceVersion(config.BuildTime[:16]),
		semconv.ServiceInstanceID(config.Color),
		semconv.DeploymentEnvironmentNameKey.String(config.Env),
	)
	return resource.Merge(resource.Default(), r)
}

func (ot *OpenTelemetry) initTracer(config Config) error {
	exporter, err := otlptrace.New(
		context.Background(),
		otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(otelEndpoint),
			otlptracegrpc.WithHeaders(map[string]string{
				"x-honeycomb-team": config.HoneycombKey,
			}),
		),
	)
	if err != nil {
		return fmt.Errorf("create exporter: %v", err)
	}
	ot.traceExporter = exporter
	ot.traceProvider = trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithBatcher(exporter),
		trace.WithResource(ot.resource),
	)
	otel.SetTracerProvider(ot.traceProvider)
	return nil
}

func (ot *OpenTelemetry) initMeter(config Config) error {
	exporter, err := otlpmetricgrpc.New(
		context.Background(),
		otlpmetricgrpc.WithEndpoint(otelEndpoint),
		otlpmetricgrpc.WithHeaders(map[string]string{
			"x-honeycomb-team": config.HoneycombKey,
		}),
	)
	if err != nil {
		return fmt.Errorf("create exporter: %v", err)
	}
	ot.metricExporter = exporter
	reader := metric.NewPeriodicReader(
		exporter,
		metric.WithInterval(30*time.Second),
	)
	ot.metricProvider = metric.NewMeterProvider(
		metric.WithResource(ot.resource),
		metric.WithReader(reader),
	)
	otel.SetMeterProvider(ot.metricProvider)
	return nil
}

func (ot *OpenTelemetry) initLogger(config Config) error {
	exporter, err := otlploggrpc.New(
		context.Background(),
		otlploggrpc.WithEndpoint(otelEndpoint),
		otlploggrpc.WithHeaders(map[string]string{
			"x-honeycomb-team": config.HoneycombKey,
		}),
	)
	if err != nil {
		return fmt.Errorf("create exporter: %v", err)
	}
	ot.logProcessor = log.NewBatchProcessor(exporter)
	ot.loggerProvider = log.NewLoggerProvider(
		log.WithProcessor(ot.logProcessor),
		log.WithResource(ot.resource),
	)
	global.SetLoggerProvider(ot.loggerProvider)
	ot.otelHandler = otelslog.NewHandler("firefly-api",
		otelslog.WithLoggerProvider(ot.loggerProvider),
	)
	return nil
}
