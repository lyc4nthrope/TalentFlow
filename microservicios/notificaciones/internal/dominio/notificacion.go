// Package dominio contiene las reglas del servicio de notificaciones: qué notificación
// produce cada evento. No conoce ni el broker, ni HTTP, ni la base de datos.
package dominio

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Tipo de notificación. SEGURIDAD y CUENTA son los que muestra el reto5.pdf en el
// formato del log; VACACIONES viene del Reto 4.
type Tipo string

const (
	TipoSeguridad  Tipo = "SEGURIDAD"
	TipoCuenta     Tipo = "CUENTA"
	TipoVacaciones Tipo = "VACACIONES"
)

// Tipos de evento que consume este servicio (Catálogo de Eventos, sección 3).
//
// Desde el Reto 5 la bienvenida sale de usuario.creado (trae el token de activación)
// y la despedida de cuenta.desactivada con motivo RETIRO: empleado.creado y
// empleado.retirado ya no generan notificaciones. vacaciones.finalizadas tampoco se
// consume: la "bienvenida de regreso" sale de cuenta.activada (FIN_VACACIONES), así un
// empleado retirado durante sus vacaciones no la recibe.
const (
	EventoUsuarioCreado         = "usuario.creado"
	EventoUsuarioRecuperacion   = "usuario.recuperacion"
	EventoCuentaActivada        = "cuenta.activada"
	EventoCuentaDesactivada     = "cuenta.desactivada"
	EventoVacacionesProgramadas = "vacaciones.programadas"
	EventoVacacionesIniciadas   = "vacaciones.iniciadas"
)

// EventosConsumidos son las routing keys que la cola notificaciones.eventos debe
// tener ligadas en infra/rabbitmq/definitions.json (una prueba lo verifica).
var EventosConsumidos = []string{
	EventoUsuarioCreado,
	EventoUsuarioRecuperacion,
	EventoCuentaActivada,
	EventoCuentaDesactivada,
	EventoVacacionesProgramadas,
	EventoVacacionesIniciadas,
}

// Motivos de los eventos de cuenta (Catálogo de Eventos, 3.6 y 3.7).
const (
	MotivoActivacionInicial = "ACTIVACION_INICIAL"
	MotivoFinVacaciones     = "FIN_VACACIONES"
	MotivoVacaciones        = "VACACIONES"
	MotivoRetiro            = "RETIRO"
)

// Notificacion es una notificación registrada (y "enviada" mediante el log).
//
// Mensaje es lo que se guarda en el historial y devuelve la API: los tokens de
// activación y recuperación van enmascarados, porque el historial lo puede leer
// cualquier usuario autenticado y un token vigente permite tomar la cuenta.
// MensajeEntrega es el texto completo (con el token) que solo usan los canales de
// envío (la línea [NOTIFICACIÓN] del log y el cuerpo del correo, como pide el PDF);
// nunca se persiste ni se serializa. Si está vacío, se entrega Mensaje.
//
// EmpleadoID queda vacío cuando el evento no lo trae (usuario.recuperacion solo trae
// el email, D20): se guarda como NULL y se omite en el JSON. Asunto es el asunto del
// correo (canal SMTP); no se persiste.
type Notificacion struct {
	ID             string    `json:"id"`
	Tipo           Tipo      `json:"tipo"`
	Destinatario   string    `json:"destinatario"`
	Mensaje        string    `json:"mensaje"`
	FechaEnvio     time.Time `json:"fechaEnvio"`
	EmpleadoID     string    `json:"empleadoId,omitempty"`
	Asunto         string    `json:"-"`
	MensajeEntrega string    `json:"-"`
}

// TextoAEntregar es el texto que reciben los canales de envío.
func (n Notificacion) TextoAEntregar() string {
	if n.MensajeEntrega != "" {
		return n.MensajeEntrega
	}
	return n.Mensaje
}

var (
	// ErrTipoNoSoportado: el evento no le interesa a este servicio.
	ErrTipoNoSoportado = errors.New("tipo de evento no soportado")
	// ErrDatosInvalidos: la carga útil no cumple el Catálogo de Eventos.
	ErrDatosInvalidos = errors.New("carga útil inválida")
)

// Cargas útiles, campo por campo según el Catálogo de Eventos. Solo se declaran los
// campos que este servicio usa; el resto se ignora al decodificar.
type usuarioCreado struct { // 3.4
	EmpleadoID      string `json:"empleadoId"`
	Email           string `json:"email"`
	TokenActivacion string `json:"tokenActivacion"`
	ExpiraEn        string `json:"expiraEn"`
}

type usuarioRecuperacion struct { // 3.5
	Email             string `json:"email"`
	TokenRecuperacion string `json:"tokenRecuperacion"`
	ExpiraEn          string `json:"expiraEn"`
}

type cuentaActivada struct { // 3.6
	EmpleadoID string `json:"empleadoId"`
	Email      string `json:"email"`
	Motivo     string `json:"motivo"`
}

type cuentaDesactivada struct { // 3.7
	EmpleadoID string `json:"empleadoId"`
	Email      string `json:"email"`
	Motivo     string `json:"motivo"`
	Permanente *bool  `json:"permanente"`
}

type vacacionesProgramadas struct { // 3.8
	VacacionesID string `json:"vacacionesId"`
	EmpleadoID   string `json:"empleadoId"`
	Email        string `json:"email"`
	FechaInicio  string `json:"fechaInicio"`
	FechaFin     string `json:"fechaFin"`
	DiasHabiles  int    `json:"diasHabiles"`
}

type vacacionesIniciadas struct { // 3.9
	VacacionesID string `json:"vacacionesId"`
	EmpleadoID   string `json:"empleadoId"`
	Email        string `json:"email"`
	FechaInicio  string `json:"fechaInicio"`
	FechaFin     string `json:"fechaFin"`
}

// Comienzo de los mensajes del reto5.pdf (el propio PDF los corta en el ejemplo).
const (
	prefijoToken = "Para establecer o recuperar su contraseña use el token"
)

// NotificacionDesdeEvento traduce un evento en la notificación que le corresponde.
func NotificacionDesdeEvento(tipoEvento string, data json.RawMessage, ahora time.Time) (Notificacion, error) {
	switch tipoEvento {
	case EventoUsuarioCreado:
		var d usuarioCreado
		if err := decodificar(data, &d); err != nil {
			return Notificacion{}, err
		}
		if err := exigir(map[string]string{
			"empleadoId": d.EmpleadoID, "email": d.Email, "tokenActivacion": d.TokenActivacion, "expiraEn": d.ExpiraEn,
		}); err != nil {
			return Notificacion{}, err
		}
		n := nueva(TipoSeguridad, d.Email, d.EmpleadoID, ahora,
			"Bienvenido a TalentFlow: active su cuenta",
			mensajeToken(enmascarar(d.TokenActivacion), d.ExpiraEn)+" Bienvenido a TalentFlow.")
		n.MensajeEntrega = mensajeToken(d.TokenActivacion, d.ExpiraEn) + " Bienvenido a TalentFlow."
		return n, nil

	case EventoUsuarioRecuperacion:
		var d usuarioRecuperacion
		if err := decodificar(data, &d); err != nil {
			return Notificacion{}, err
		}
		if err := exigir(map[string]string{
			"email": d.Email, "tokenRecuperacion": d.TokenRecuperacion, "expiraEn": d.ExpiraEn,
		}); err != nil {
			return Notificacion{}, err
		}
		// Sin empleadoId: el catálogo no lo incluye en este evento (D20).
		n := nueva(TipoSeguridad, d.Email, "", ahora,
			"Recuperación de contraseña", mensajeToken(enmascarar(d.TokenRecuperacion), d.ExpiraEn))
		n.MensajeEntrega = mensajeToken(d.TokenRecuperacion, d.ExpiraEn)
		return n, nil

	case EventoCuentaActivada:
		var d cuentaActivada
		if err := decodificar(data, &d); err != nil {
			return Notificacion{}, err
		}
		if err := exigir(map[string]string{"empleadoId": d.EmpleadoID, "email": d.Email, "motivo": d.Motivo}); err != nil {
			return Notificacion{}, err
		}
		var mensaje string
		switch d.Motivo {
		case MotivoFinVacaciones:
			mensaje = "Bienvenido de regreso. Su cuenta fue reactivada."
		case MotivoActivacionInicial:
			mensaje = "Su cuenta fue activada. Ya puede iniciar sesión."
		default:
			return Notificacion{}, fmt.Errorf("%w: motivo desconocido %q", ErrDatosInvalidos, d.Motivo)
		}
		return nueva(TipoCuenta, d.Email, d.EmpleadoID, ahora, "Estado de su cuenta", mensaje), nil

	case EventoCuentaDesactivada:
		var d cuentaDesactivada
		if err := decodificar(data, &d); err != nil {
			return Notificacion{}, err
		}
		if err := exigir(map[string]string{"empleadoId": d.EmpleadoID, "email": d.Email, "motivo": d.Motivo}); err != nil {
			return Notificacion{}, err
		}
		if d.Permanente == nil {
			return Notificacion{}, fmt.Errorf("%w: faltan [permanente]", ErrDatosInvalidos)
		}
		var mensaje string
		switch {
		case d.Motivo == MotivoVacaciones && !*d.Permanente:
			mensaje = "Su cuenta fue desactivada temporalmente por vacaciones."
		case d.Motivo == MotivoRetiro && *d.Permanente:
			mensaje = "Su cuenta fue desactivada de forma permanente por retiro."
		default:
			return Notificacion{}, fmt.Errorf("%w: motivo %q con permanente=%t no es válido",
				ErrDatosInvalidos, d.Motivo, *d.Permanente)
		}
		return nueva(TipoCuenta, d.Email, d.EmpleadoID, ahora, "Estado de su cuenta", mensaje), nil

	case EventoVacacionesProgramadas:
		var d vacacionesProgramadas
		if err := decodificar(data, &d); err != nil {
			return Notificacion{}, err
		}
		if err := exigir(map[string]string{
			"empleadoId": d.EmpleadoID, "email": d.Email, "fechaInicio": d.FechaInicio, "fechaFin": d.FechaFin,
		}); err != nil {
			return Notificacion{}, err
		}
		return nueva(TipoVacaciones, d.Email, d.EmpleadoID, ahora, "Vacaciones programadas",
			fmt.Sprintf("Sus vacaciones del %s al %s (%d días hábiles) quedaron programadas.",
				d.FechaInicio, d.FechaFin, d.DiasHabiles)), nil

	case EventoVacacionesIniciadas:
		var d vacacionesIniciadas
		if err := decodificar(data, &d); err != nil {
			return Notificacion{}, err
		}
		if err := exigir(map[string]string{
			"vacacionesId": d.VacacionesID, "empleadoId": d.EmpleadoID, "email": d.Email,
			"fechaInicio": d.FechaInicio, "fechaFin": d.FechaFin,
		}); err != nil {
			return Notificacion{}, err
		}
		return nueva(TipoVacaciones, d.Email, d.EmpleadoID, ahora, "Inicio de sus vacaciones",
			fmt.Sprintf("Sus vacaciones %s comenzaron (%s) y terminan el %s. Su cuenta queda inactiva durante el período.",
				d.VacacionesID, d.FechaInicio, d.FechaFin)), nil

	default:
		return Notificacion{}, fmt.Errorf("%w: %s", ErrTipoNoSoportado, tipoEvento)
	}
}

// LineaDeLog es la "simulación" del envío que pide el reto: un log estructurado. Lleva
// el texto completo (con el token), como el ejemplo del reto5.pdf.
func (n Notificacion) LineaDeLog() string {
	return fmt.Sprintf("[NOTIFICACIÓN] Tipo: %s | Para: %s | Mensaje: %q", n.Tipo, n.Destinatario, n.TextoAEntregar())
}

// enmascarar deja visibles solo los últimos 4 caracteres del token: suficiente para
// que soporte identifique cuál se envió, inútil para usarlo.
func enmascarar(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return "****" + token[len(token)-4:]
}

func mensajeToken(token, expiraEn string) string {
	return fmt.Sprintf("%s %s (vence %s).", prefijoToken, token, expiraEn)
}

func nueva(tipo Tipo, destinatario, empleadoID string, ahora time.Time, asunto, mensaje string) Notificacion {
	return Notificacion{
		Tipo:         tipo,
		Destinatario: destinatario,
		Mensaje:      mensaje,
		FechaEnvio:   ahora.UTC(),
		EmpleadoID:   empleadoID,
		Asunto:       asunto,
	}
}

func decodificar(data json.RawMessage, destino any) error {
	if err := json.Unmarshal(data, destino); err != nil {
		return fmt.Errorf("%w: %v", ErrDatosInvalidos, err)
	}
	return nil
}

func exigir(campos map[string]string) error {
	var faltantes []string
	for nombre, valor := range campos {
		if strings.TrimSpace(valor) == "" {
			faltantes = append(faltantes, nombre)
		}
	}
	if len(faltantes) > 0 {
		sort.Strings(faltantes) // el orden de un map no es determinista
		return fmt.Errorf("%w: faltan %v", ErrDatosInvalidos, faltantes)
	}
	return nil
}
