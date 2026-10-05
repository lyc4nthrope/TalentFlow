package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/dominio"
)

type consultasFalsas struct {
	notificaciones []dominio.Notificacion
	errPing        error
}

func (c consultasFalsas) Listar(context.Context) ([]dominio.Notificacion, error) {
	return c.notificaciones, nil
}

func (c consultasFalsas) ListarPorEmpleado(_ context.Context, id string) ([]dominio.Notificacion, error) {
	var resultado []dominio.Notificacion
	for _, n := range c.notificaciones {
		if n.EmpleadoID == id {
			resultado = append(resultado, n)
		}
	}
	return resultado, nil
}

func (c consultasFalsas) Ping(context.Context) error { return c.errPing }

type brokerFalso string

func (b brokerFalso) Estado() string { return string(b) }

func peticion(t *testing.T, manejador http.Handler, ruta string) *httptest.ResponseRecorder {
	t.Helper()
	grabador := httptest.NewRecorder()
	manejador.ServeHTTP(grabador, httptest.NewRequest(http.MethodGet, ruta, nil))
	return grabador
}

func nuevoManejadorDePrueba(c consultasFalsas) http.Handler {
	return NuevoManejador(c, brokerFalso("UP"), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

var historial = []dominio.Notificacion{
	{ID: "1", Tipo: dominio.TipoBienvenida, EmpleadoID: "E001", Destinatario: "a@x.com"},
	{ID: "2", Tipo: dominio.TipoVacaciones, EmpleadoID: "E002", Destinatario: "b@x.com"},
}

func TestListaTodasLasNotificaciones(t *testing.T) {
	respuesta := peticion(t, nuevoManejadorDePrueba(consultasFalsas{notificaciones: historial}), "/notificaciones")

	var cuerpo []dominio.Notificacion
	if respuesta.Code != http.StatusOK || json.Unmarshal(respuesta.Body.Bytes(), &cuerpo) != nil || len(cuerpo) != 2 {
		t.Fatalf("respuesta inesperada: %d %s", respuesta.Code, respuesta.Body)
	}
}

func TestListaLasDeUnEmpleado(t *testing.T) {
	respuesta := peticion(t, nuevoManejadorDePrueba(consultasFalsas{notificaciones: historial}), "/notificaciones/E002")

	var cuerpo []dominio.Notificacion
	json.Unmarshal(respuesta.Body.Bytes(), &cuerpo) //nolint:errcheck
	if respuesta.Code != http.StatusOK || len(cuerpo) != 1 || cuerpo[0].ID != "2" {
		t.Fatalf("respuesta inesperada: %d %s", respuesta.Code, respuesta.Body)
	}
}

func TestEmpleadoSinNotificacionesDevuelveListaVaciaNoNull(t *testing.T) {
	respuesta := peticion(t, nuevoManejadorDePrueba(consultasFalsas{}), "/notificaciones/E999")

	if respuesta.Code != http.StatusOK || strings.TrimSpace(respuesta.Body.String()) != "[]" {
		t.Fatalf("se esperaba 200 con [], se obtuvo %d %s", respuesta.Code, respuesta.Body)
	}
}

func TestHealthReportaBDYBroker(t *testing.T) {
	sano := peticion(t, nuevoManejadorDePrueba(consultasFalsas{}), "/health")
	caido := peticion(t, nuevoManejadorDePrueba(consultasFalsas{errPing: errors.New("sin conexión")}), "/health")

	if sano.Code != http.StatusOK || !strings.Contains(sano.Body.String(), `"broker":"UP"`) {
		t.Fatalf("health sano inesperado: %d %s", sano.Code, sano.Body)
	}
	if caido.Code != http.StatusServiceUnavailable || !strings.Contains(caido.Body.String(), `"db":"DOWN"`) {
		t.Fatalf("health con BD caída inesperado: %d %s", caido.Code, caido.Body)
	}
}

func TestDocumentacionDisponibleBajoElPrefijoDelServicio(t *testing.T) {
	manejador := nuevoManejadorDePrueba(consultasFalsas{})

	docs := peticion(t, manejador, "/notificaciones/docs")
	spec := peticion(t, manejador, "/notificaciones/openapi.json")

	if docs.Code != http.StatusOK || !strings.Contains(docs.Body.String(), "swagger-ui") {
		t.Fatalf("docs inesperado: %d", docs.Code)
	}
	if spec.Code != http.StatusOK || !json.Valid(spec.Body.Bytes()) {
		t.Fatalf("openapi.json inesperado: %d", spec.Code)
	}
}

func TestRutaDesconocidaResponde404EnJSON(t *testing.T) {
	respuesta := peticion(t, nuevoManejadorDePrueba(consultasFalsas{}), "/otra-cosa")

	if respuesta.Code != http.StatusNotFound || !strings.Contains(respuesta.Body.String(), `"status":404`) {
		t.Fatalf("respuesta inesperada: %d %s", respuesta.Code, respuesta.Body)
	}
}
