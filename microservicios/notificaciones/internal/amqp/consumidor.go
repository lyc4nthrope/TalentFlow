// Package amqp consume los eventos de la cola del servicio en RabbitMQ.
package amqp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/aplicacion"
)

// Procesador es lo que el consumidor necesita del caso de uso.
type Procesador interface {
	Procesar(ctx context.Context, cuerpo []byte) aplicacion.Resultado
}

type Config struct {
	URL      string
	Cola     string
	Etiqueta string // nombre del consumidor, visible en la UI de RabbitMQ
	Prefetch int    // mensajes entregados sin confirmar a la vez
	// EsperaReintento: pausa antes de devolver a la cola un mensaje con error
	// transitorio, para no reintentarlo en un bucle cerrado (antipatrón "retry infinito").
	EsperaReintento time.Duration
}

type Consumidor struct {
	cfg        Config
	procesador Procesador
	logger     *slog.Logger
	conectado  atomic.Bool
}

func NuevoConsumidor(cfg Config, procesador Procesador, logger *slog.Logger) *Consumidor {
	return &Consumidor{cfg: cfg, procesador: procesador, logger: logger}
}

// Estado del vínculo con el broker, para /health.
func (c *Consumidor) Estado() string {
	if c.conectado.Load() {
		return "UP"
	}
	return "DOWN"
}

// Ejecutar consume hasta que se cancele ctx. Si la conexión se pierde (o el broker no
// está disponible al arrancar), reconecta con espera exponencial (1s, 2s, 4s... máx. 30s).
func (c *Consumidor) Ejecutar(ctx context.Context) {
	const esperaInicial = time.Second
	espera := esperaInicial
	for ctx.Err() == nil {
		err := c.consumir(ctx)
		if c.conectado.Swap(false) {
			espera = esperaInicial // estuvo conectado: la caída es nueva, empezar de cero
		}
		if ctx.Err() != nil {
			return
		}
		c.logger.Warn("consumo interrumpido; se reintentará la conexión", "error", err, "espera", espera.String())
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

	// Limita los mensajes en vuelo: sin esto el broker empujaría toda la cola a la
	// memoria del proceso.
	if err := canal.Qos(c.cfg.Prefetch, 0, false); err != nil {
		return fmt.Errorf("configurar prefetch: %w", err)
	}
	// Verifica que la cola exista (la crea broker-init); no la redeclara con otros
	// argumentos, que haría fallar el canal.
	if _, err := canal.QueueDeclarePassive(c.cfg.Cola, true, false, false, false, nil); err != nil {
		return fmt.Errorf("la cola %q no existe: %w", c.cfg.Cola, err)
	}

	// autoAck=false: el mensaje solo se elimina de la cola cuando lo confirmamos, es
	// decir, cuando su efecto ya quedó en la BD. Si el proceso muere antes, el broker
	// lo reentrega (y la deduplicación evita el doble efecto).
	entregas, err := canal.Consume(c.cfg.Cola, c.cfg.Etiqueta, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("iniciar consumo: %w", err)
	}

	cierre := conexion.NotifyClose(make(chan *amqp091.Error, 1))
	c.conectado.Store(true)
	c.logger.Info("consumiendo eventos", "cola", c.cfg.Cola)

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-cierre:
			return fmt.Errorf("conexión cerrada por el broker: %v", err)
		case entrega, abierto := <-entregas:
			if !abierto {
				return errors.New("el canal de entregas se cerró")
			}
			c.manejar(ctx, entrega)
		}
	}
}

func (c *Consumidor) manejar(ctx context.Context, entrega amqp091.Delivery) {
	switch c.procesador.Procesar(ctx, entrega.Body) {
	case aplicacion.Procesado, aplicacion.Duplicado, aplicacion.Descartado:
		if err := entrega.Ack(false); err != nil {
			c.logger.Error("no se pudo confirmar el mensaje", "error", err)
		}
	case aplicacion.ErrorTransitorio:
		dormir(ctx, c.cfg.EsperaReintento)
		if err := entrega.Nack(false, true); err != nil {
			c.logger.Error("no se pudo devolver el mensaje a la cola", "error", err)
		}
	}
}

// dormir espera d o hasta que se cancele ctx. Devuelve false si se canceló.
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
