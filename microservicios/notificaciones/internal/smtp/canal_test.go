package smtp

import (
	"bufio"
	"bytes"
	"io"
	"log/slog"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"strings"
	"testing"
	"time"

	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/dominio"
)

// correoRecibido es lo que el servidor de prueba capturó de un diálogo SMTP.
type correoRecibido struct {
	remitente    string
	destinatario string
	datos        string
}

// servidorSMTP levanta en 127.0.0.1 un servidor que habla el diálogo SMTP mínimo
// (EHLO, MAIL, RCPT, DATA, QUIT) y entrega por el canal el correo que recibió.
func servidorSMTP(t *testing.T) (host, puerto string, recibidos <-chan correoRecibido) {
	t.Helper()
	escucha, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("no se pudo abrir el servidor de prueba: %v", err)
	}
	t.Cleanup(func() { escucha.Close() })

	canal := make(chan correoRecibido, 1)
	go func() {
		conexion, err := escucha.Accept()
		if err != nil {
			return
		}
		defer conexion.Close()
		lector := bufio.NewReader(conexion)
		responder := func(linea string) { io.WriteString(conexion, linea+"\r\n") }

		var correo correoRecibido
		responder("220 prueba ESMTP")
		for {
			linea, err := lector.ReadString('\n')
			if err != nil {
				return
			}
			linea = strings.TrimRight(linea, "\r\n")
			switch comando := strings.ToUpper(linea); {
			case strings.HasPrefix(comando, "EHLO"), strings.HasPrefix(comando, "HELO"):
				responder("250 prueba")
			case strings.HasPrefix(comando, "MAIL FROM:"):
				correo.remitente = strings.Trim(linea[len("MAIL FROM:"):], "<>")
				responder("250 OK")
			case strings.HasPrefix(comando, "RCPT TO:"):
				correo.destinatario = strings.Trim(linea[len("RCPT TO:"):], "<>")
				responder("250 OK")
			case comando == "DATA":
				responder("354 fin con <CRLF>.<CRLF>")
				var datos strings.Builder
				for {
					l, err := lector.ReadString('\n')
					if err != nil {
						return
					}
					if l == ".\r\n" {
						break
					}
					datos.WriteString(l)
				}
				correo.datos = datos.String()
				responder("250 OK")
				canal <- correo
			case comando == "QUIT":
				responder("221 adiós")
				return
			default:
				responder("500 comando no reconocido")
			}
		}
	}()

	host, puerto, _ = net.SplitHostPort(escucha.Addr().String())
	return host, puerto, canal
}

func nuevoCanalDePrueba(host, puerto string, plazo time.Duration) (*Canal, *bytes.Buffer) {
	var registro bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&registro, nil))
	cfg := Config{Host: host, Puerto: puerto, Remitente: "notificaciones@talentflow.local", Plazo: plazo}
	return NuevoCanal(cfg, logger), &registro
}

var desvinculacion = dominio.Notificacion{
	ID:           "n-1",
	Tipo:         dominio.TipoDesvinculacion,
	Destinatario: "juan.perez@empresa.com",
	Mensaje:      "Su cuenta ha sido desactivada por desvinculación de la empresa. Gracias por su trabajo.",
	FechaEnvio:   time.Date(2026, 9, 28, 14, 0, 0, 0, time.UTC),
	EmpleadoID:   "E001",
}

func TestEnviaElCorreoConRemitenteDestinatarioAsuntoYCuerpo(t *testing.T) {
	host, puerto, recibidos := servidorSMTP(t)
	canal, registro := nuevoCanalDePrueba(host, puerto, 2*time.Second)

	canal.Enviar(desvinculacion)

	var correo correoRecibido
	select {
	case correo = <-recibidos:
	default:
		t.Fatalf("el servidor no recibió ningún correo; log: %s", registro)
	}
	if correo.remitente != "notificaciones@talentflow.local" || correo.destinatario != "juan.perez@empresa.com" {
		t.Fatalf("sobre SMTP = de %q para %q", correo.remitente, correo.destinatario)
	}

	mensaje, err := mail.ReadMessage(strings.NewReader(correo.datos))
	if err != nil {
		t.Fatalf("el correo no es un mensaje válido: %v\n%s", err, correo.datos)
	}
	esperados := map[string]string{
		"From":         "notificaciones@talentflow.local",
		"To":           "juan.perez@empresa.com",
		"Content-Type": "text/plain; charset=UTF-8",
		"Mime-Version": "1.0",
		"Date":         "Mon, 28 Sep 2026 14:00:00 +0000",
	}
	for encabezado, esperado := range esperados {
		if obtenido := mensaje.Header.Get(encabezado); obtenido != esperado {
			t.Errorf("%s = %q, se esperaba %q", encabezado, obtenido, esperado)
		}
	}

	// Las tildes no viajan en crudo: el asunto y el cuerpo llegan codificados en ASCII.
	if strings.ContainsFunc(correo.datos, func(r rune) bool { return r > 127 }) {
		t.Errorf("el correo contiene bytes fuera de ASCII:\n%s", correo.datos)
	}
	asunto, err := new(mime.WordDecoder).DecodeHeader(mensaje.Header.Get("Subject"))
	if err != nil || asunto != "Desvinculación de la empresa" {
		t.Errorf("asunto = %q (err %v), se esperaba %q", asunto, err, "Desvinculación de la empresa")
	}
	cuerpo, err := io.ReadAll(quotedprintable.NewReader(mensaje.Body))
	if err != nil || strings.TrimSpace(string(cuerpo)) != desvinculacion.Mensaje {
		t.Errorf("cuerpo = %q (err %v), se esperaba %q", cuerpo, err, desvinculacion.Mensaje)
	}
}

func TestAsuntoSegunElTipoDeNotificacion(t *testing.T) {
	casos := map[dominio.Tipo]string{
		dominio.TipoBienvenida:     "Bienvenida a la empresa",
		dominio.TipoDesvinculacion: "Desvinculación de la empresa",
		dominio.TipoVacaciones:     "Vacaciones programadas",
		dominio.Tipo("OTRO"):       "Notificación: OTRO",
	}
	for tipo, esperado := range casos {
		if obtenido := asunto(tipo); obtenido != esperado {
			t.Errorf("asunto(%s) = %q, se esperaba %q", tipo, obtenido, esperado)
		}
	}
}

func TestServidorInalcanzableRegistraElErrorYNoBloquea(t *testing.T) {
	// Puerto libre y cerrado: se abre y se cierra para que nadie escuche en él.
	escucha, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	host, puerto, _ := net.SplitHostPort(escucha.Addr().String())
	escucha.Close()
	canal, registro := nuevoCanalDePrueba(host, puerto, 2*time.Second)

	inicio := time.Now()
	canal.Enviar(desvinculacion) // no debe entrar en pánico

	if duracion := time.Since(inicio); duracion > 3*time.Second {
		t.Fatalf("el envío tardó %v: superó el plazo", duracion)
	}
	if !strings.Contains(registro.String(), "no se pudo enviar el correo") {
		t.Fatalf("el fallo debía quedar en el log: %s", registro)
	}
}

func TestServidorQueNoRespondeSeAbandonaAlVencerElPlazo(t *testing.T) {
	// Acepta la conexión y calla: sin plazo, el envío esperaría el saludo para siempre.
	escucha, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer escucha.Close()
	host, puerto, _ := net.SplitHostPort(escucha.Addr().String())
	canal, registro := nuevoCanalDePrueba(host, puerto, 200*time.Millisecond)

	inicio := time.Now()
	canal.Enviar(desvinculacion)

	if duracion := time.Since(inicio); duracion > 2*time.Second {
		t.Fatalf("el envío tardó %v con un plazo de 200ms", duracion)
	}
	if !strings.Contains(registro.String(), "no se pudo enviar el correo") {
		t.Fatalf("el fallo debía quedar en el log: %s", registro)
	}
}
