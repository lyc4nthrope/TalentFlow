package amqp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/eventos"
	amqp091 "github.com/rabbitmq/amqp091-go"
)

type ProcesadorEmpleado interface {
	CrearCuentaDesdeEmpleado(ctx context.Context, email, nombre, apellido, empleadoID string) error
}

type Config struct {
	URL      string
	Cola     string
	Etiqueta string
	Prefetch int
}

type Consumidor struct {
	cfg        Config
	procesador ProcesadorEmpleado
	conectado  atomic.Bool
}

func NuevoConsumidor(cfg Config, procesador ProcesadorEmpleado) *Consumidor {
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

	if err := canal.QueueBind(c.cfg.Cola, "empleado.creado", "talentflow.eventos", false, nil); err != nil {
		return fmt.Errorf("bind cola %q a empleado.creado: %w", c.cfg.Cola, err)
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

	var payload struct {
		EmpleadoID string `json:"empleadoId"`
		Nombre     string `json:"nombre"`
		Apellido   string `json:"apellido"`
		Email      string `json:"email"`
	}
	if err := json.Unmarshal(sobre.Data, &payload); err != nil {
		log.Printf("auth-service: evento descartado, payload inválido: %v", err)
		_ = entrega.Ack(false)
		return
	}

	if err := c.procesador.CrearCuentaDesdeEmpleado(ctx, payload.Email, payload.Nombre, payload.Apellido, payload.EmpleadoID); err != nil {
		log.Printf("auth-service: no se pudo crear la cuenta: %v", err)
		_ = entrega.Nack(false, true)
		return
	}

	if err := entrega.Ack(false); err != nil {
		log.Printf("auth-service: no se pudo confirmar el mensaje: %v", err)
	}
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
