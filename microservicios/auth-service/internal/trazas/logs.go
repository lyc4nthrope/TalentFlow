package trazas

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// NuevoLogger escribe logs JSON con el formato común de los 7 servicios (Reto 8, F8):
// timestamp, level, service, message y, si el ctx trae un span, traceId y spanId. Grafana
// enlaza ese traceId con la traza en Jaeger. Hay que loguear con InfoContext(ctx, ...).
func NuevoLogger(salida io.Writer, servicio string) *slog.Logger {
	json := slog.NewJSONHandler(salida, &slog.HandlerOptions{
		ReplaceAttr: func(grupos []string, a slog.Attr) slog.Attr {
			if len(grupos) == 0 {
				switch a.Key {
				case slog.TimeKey:
					a.Key = "timestamp"
				case slog.MessageKey:
					a.Key = "message"
				}
			}
			return a
		},
	})
	return slog.New(conTraza{json}).With("service", servicio)
}

// conTraza agrega el traceId y el spanId del span que viaja en el ctx.
type conTraza struct{ slog.Handler }

func (h conTraza) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(slog.String("traceId", sc.TraceID().String()), slog.String("spanId", sc.SpanID().String()))
	}
	return h.Handler.Handle(ctx, r)
}

func (h conTraza) WithAttrs(attrs []slog.Attr) slog.Handler {
	return conTraza{h.Handler.WithAttrs(attrs)}
}

func (h conTraza) WithGroup(nombre string) slog.Handler { return conTraza{h.Handler.WithGroup(nombre)} }
