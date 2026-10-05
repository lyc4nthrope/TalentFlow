// Package aplicacion orquesta el caso de uso "procesar un evento": validar el mensaje,
// deduplicar por id, registrar la notificación y simular el envío.
package aplicacion

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/dominio"
	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/eventos"
)

// Repositorio es el puerto de persistencia que necesita el caso de uso.
type Repositorio interface {
	// RegistrarSiEsNuevo registra el id del evento como procesado y guarda la
	// notificación en UNA transacción. Devuelve nueva=false si ese id ya se había
	// procesado (duplicado): en ese caso no guarda nada.
	RegistrarSiEsNuevo(ctx context.Context, eventoID string, n dominio.Notificacion) (guardada dominio.Notificacion, nueva bool, err error)
}

// ErrRechazoPermanente lo devuelve el repositorio cuando la BD rechaza los datos en
// sí (valor demasiado largo, restricción violada...). Reintentar no cambiaría nada:
// el mensaje se descarta en vez de reintentarse para siempre (mensaje envenenado).
var ErrRechazoPermanente = errors.New("datos rechazados de forma permanente")

// Canal es el puerto de "envío" de la notificación. En el Reto 4 se simula por consola
// (CanalConsola); un envío real por SMTP (bonus: Mailhog) sería otra implementación.
type Canal interface {
	Enviar(n dominio.Notificacion)
}

// Resultado le dice al consumidor qué hacer con el mensaje en el broker.
type Resultado int

const (
	// Procesado: efecto aplicado. Confirmar (ack).
	Procesado Resultado = iota
	// Duplicado: ya se había procesado ese id. Confirmar (ack) sin repetir el efecto.
	Duplicado
	// Descartado: nunca podrá procesarse (mensaje corrupto o tipo no soportado).
	// Confirmar para que no vuelva; queda en el log. (DLQ: Reto 29.)
	Descartado
	// ErrorTransitorio: falló algo que puede recuperarse (p. ej. BD caída).
	// Devolver a la cola (nack con requeue) para reintentarlo.
	ErrorTransitorio
)

type Procesador struct {
	repo   Repositorio
	canal  Canal
	logger *slog.Logger
	reloj  func() time.Time
}

func NuevoProcesador(repo Repositorio, canal Canal, logger *slog.Logger, reloj func() time.Time) *Procesador {
	return &Procesador{repo: repo, canal: canal, logger: logger, reloj: reloj}
}

func (p *Procesador) Procesar(ctx context.Context, cuerpo []byte) Resultado {
	sobre, err := eventos.Parsear(cuerpo)
	if err != nil {
		p.logger.Error("mensaje descartado", "motivo", err.Error())
		return Descartado
	}
	log := p.logger.With("eventoId", sobre.ID, "tipo", sobre.Type)

	notificacion, err := dominio.NotificacionDesdeEvento(sobre.Type, sobre.Data, p.reloj())
	if err != nil {
		if errors.Is(err, dominio.ErrTipoNoSoportado) {
			log.Warn("evento ignorado: este servicio no lo consume")
		} else {
			log.Error("evento descartado", "motivo", err.Error())
		}
		return Descartado
	}

	guardada, nueva, err := p.repo.RegistrarSiEsNuevo(ctx, sobre.ID, notificacion)
	if errors.Is(err, ErrRechazoPermanente) {
		log.Error("evento descartado: la base de datos rechazó sus datos", "error", err.Error())
		return Descartado
	}
	if err != nil {
		log.Error("no se pudo registrar la notificación; se reintentará", "error", err.Error())
		return ErrorTransitorio
	}
	if !nueva {
		log.Info("evento duplicado descartado: ya se había procesado este id")
		return Duplicado
	}

	// Se envía solo después de registrarla: si el envío ocurriera antes y la BD
	// fallara, el reintento enviaría la notificación dos veces.
	p.canal.Enviar(guardada)
	log.Info("notificación registrada", "notificacionId", guardada.ID, "empleadoId", guardada.EmpleadoID)
	return Procesado
}
