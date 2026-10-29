package trazas

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func TestElTraceparentViajaPorLasCabecerasDelMensaje(t *testing.T) {
	grabador := grabarSpans(t)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	ctx, origen := otel.Tracer("prueba").Start(context.Background(), "POST /empleados")
	mensaje := map[string]any{"x-otra": 7}
	_, publicacion := IniciarPublicacion(ctx, mensaje, "empleado.creado")
	Terminar(publicacion, nil)
	origen.End()

	if _, ok := mensaje["traceparent"].(string); !ok {
		t.Fatalf("no se inyectó traceparent: %v", mensaje)
	}
	_, consumo := IniciarConsumo(context.Background(), mensaje, "empleado.creado")
	Terminar(consumo, errors.New("BD caída"))

	spans := grabador.Ended()
	pub, con := spans[0], spans[2]
	if pub.SpanKind() != trace.SpanKindProducer || con.SpanKind() != trace.SpanKindConsumer {
		t.Fatalf("tipos: %v %v", pub.SpanKind(), con.SpanKind())
	}
	if con.Name() != "process empleado.creado" || con.Parent().SpanID() != pub.SpanContext().SpanID() {
		t.Errorf("el consumo debe ser hijo de la publicación: %s", con.Name())
	}
	if con.SpanContext().TraceID() != origen.SpanContext().TraceID() {
		t.Error("el consumo debe seguir en la traza de la petición")
	}
	if con.Status().Code != codes.Error {
		t.Error("un error de procesamiento marca el span")
	}
	if len(cabeceras(mensaje).Keys()) != 2 || cabeceras(mensaje).Get("x-otra") != "" {
		t.Error("Keys y Get sobre valores que no son texto")
	}
}
