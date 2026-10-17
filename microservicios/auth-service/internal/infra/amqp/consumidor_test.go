package amqp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/aplicacion"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
)

type acuse struct{ ack, nack, requeue bool }

func (a *acuse) Ack(uint64, bool) error              { a.ack = true; return nil }
func (a *acuse) Nack(_ uint64, _ bool, r bool) error { a.nack, a.requeue = true, r; return nil }
func (a *acuse) Reject(uint64, bool) error           { return nil }

type procesadorFalso struct {
	err      error
	recibido *dominio.EmpleadoCreado
	// llamada registra qué caso de uso se invocó, con qué id y qué datos.
	llamada, eventoID string
	datos             any
}

func (p *procesadorFalso) CrearCuentaDesdeEmpleado(_ context.Context, id string, e dominio.EmpleadoCreado) error {
	p.recibido, p.llamada, p.eventoID, p.datos = &e, "crear", id, e
	return p.err
}

func (p *procesadorFalso) RetirarCuenta(_ context.Context, id string, e dominio.EmpleadoRetirado) error {
	p.llamada, p.eventoID, p.datos = "retirar", id, e
	return p.err
}

func (p *procesadorFalso) SuspenderPorVacaciones(_ context.Context, id string, e dominio.VacacionesIniciadas) error {
	p.llamada, p.eventoID, p.datos = "suspender", id, e
	return p.err
}

func (p *procesadorFalso) ReactivarPorFinDeVacaciones(_ context.Context, id string, e dominio.VacacionesFinalizadas) error {
	p.llamada, p.eventoID, p.datos = "reactivar", id, e
	return p.err
}

func entregar(t *testing.T, p *procesadorFalso, cuerpo string) *acuse {
	t.Helper()
	a := &acuse{}
	c := NuevoConsumidor(Config{}, p)
	// Contexto cancelado: la espera antes de reencolar termina al instante.
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()
	c.manejar(ctx, amqp091.Delivery{Acknowledger: a, Body: []byte(cuerpo)})
	return a
}

const empleadoCreado = `{"id":"3f9b2c10-8e4a-4d6b-9f21-7c0a5d8e1234","type":"empleado.creado","version":1,
"occurredAt":"2026-10-15T12:00:00Z","producer":"empleados-service",
"data":{"empleadoId":"E001","nombre":"Juan","apellido":"Pérez","email":"juan@empresa.com"}}`

func TestEmpleadoCreadoSeProcesaYSeConfirma(t *testing.T) {
	p := &procesadorFalso{}
	a := entregar(t, p, empleadoCreado)
	if !a.ack || a.nack || p.recibido == nil || p.recibido.EmpleadoID != "E001" || p.recibido.Email != "juan@empresa.com" ||
		p.eventoID != "3f9b2c10-8e4a-4d6b-9f21-7c0a5d8e1234" {
		t.Fatalf("acuse=%+v recibido=%+v", a, p.recibido)
	}
}

func TestMensajesEnvenenadosSeDescartanSinProcesar(t *testing.T) {
	for _, cuerpo := range []string{
		`no es json`,
		`{"id":"x","type":"empleado.actualizado","data":{}}`,
		`{"id":"x","type":"empleado.creado","data":"texto"}`,
	} {
		p := &procesadorFalso{}
		a := entregar(t, p, cuerpo)
		if !a.ack || a.nack || p.llamada != "" {
			t.Fatalf("%s: acuse=%+v procesado=%v", cuerpo, a, p.recibido != nil)
		}
	}
}

func TestSolicitudInvalidaSeDescarta(t *testing.T) {
	p := &procesadorFalso{err: fmt.Errorf("%w: sin email", aplicacion.ErrSolicitudInvalida)}
	if a := entregar(t, p, empleadoCreado); !a.ack || a.nack {
		t.Fatalf("acuse=%+v", a)
	}
}

func TestRechazoPermanenteDeLaBDSeDescarta(t *testing.T) {
	p := &procesadorFalso{err: fmt.Errorf("crear cuenta: %w: valor demasiado largo", aplicacion.ErrRechazoPermanente)}
	if a := entregar(t, p, empleadoCreado); !a.ack || a.nack {
		t.Fatalf("acuse=%+v", a)
	}
}

func TestFalloTransitorioSeReencola(t *testing.T) {
	p := &procesadorFalso{err: errors.New("bd caída")}
	if a := entregar(t, p, empleadoCreado); a.ack || !a.nack || !a.requeue {
		t.Fatalf("acuse=%+v", a)
	}
}

func TestCadaTipoSeDespachaASuCasoDeUsoConElIdDelSobre(t *testing.T) {
	casos := []struct {
		cuerpo, llamada string
		datos           any
	}{
		{`{"id":"r-1","type":"empleado.retirado","version":1,"data":{"empleadoId":"E001","email":"j@e.com","fechaRetiro":"2026-11-30T16:45:00Z","motivo":"RENUNCIA"}}`,
			"retirar", dominio.EmpleadoRetirado{EmpleadoID: "E001"}},
		{`{"id":"i-1","type":"vacaciones.iniciadas","version":1,"data":{"vacacionesId":"V-2026-0042","empleadoId":"E001","email":"j@e.com","fechaInicio":"2026-03-15T08:00:00-05:00","fechaFin":"2026-03-30T18:00:00-05:00"}}`,
			"suspender", dominio.VacacionesIniciadas{VacacionesID: "V-2026-0042", EmpleadoID: "E001"}},
		{`{"id":"f-1","type":"vacaciones.finalizadas","version":1,"data":{"vacacionesId":"V-2026-0042","empleadoId":"E001","email":"j@e.com","fechaFin":"2026-03-30T18:00:00-05:00"}}`,
			"reactivar", dominio.VacacionesFinalizadas{VacacionesID: "V-2026-0042", EmpleadoID: "E001"}},
	}
	for _, c := range casos {
		p := &procesadorFalso{}
		a := entregar(t, p, c.cuerpo)
		id := c.cuerpo[7:10]
		if !a.ack || a.nack || p.llamada != c.llamada || p.eventoID != id || p.datos != c.datos {
			t.Fatalf("%s: acuse=%+v llamada=%s id=%s datos=%+v", c.llamada, a, p.llamada, p.eventoID, p.datos)
		}
	}
}

func TestPayloadIlegibleDeUnTipoConocidoSeDescartaSinProcesar(t *testing.T) {
	for _, tipo := range []string{"empleado.retirado", "vacaciones.iniciadas", "vacaciones.finalizadas"} {
		p := &procesadorFalso{}
		a := entregar(t, p, `{"id":"x","type":"`+tipo+`","data":"texto"}`)
		if !a.ack || a.nack || p.llamada != "" {
			t.Fatalf("%s: acuse=%+v llamada=%q", tipo, a, p.llamada)
		}
	}
}

func TestLaColaEscuchaLosCuatroEventosDelCiclo(t *testing.T) {
	esperados := "empleado.creado,empleado.retirado,vacaciones.iniciadas,vacaciones.finalizadas"
	if got := strings.Join(dominio.EventosConsumidos, ","); got != esperados {
		t.Fatalf("EventosConsumidos = %s", got)
	}
}

// La topología versionada (infra/rabbitmq/definitions.json) debe ligar auth.eventos
// exactamente a los eventos que el código consume, y no a eventos inventados.
func TestDefinitionsJSONLigaAuthEventosALosMismosEventos(t *testing.T) {
	crudo, err := os.ReadFile("../../../../../infra/rabbitmq/definitions.json")
	if err != nil {
		t.Skipf("definitions.json no disponible fuera del repositorio: %v", err)
	}
	var def struct {
		Bindings []struct {
			Destination string `json:"destination"`
			RoutingKey  string `json:"routing_key"`
		} `json:"bindings"`
	}
	if err := json.Unmarshal(crudo, &def); err != nil {
		t.Fatal(err)
	}
	var claves []string
	for _, b := range def.Bindings {
		if b.Destination == "auth.eventos" {
			claves = append(claves, b.RoutingKey)
		}
		if strings.HasPrefix(b.RoutingKey, "auth.") {
			t.Errorf("binding a un evento que no está en el catálogo: %s -> %s", b.RoutingKey, b.Destination)
		}
	}
	if strings.Join(claves, ",") != strings.Join(dominio.EventosConsumidos, ",") {
		t.Fatalf("bindings de auth.eventos = %v; el consumidor escucha %v", claves, dominio.EventosConsumidos)
	}
}

func TestElConsumidorDelAuthServiceUsaPrefetchUnoParaConservarElOrden(t *testing.T) {
	cfg := ConfigConsumidorAuth("amqp://x", "talentflow.eventos")
	if cfg.Prefetch != 1 || PrefetchOrdenado != 1 || cfg.Cola != "auth.eventos" || cfg.Exchange != "talentflow.eventos" {
		t.Fatalf("config = %+v", cfg)
	}
}

// GET /health publica el estado del consumidor: DOWN hasta que hay una conexión viva.
func TestEstadoDelConsumidor(t *testing.T) {
	c := NuevoConsumidor(Config{}, nil)
	if c.Estado() != "DOWN" {
		t.Fatalf("sin conexión: %s", c.Estado())
	}
	c.conectado.Store(true)
	if c.Estado() != "UP" {
		t.Fatalf("conectado: %s", c.Estado())
	}
}
