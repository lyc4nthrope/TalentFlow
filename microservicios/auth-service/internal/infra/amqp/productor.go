package amqp

import (
	"context"
	"encoding/json"
	"fmt"

	amqp091 "github.com/rabbitmq/amqp091-go"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/eventos"
)

type Productor struct {
	url      string
	exchange string
}

func NuevoProductor(url, exchange string) *Productor {
	return &Productor{
		url:      url,
		exchange: exchange,
	}
}

func (p *Productor) Publicar(ctx context.Context, routingKey string, sobre eventos.Envelope) error {
	conexion, err := amqp091.Dial(p.url)
	if err != nil {
		return fmt.Errorf("error conectando a RabbitMQ: %w", err)
	}
	defer conexion.Close()

	canal, err := conexion.Channel()
	if err != nil {
		return fmt.Errorf("error abriendo canal AMQP: %w", err)
	}
	defer canal.Close()

	cuerpo, err := json.Marshal(sobre)
	if err != nil {
		return fmt.Errorf("error serializando envelope: %w", err)
	}

	return canal.PublishWithContext(
		ctx,
		p.exchange, // "talentflow.eventos"
		routingKey, // p.ej. "auth.password_reset_requested" o "auth.password_changed"
		false,
		false,
		amqp091.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp091.Persistent,
			Body:         cuerpo,
		},
	)
}