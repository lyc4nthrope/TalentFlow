package trazas

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func grabarSpans(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	grabador := tracetest.NewSpanRecorder()
	anterior := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(grabador)))
	t.Cleanup(func() { otel.SetTracerProvider(anterior) })
	return grabador
}

func app() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /recursos/{id}", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("GET /health", func(http.ResponseWriter, *http.Request) {})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	return mux
}

func TestSpanConElPatronDeLaRutaYSinRuido(t *testing.T) {
	grabador := grabarSpans(t)
	h := Envolver(app())
	for _, url := range []string{"/recursos/7", "/health", "/metrics", "/no-existe"} {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, url, nil))
	}
	spans := grabador.Ended()
	if len(spans) != 2 {
		t.Fatalf("se esperaban 2 spans (sin /health ni /metrics), hubo %d", len(spans))
	}
	if spans[0].Name() != "GET /recursos/{id}" {
		t.Errorf("nombre = %q", spans[0].Name())
	}
	if spans[1].Name() != "GET /" {
		t.Errorf("el comodín no debe llevar la URL, fue %q", spans[1].Name())
	}
}

func TestContinuaLaTrazaDelTraceparentRecibido(t *testing.T) {
	grabador := grabarSpans(t)
	t.Setenv("OTEL_SDK_DISABLED", "true") // solo el propagador W3C; el proveedor es el grabador
	if _, err := Iniciar(context.Background()); err != nil {
		t.Fatal(err)
	}
	peticion := httptest.NewRequest(http.MethodGet, "/recursos/1", nil)
	peticion.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	Envolver(app()).ServeHTTP(httptest.NewRecorder(), peticion)
	if got := grabador.Ended()[0].SpanContext().TraceID().String(); got != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("traceId = %s", got)
	}
}

func TestIniciarApagadoYEncendido(t *testing.T) {
	t.Setenv("OTEL_SDK_DISABLED", "true")
	apagar, err := Iniciar(context.Background())
	if err != nil || apagar(context.Background()) != nil {
		t.Fatalf("apagado: %v", err)
	}
	t.Setenv("OTEL_SDK_DISABLED", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
	anterior := otel.GetTracerProvider()
	t.Cleanup(func() { otel.SetTracerProvider(anterior) })
	apagar, err = Iniciar(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancelar := context.WithTimeout(context.Background(), time.Second)
	defer cancelar()
	_ = apagar(ctx) // sin spans pendientes no hay nada que enviar
}
