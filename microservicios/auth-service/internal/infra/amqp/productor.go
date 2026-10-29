package amqp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/eventos"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/trazas"
)

// Productor publica en el exchange del ecosistema con routing key = type del evento.
// Abre una conexión por publicación: el volumen del auth-service es bajo y así no hay
// estado de reconexión que mantener.
type Productor struct {
	url      string
	exchange string
}

func NuevoProductor(url, exchange string) *Productor {
	return &Productor{url: url, exchange: exchange}
}

func (p *Productor) Publicar(ctx context.Context, sobre eventos.Envelope) error {
	conexion, err := amqp091.DialConfig(p.url, amqp091.Config{Dial: amqp091.DefaultDial(3 * time.Second)})
	if err != nil {
		return fmt.Errorf("conectar a RabbitMQ: %w", err)
	}
	defer conexion.Close()

	canal, err := conexion.Channel()
	if err != nil {
		return fmt.Errorf("abrir canal AMQP: %w", err)
	}
	defer canal.Close()
	if err := canal.Confirm(false); err != nil {
		return fmt.Errorf("activar confirmaciones: %w", err)
	}

	cuerpo, err := json.Marshal(sobre)
	if err != nil {
		return fmt.Errorf("serializar envelope: %w", err)
	}

	// Reto 8 (D10): span PRODUCER y su traceparent en las cabeceras del mensaje.
	cabeceras := amqp091.Table{}
	ctx, span := trazas.IniciarPublicacion(ctx, cabeceras, sobre.Type)
	err = p.publicar(ctx, canal, sobre, cuerpo, cabeceras)
	trazas.Terminar(span, err)
	return err
}

func (p *Productor) publicar(ctx context.Context, canal *amqp091.Channel, sobre eventos.Envelope, cuerpo []byte, cabeceras amqp091.Table) error {
	ctx, cancelar := context.WithTimeout(ctx, 3*time.Second)
	defer cancelar()
	confirmacion, err := canal.PublishWithDeferredConfirmWithContext(ctx, p.exchange, sobre.Type, false, false,
		amqp091.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp091.Persistent,
			MessageId:    sobre.ID,
			Type:         sobre.Type,
			Headers:      cabeceras,
			Body:         cuerpo,
		})
	if err != nil {
		return fmt.Errorf("publicar %s: %w", sobre.Type, err)
	}
	confirmado, err := confirmacion.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("esperar confirmación de %s: %w", sobre.Type, err)
	}
	if !confirmado {
		return fmt.Errorf("el broker rechazó %s", sobre.Type)
	}
	return nil
}
