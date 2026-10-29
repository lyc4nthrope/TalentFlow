package trazas

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestLogJSONConFormatoComunYTraceId(t *testing.T) {
	var salida bytes.Buffer
	logger := NuevoLogger(&salida, "mi-servicio").WithGroup("datos").With("x", 1)
	traza, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	span, _ := trace.SpanIDFromHex("00f067aa0ba902b7")
	ctx := trace.ContextWithSpanContext(context.Background(),
		trace.NewSpanContext(trace.SpanContextConfig{TraceID: traza, SpanID: span, TraceFlags: trace.FlagsSampled}))

	logger.InfoContext(ctx, "evento procesado")
	logger.Info("sin traza")

	lineas := bytes.Split(bytes.TrimSpace(salida.Bytes()), []byte("\n"))
	var conTraza, sinTraza map[string]any
	if err := json.Unmarshal(lineas[0], &conTraza); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(lineas[1], &sinTraza)
	if conTraza["message"] != "evento procesado" || conTraza["service"] != "mi-servicio" || conTraza["level"] != "INFO" {
		t.Errorf("formato: %v", conTraza)
	}
	if _, ok := conTraza["timestamp"]; !ok {
		t.Error("falta timestamp")
	}
	if conTraza["datos"].(map[string]any)["traceId"] != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("traceId: %v", conTraza)
	}
	if _, ok := sinTraza["datos"].(map[string]any)["traceId"]; ok {
		t.Error("sin span no debe haber traceId")
	}
}
