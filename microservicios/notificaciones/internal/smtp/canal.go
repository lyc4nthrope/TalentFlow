// Package smtp envía las notificaciones como correo real por SMTP (bonus del Reto 4;
// en local, Mailhog). Solo usa la biblioteca estándar.
package smtp

import (
	"bytes"
	"fmt"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"net"
	netsmtp "net/smtp"
	"time"

	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/dominio"
)

type Config struct {
	Host      string
	Puerto    string
	Remitente string
	// Plazo total del envío (conexión + diálogo SMTP). El consumidor procesa los
	// mensajes de la cola mientras tanto: un servidor de correo lento o caído no
	// puede detenerlo más que esto.
	Plazo time.Duration
}

// Canal implementa el puerto aplicacion.Canal sobre SMTP sin autenticación ni TLS
// (lo que ofrece Mailhog).
type Canal struct {
	cfg    Config
	logger *slog.Logger
}

func NuevoCanal(cfg Config, logger *slog.Logger) *Canal {
	return &Canal{cfg: cfg, logger: logger}
}

// Enviar entrega el correo. Si falla, deja el error en el log y termina: la
// notificación ya quedó registrada y el mensaje del broker se confirma igual.
// No se reintenta ni se devuelve a la cola, porque eso podría duplicar correos.
func (c *Canal) Enviar(n dominio.Notificacion) {
	log := c.logger.With("notificacionId", n.ID, "destinatario", n.Destinatario)
	if err := c.entregar(n); err != nil {
		log.Error("no se pudo enviar el correo; la notificación queda registrada", "error", err.Error())
		return
	}
	log.Info("correo enviado", "servidor", c.direccion())
}

func (c *Canal) direccion() string {
	return net.JoinHostPort(c.cfg.Host, c.cfg.Puerto)
}

func (c *Canal) entregar(n dominio.Notificacion) error {
	// Un único límite para todo el envío: net/smtp.SendMail no tiene timeout y
	// bloquearía al consumidor si el servidor acepta la conexión y no responde.
	limite := time.Now().Add(c.cfg.Plazo)
	conexion, err := (&net.Dialer{Deadline: limite}).Dial("tcp", c.direccion())
	if err != nil {
		return fmt.Errorf("conectar con el servidor SMTP: %w", err)
	}
	defer conexion.Close()
	if err := conexion.SetDeadline(limite); err != nil {
		return fmt.Errorf("fijar el plazo de la conexión: %w", err)
	}

	cliente, err := netsmtp.NewClient(conexion, c.cfg.Host)
	if err != nil {
		return fmt.Errorf("saludo del servidor SMTP: %w", err)
	}
	// Mail y Rcpt rechazan direcciones con saltos de línea: un email malicioso en el
	// evento no puede inyectar comandos ni encabezados.
	if err := cliente.Mail(c.cfg.Remitente); err != nil {
		return fmt.Errorf("MAIL FROM: %w", err)
	}
	if err := cliente.Rcpt(n.Destinatario); err != nil {
		return fmt.Errorf("RCPT TO: %w", err)
	}
	cuerpo, err := cliente.Data()
	if err != nil {
		return fmt.Errorf("DATA: %w", err)
	}
	if _, err := cuerpo.Write(c.mensaje(n)); err != nil {
		return fmt.Errorf("escribir el correo: %w", err)
	}
	if err := cuerpo.Close(); err != nil {
		return fmt.Errorf("el servidor no aceptó el correo: %w", err)
	}
	// El correo ya fue aceptado: un fallo al despedirse no cambia el resultado.
	_ = cliente.Quit()
	return nil
}

// mensaje arma el correo de texto plano en UTF-8. El asunto va como "encoded-word"
// y el cuerpo en quoted-printable, así las tildes viajan en 7 bits y cualquier
// servidor las acepta.
func (c *Canal) mensaje(n dominio.Notificacion) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", c.cfg.Remitente)
	fmt.Fprintf(&b, "To: %s\r\n", n.Destinatario)
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", asunto(n.Tipo)))
	fmt.Fprintf(&b, "Date: %s\r\n", n.FechaEnvio.Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	b.WriteString("\r\n")
	qp := quotedprintable.NewWriter(&b)
	_, _ = qp.Write([]byte(n.Mensaje + "\r\n")) // escribir en un bytes.Buffer no falla
	_ = qp.Close()
	return b.Bytes()
}

func asunto(tipo dominio.Tipo) string {
	switch tipo {
	case dominio.TipoBienvenida:
		return "Bienvenida a la empresa"
	case dominio.TipoDesvinculacion:
		return "Desvinculación de la empresa"
	case dominio.TipoVacaciones:
		return "Vacaciones programadas"
	default:
		return "Notificación: " + string(tipo)
	}
}
