// Package broker consume mensajes de RabbitMQ con reconexión automática.
//
// Topología (compartida por todo el ecosistema, ver docs/eventos.md):
//   - Exchange `talentflow.events`, tipo topic, durable.
//   - Routing key = nombre del evento (p. ej. `empleado.creado`).
//   - Una cola durable propia por servicio, enlazada a los eventos que le interesan.
package broker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"talentflow/perfiles-service/internal/eventos"
)

// Manejador procesa el cuerpo de un mensaje y devuelve un texto con el resultado.
type Manejador func(ctx context.Context, cuerpo []byte) (string, error)

type Consumidor struct {
	URL       string
	Exchange  string
	Cola      string
	Eventos   []string
	Manejador Manejador
	Log       *slog.Logger

	conectado atomic.Bool
}

func (c *Consumidor) Conectado() bool { return c.conectado.Load() }

// Run mantiene la conexión y el consumo hasta que ctx se cancele. Si el broker se cae
// o aún no está listo, reintenta indefinidamente.
func (c *Consumidor) Run(ctx context.Context) {
	for ctx.Err() == nil {
		err := c.sesion(ctx)
		c.conectado.Store(false)
		if ctx.Err() != nil {
			return
		}
		c.Log.Warn("sesión con el broker terminada; reintentando en 3s", "error", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}

func (c *Consumidor) sesion(ctx context.Context) error {
	conn, err := amqp.Dial(c.URL)
	if err != nil {
		return fmt.Errorf("conectando: %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("abriendo canal: %w", err)
	}
	if err := ch.ExchangeDeclare(c.Exchange, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declarando exchange: %w", err)
	}
	if _, err := ch.QueueDeclare(c.Cola, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declarando cola: %w", err)
	}
	for _, clave := range c.Eventos {
		if err := ch.QueueBind(c.Cola, clave, c.Exchange, false, nil); err != nil {
			return fmt.Errorf("enlazando %s: %w", clave, err)
		}
	}
	if err := ch.Qos(10, 0, false); err != nil {
		return fmt.Errorf("qos: %w", err)
	}
	entregas, err := ch.Consume(c.Cola, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consumiendo: %w", err)
	}
	cierre := conn.NotifyClose(make(chan *amqp.Error, 1))

	c.conectado.Store(true)
	c.Log.Info("consumiendo cola", "cola", c.Cola, "eventos", c.Eventos)

	for {
		select {
		case <-ctx.Done():
			return nil
		case e := <-cierre:
			return fmt.Errorf("conexión cerrada: %v", e)
		case d, ok := <-entregas:
			if !ok {
				return errors.New("canal de entregas cerrado")
			}
			c.procesar(ctx, d)
		}
	}
}

// Política de confirmación:
//   - procesado / duplicado / ignorado -> ack
//   - mensaje inválido (reintentar no lo arregla) -> reject sin reencolar
//   - fallo transitorio (p. ej. BD caída) -> nack con reencolado tras una pausa
func (c *Consumidor) procesar(ctx context.Context, d amqp.Delivery) {
	resultado, err := c.Manejador(ctx, d.Body)
	switch {
	case err == nil:
		_ = d.Ack(false)
		c.Log.Info("mensaje procesado", "messageId", d.MessageId, "routingKey", d.RoutingKey, "resultado", resultado)
	case errors.Is(err, eventos.ErrInvalido):
		c.Log.Error("mensaje descartado por inválido", "messageId", d.MessageId, "error", err)
		_ = d.Reject(false)
	default:
		c.Log.Error("fallo procesando el mensaje; se reencola", "messageId", d.MessageId, "error", err)
		select {
		case <-ctx.Done():
		case <-time.After(2 * time.Second):
		}
		_ = d.Nack(false, true)
	}
}
