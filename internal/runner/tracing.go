package runner

import (
	"context"
	"log"

	"github.com/stateful/runme/v3/internal/version"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

var (
	runmeTp    *sdktrace.TracerProvider
	notebookTp *sdktrace.TracerProvider
	cellTp     *sdktrace.TracerProvider
)

// initTracer creates and registers trace provider instance.
func init() {
	runmeTp = traceProvider("runme")
	notebookTp = traceProvider("notebook")
	cellTp = traceProvider("cell")
}

func traceProvider(serviceName string) *sdktrace.TracerProvider {
	// stdr.SetVerbosity(5)

	// ppExp, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	// if err != nil {
	// 	log.Fatal("failed to initialize stdouttrace exporter: ", err)
	// }
	// ppBsp := sdktrace.NewBatchSpanProcessor(ppExp)
	ctx := context.Background()

	grpcExp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
	if err != nil {
		log.Fatal("failed to initialize otlptracegrpc exporter: ", err)
	}
	grpcBsp := sdktrace.NewBatchSpanProcessor(grpcExp)
	r, err := newResource(context.Background(), serviceName)
	if err != nil {
		log.Fatal("failed to initialize otel resource: ", err)
	}
	return sdktrace.NewTracerProvider(
		sdktrace.WithResource(r),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(grpcBsp),
		// sdktrace.WithSpanProcessor(ppBsp),
	)
	// otel.SetTracerProvider(tp)
}

func newResource(ctx context.Context, serviceName string) (*resource.Resource, error) {
	envR, err := resource.New(ctx,
		resource.WithFromEnv(),   // pull attributes from OTEL_RESOURCE_ATTRIBUTES and OTEL_SERVICE_NAME environment variables
		resource.WithProcess(),   // This option configures a set of Detectors that discover process information
		resource.WithOS(),        // This option configures a set of Detectors that discover OS information
		resource.WithContainer(), // This option configures a set of Detectors that discover container information
		resource.WithHost(),      // This option configures a set of Detectors that discover host information
	)
	if err != nil {
		log.Fatal("failed to initialize otel resource: ", err)
	}
	defaultR, err := resource.Merge(resource.Default(), envR)
	if err != nil {
		log.Fatal("failed to initialize otel resource: ", err)
	}
	r, err := resource.Merge(defaultR, resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String(version.BaseVersion()),
	))
	return r, err
}
