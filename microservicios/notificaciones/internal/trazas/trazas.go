// Package trazas configura OpenTelemetry (Reto 8, D1, D7 y D12): spans de cada petición
// HTTP enviados a Jaeger por OTLP/HTTP. Configuración por entorno (OTEL_SERVICE_NAME,
// OTEL_EXPORTER_OTLP_ENDPOINT); OTEL_SDK_DISABLED=true lo apaga. Infraestructura: fuera del
// umbral de cobertura de negocio, con su prueba.
package trazas

import (
	"context"
	"net/http"
	"os"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Apagar envía los spans pendientes antes de que el proceso termine.
type Apagar func(context.Context) error

// Iniciar registra el proveedor global de trazas y el propagador W3C (traceparent).
func Iniciar(ctx context.Context) (Apagar, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	if strings.EqualFold(os.Getenv("OTEL_SDK_DISABLED"), "true") {
		return func(context.Context) error { return nil }, nil
	}
	exportador, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	// resource.Default() ya lee OTEL_SERVICE_NAME y OTEL_RESOURCE_ATTRIBUTES.
	proveedor := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exportador), sdktrace.WithResource(resource.Default()))
	otel.SetTracerProvider(proveedor)
	return proveedor.Shutdown, nil
}

// Envolver abre un span por petición (o continúa la traza del traceparent recibido).
// otelhttp lo nombra con el patrón que casó en el ServeMux ("GET /auth/login"), no con la
// URL. Los healthchecks y los scrapes de Prometheus se omiten: taparían las trazas de negocio.
func Envolver(h http.Handler) http.Handler {
	return otelhttp.NewHandler(h, "http",
		otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/health" && r.URL.Path != "/metrics"
		}))
}
