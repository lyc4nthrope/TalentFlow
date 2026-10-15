package amqp

import (
	"context"
	"errors"
	"fmt"
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
}

func (p *procesadorFalso) CrearCuentaDesdeEmpleado(_ context.Context, e dominio.EmpleadoCreado) error {
	p.recibido = &e
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
	if !a.ack || a.nack || p.recibido == nil || p.recibido.EmpleadoID != "E001" || p.recibido.Email != "juan@empresa.com" {
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
		if !a.ack || a.nack || p.recibido != nil {
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
