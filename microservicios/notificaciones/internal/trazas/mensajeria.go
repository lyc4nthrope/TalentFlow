package trazas

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Propagación por RabbitMQ (Reto 8, D10): el traceparent viaja en las cabeceras del mensaje,
// igual que en HTTP. amqp091.Table es un map[string]any, así que se recibe como tal y este
// paquete no depende del cliente de RabbitMQ.

const nombreTracer = "talentflow/mensajeria"

// cabeceras adapta las cabeceras AMQP al propagador de OpenTelemetry.
type cabeceras map[string]any

func (c cabeceras) Get(clave string) string {
	valor, _ := c[clave].(string)
	return valor
}

func (c cabeceras) Set(clave, valor string) { c[clave] = valor }

func (c cabeceras) Keys() []string {
	claves := make([]string, 0, len(c))
	for clave := range c {
		claves = append(claves, clave)
	}
	return claves
}

// IniciarPublicacion abre un span PRODUCER hijo de ctx e inyecta su traceparent en las
// cabeceras del mensaje (que no pueden ser nil). Quien llama cierra el span con Terminar.
func IniciarPublicacion(ctx context.Context, destino map[string]any, tipo string) (context.Context, trace.Span) {
	ctx, span := otel.Tracer(nombreTracer).Start(ctx, "publish "+tipo,
		trace.WithSpanKind(trace.SpanKindProducer), trace.WithAttributes(atributos(tipo)...))
	otel.GetTextMapPropagator().Inject(ctx, cabeceras(destino))
	return ctx, span
}

// IniciarConsumo continúa la traza del traceparent recibido con un span CONSUMER; el
// contexto devuelto debe llegar al caso de uso para que lo que publique quede encadenado.
func IniciarConsumo(ctx context.Context, recibidas map[string]any, tipo string) (context.Context, trace.Span) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, cabeceras(recibidas))
	return otel.Tracer(nombreTracer).Start(ctx, "process "+tipo,
		trace.WithSpanKind(trace.SpanKindConsumer), trace.WithAttributes(atributos(tipo)...))
}

// Terminar marca el span con error si lo hubo y lo cierra.
func Terminar(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

func atributos(tipo string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("messaging.system", "rabbitmq"),
		attribute.String("messaging.rabbitmq.destination.routing_key", tipo),
	}
}
