package dominio

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

var ahora = time.Date(2026, 9, 28, 14, 0, 0, 0, time.UTC)

func TestNotificacionDesdeEvento(t *testing.T) {
	casos := []struct {
		nombre       string
		tipoEvento   string
		data         string
		tipo         Tipo
		destinatario string
		contiene     string
	}{
		{
			nombre:       "empleado.creado genera BIENVENIDA con el nombre completo",
			tipoEvento:   "empleado.creado",
			data:         `{"empleadoId":"E001","nombre":"Juan","apellido":"Pérez","email":"juan.perez@empresa.com","numeroEmpleado":"EMP-1","cargo":"Dev","area":"TI","departamentoId":"IT","fechaIngreso":"2026-03-01","estado":"ACTIVO"}`,
			tipo:         TipoBienvenida,
			destinatario: "juan.perez@empresa.com",
			contiene:     "Bienvenido Juan Pérez",
		},
		{
			nombre:       "empleado.retirado genera DESVINCULACION",
			tipoEvento:   "empleado.retirado",
			data:         `{"empleadoId":"E001","email":"juan.perez@empresa.com","fechaRetiro":"2026-11-30T16:45:00Z","motivo":"RENUNCIA"}`,
			tipo:         TipoDesvinculacion,
			destinatario: "juan.perez@empresa.com",
			contiene:     "Su cuenta ha sido desactivada",
		},
		{
			nombre:       "vacaciones.programadas genera VACACIONES con las fechas",
			tipoEvento:   "vacaciones.programadas",
			data:         `{"vacacionesId":"V-2026-0042","empleadoId":"E001","email":"juan.perez@empresa.com","fechaInicio":"2026-12-15","fechaFin":"2026-12-30","diasHabiles":12}`,
			tipo:         TipoVacaciones,
			destinatario: "juan.perez@empresa.com",
			contiene:     "del 2026-12-15 al 2026-12-30 (12 días hábiles)",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			n, err := NotificacionDesdeEvento(c.tipoEvento, json.RawMessage(c.data), ahora)
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if n.Tipo != c.tipo || n.Destinatario != c.destinatario || n.EmpleadoID != "E001" {
				t.Fatalf("notificación inesperada: %+v", n)
			}
			if !strings.Contains(n.Mensaje, c.contiene) {
				t.Fatalf("mensaje %q no contiene %q", n.Mensaje, c.contiene)
			}
			if !n.FechaEnvio.Equal(ahora) {
				t.Fatalf("fechaEnvio = %v, se esperaba %v", n.FechaEnvio, ahora)
			}
		})
	}
}

func TestNotificacionDesdeEventoRechazaTipoNoSoportado(t *testing.T) {
	_, err := NotificacionDesdeEvento("empleado.actualizado", json.RawMessage(`{}`), ahora)
	if !errors.Is(err, ErrTipoNoSoportado) {
		t.Fatalf("se esperaba ErrTipoNoSoportado, se obtuvo %v", err)
	}
}

func TestNotificacionDesdeEventoRechazaCargaIncompleta(t *testing.T) {
	_, err := NotificacionDesdeEvento("empleado.creado", json.RawMessage(`{"empleadoId":"E001"}`), ahora)
	if !errors.Is(err, ErrDatosInvalidos) {
		t.Fatalf("se esperaba ErrDatosInvalidos, se obtuvo %v", err)
	}
	if !strings.Contains(err.Error(), "[email nombre]") {
		t.Fatalf("los campos faltantes deben listarse en orden: %v", err)
	}
}

func TestLineaDeLogTieneElFormatoDelReto(t *testing.T) {
	n := Notificacion{Tipo: TipoBienvenida, Destinatario: "juan@empresa.com", Mensaje: "Hola"}
	esperada := `[NOTIFICACIÓN] Tipo: BIENVENIDA | Para: juan@empresa.com | Mensaje: "Hola"`
	if got := n.LineaDeLog(); got != esperada {
		t.Fatalf("línea = %s", got)
	}
}
