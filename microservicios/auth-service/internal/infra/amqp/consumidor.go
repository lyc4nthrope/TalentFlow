package amqp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/aplicacion"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/eventos"
	amqp091 "github.com/rabbitmq/amqp091-go"
)

// ProcesadorEventos son los casos de uso que reaccionan a cada evento consumido.
// Reciben el id del envelope para deduplicar (D4).
type ProcesadorEventos interface {
	CrearCuentaDesdeEmpleado(ctx context.Context, eventoID string, e dominio.EmpleadoCreado) error
	RetirarCuenta(ctx context.Context, eventoID string, e dominio.EmpleadoRetirado) error
	SuspenderPorVacaciones(ctx context.Context, eventoID string, e dominio.VacacionesIniciadas) error
	ReactivarPorFinDeVacaciones(ctx context.Context, eventoID string, e dominio.VacacionesFinalizadas) error
}

// esperaReintento evita reencolar al instante un mensaje que falló por un problema
// transitorio (BD caída): sin espera sería un bucle que consume CPU sin avanzar.
const esperaReintento = 5 * time.Second

type Config struct {
	URL      string
	Exchange string
	Cola     string
	Etiqueta string
	Prefetch int
}

// PrefetchOrdenado es 1 a propósito: con un prefetch mayor, un mensaje que falla de
// forma transitoria se reencola DETRÁS de los que ya estaban entregados, y el orden
// importa en el ciclo de vida (vacaciones.finalizadas antes que su vacaciones.iniciadas
// dejaría la cuenta suspendida para siempre; empleado.retirado antes que su
// empleado.creado daría una cuenta a un retirado). Con 1 el mensaje reencolado vuelve
// a la cabeza de la cola. El volumen de eventos de cuentas es bajo: el rendimiento no
// es una preocupación aquí.
const PrefetchOrdenado = 1

// ConfigConsumidorAuth es la configuración del consumidor del auth-service.
func ConfigConsumidorAuth(url, exchange string) Config {
	return Config{URL: url, Exchange: exchange, Cola: "auth.eventos", Etiqueta: "auth-service", Prefetch: PrefetchOrdenado}
}

type Consumidor struct {
	cfg        Config
	procesador ProcesadorEventos
	conectado  atomic.Bool
}

func NuevoConsumidor(cfg Config, procesador ProcesadorEventos) *Consumidor {
	return &Consumidor{cfg: cfg, procesador: procesador}
}

func (c *Consumidor) Ejecutar(ctx context.Context) {
	const esperaInicial = time.Second
	espera := esperaInicial
	for ctx.Err() == nil {
		err := c.consumir(ctx)
		if c.conectado.Swap(false) {
			espera = esperaInicial
		}
		if ctx.Err() != nil {
			return
		}
		log.Printf("auth-service: consumo interrumpido; reintentando en %s: %v", espera, err)
		if !dormir(ctx, espera) {
			return
		}
		espera = min(espera*2, 30*time.Second)
	}
}

func (c *Consumidor) consumir(ctx context.Context) error {
	conexion, err := amqp091.Dial(c.cfg.URL)
	if err != nil {
		return fmt.Errorf("conectar al broker: %w", err)
	}
	defer conexion.Close()

	canal, err := conexion.Channel()
	if err != nil {
		return fmt.Errorf("abrir canal: %w", err)
	}
	defer canal.Close()

	if err := canal.Qos(c.cfg.Prefetch, 0, false); err != nil {
		return fmt.Errorf("configurar prefetch: %w", err)
	}

	if _, err := canal.QueueDeclare(c.cfg.Cola, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declarar cola %q: %w", c.cfg.Cola, err)
	}

	// Los mismos bindings que infra/rabbitmq/definitions.json (idempotente).
	for _, clave := range dominio.EventosConsumidos {
		if err := canal.QueueBind(c.cfg.Cola, clave, c.cfg.Exchange, false, nil); err != nil {
			return fmt.Errorf("bind cola %q a %s: %w", c.cfg.Cola, clave, err)
		}
	}

	entregas, err := canal.Consume(c.cfg.Cola, c.cfg.Etiqueta, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("iniciar consumo: %w", err)
	}

	cierre := conexion.NotifyClose(make(chan *amqp091.Error, 1))
	c.conectado.Store(true)
	log.Printf("auth-service: consumiendo eventos en cola %s", c.cfg.Cola)

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-cierre:
			return fmt.Errorf("conexión cerrada por el broker: %v", err)
		case entrega, abierto := <-entregas:
			if !abierto {
				return errors.New("canal de entregas cerrado")
			}
			c.manejar(ctx, entrega)
		}
	}
}

func (c *Consumidor) manejar(ctx context.Context, entrega amqp091.Delivery) {
	var sobre eventos.Envelope
	if err := json.Unmarshal(entrega.Body, &sobre); err != nil {
		log.Printf("auth-service: evento descartado, envelope inválido: %v", err)
		_ = entrega.Ack(false)
		return
	}

	err := c.despachar(ctx, sobre)
	switch {
	case err == nil:
		if err := entrega.Ack(false); err != nil {
			log.Printf("auth-service: no se pudo confirmar el mensaje %s: %v", sobre.ID, err)
		}
	case errors.Is(err, errTipoNoSoportado):
		log.Printf("auth-service: evento descartado, tipo no soportado %q (id %s)", sobre.Type, sobre.ID)
		_ = entrega.Ack(false)
	case errors.Is(err, aplicacion.ErrSolicitudInvalida), errors.Is(err, aplicacion.ErrRechazoPermanente):
		// Mensaje envenenado o dato que la BD nunca aceptará: reintentarlo no sirve.
		log.Printf("auth-service: evento descartado (id %s, %s): %v", sobre.ID, sobre.Type, err)
		_ = entrega.Ack(false)
	default:
		log.Printf("auth-service: fallo transitorio procesando %s (%s); se reintentará: %v", sobre.ID, sobre.Type, err)
		dormir(ctx, esperaReintento)
		_ = entrega.Nack(false, true)
	}
}

var errTipoNoSoportado = errors.New("tipo de evento no soportado")

// despachar decodifica el payload según el tipo y llama al caso de uso. Un payload
// que no se puede decodificar es una solicitud inválida (se descarta).
func (c *Consumidor) despachar(ctx context.Context, sobre eventos.Envelope) error {
	switch sobre.Type {
	case dominio.EventoEmpleadoCreado:
		var d dominio.EmpleadoCreado
		if err := decodificar(sobre, &d); err != nil {
			return err
		}
		return c.procesador.CrearCuentaDesdeEmpleado(ctx, sobre.ID, d)
	case dominio.EventoEmpleadoRetirado:
		var d dominio.EmpleadoRetirado
		if err := decodificar(sobre, &d); err != nil {
			return err
		}
		return c.procesador.RetirarCuenta(ctx, sobre.ID, d)
	case dominio.EventoVacacionesIniciadas:
		var d dominio.VacacionesIniciadas
		if err := decodificar(sobre, &d); err != nil {
			return err
		}
		return c.procesador.SuspenderPorVacaciones(ctx, sobre.ID, d)
	case dominio.EventoVacacionesFinalizadas:
		var d dominio.VacacionesFinalizadas
		if err := decodificar(sobre, &d); err != nil {
			return err
		}
		return c.procesador.ReactivarPorFinDeVacaciones(ctx, sobre.ID, d)
	default:
		return errTipoNoSoportado
	}
}

func decodificar(sobre eventos.Envelope, destino any) error {
	if err := json.Unmarshal(sobre.Data, destino); err != nil {
		return fmt.Errorf("%w: payload de %s: %v", aplicacion.ErrSolicitudInvalida, sobre.Type, err)
	}
	return nil
}

func dormir(ctx context.Context, d time.Duration) bool {
	temporizador := time.NewTimer(d)
	defer temporizador.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-temporizador.C:
		return true
	}
}
