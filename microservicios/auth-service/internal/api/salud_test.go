package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type pingerFalso struct{ err error }

func (p pingerFalso) Ping(context.Context) error { return p.err }

type brokerFalso string

func (b brokerFalso) Estado() string { return string(b) }

type cuerpoSalud struct {
	Status     string            `json:"status"`
	Service    string            `json:"service"`
	Timestamp  string            `json:"timestamp"`
	Components map[string]string `json:"components"`
}

func pedirSalud(t *testing.T, bd error, broker string) (int, cuerpoSalud) {
	t.Helper()
	s := NuevoSaludHandler(pingerFalso{bd}, brokerFalso(broker))
	s.ahora = func() time.Time { return time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC) }
	rec := httptest.NewRecorder()
	ConfigurarRutas(NuevoAuthHandler(nil), s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	var c cuerpoSalud
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatalf("cuerpo no es JSON: %q", rec.Body.String())
	}
	return rec.Code, c
}

func TestSaludConBDYBrokerArriba(t *testing.T) {
	codigo, c := pedirSalud(t, nil, "UP")
	if codigo != http.StatusOK || c.Status != "UP" || c.Service != "auth-service" {
		t.Fatalf("codigo=%d cuerpo=%+v", codigo, c)
	}
	if c.Components["db"] != "UP" || c.Components["broker"] != "UP" || c.Components["app"] != "UP" {
		t.Fatalf("componentes = %v", c.Components)
	}
	if c.Timestamp != "2026-10-16T12:00:00Z" {
		t.Fatalf("timestamp = %q", c.Timestamp)
	}
}

func TestSaludConLaBDCaidaResponde503YLoExplica(t *testing.T) {
	codigo, c := pedirSalud(t, errors.New("conexión rechazada"), "UP")
	if codigo != http.StatusServiceUnavailable || c.Status != "DOWN" || c.Components["db"] != "DOWN" {
		t.Fatalf("codigo=%d cuerpo=%+v", codigo, c)
	}
}

// Sin broker el servicio sigue atendiendo login y contraseñas: se reporta, no tumba el estado
// (mismo criterio que notificaciones-service; el Gateway lo lista en "problemas").
func TestSaludSinBrokerSigueUPPeroLoReporta(t *testing.T) {
	codigo, c := pedirSalud(t, nil, "DOWN")
	if codigo != http.StatusOK || c.Status != "UP" || c.Components["broker"] != "DOWN" {
		t.Fatalf("codigo=%d cuerpo=%+v", codigo, c)
	}
}

// El ping a la BD tiene un plazo menor que el del Gateway (3 s).
func TestSaludNoEsperaMasDeDosSegundosPorLaBD(t *testing.T) {
	lento := pingerLento{}
	s := NuevoSaludHandler(lento, brokerFalso("UP"))
	rec := httptest.NewRecorder()
	inicio := time.Now()
	s.Salud(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if d := time.Since(inicio); d > 2500*time.Millisecond {
		t.Fatalf("tardó %v", d)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("codigo = %d", rec.Code)
	}
}

type pingerLento struct{}

func (pingerLento) Ping(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}
