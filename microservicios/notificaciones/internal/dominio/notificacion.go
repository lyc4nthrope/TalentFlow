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

// Tipo de notificación (reto4.pdf, "Estructura de una notificación").
type Tipo string

const (
	TipoBienvenida     Tipo = "BIENVENIDA"
	TipoDesvinculacion Tipo = "DESVINCULACION"
	TipoVacaciones     Tipo = "VACACIONES"

	// Nuevos tipos para Reto 5
	TipoRecuperacionClave Tipo = "RECUPERACION_CLAVE"
	TipoClaveCambiada     Tipo = "CLAVE_CAMBIADA"
)

// Tipos de evento que consume este servicio (Catálogo de Eventos, sección 3).
const (
	EventoEmpleadoCreado        = "empleado.creado"
	EventoEmpleadoRetirado      = "empleado.retirado"
	EventoVacacionesProgramadas = "vacaciones.programadas"

	// Nuevos eventos del Reto 5 (Auth Service)
	EventoPasswordResetRequested = "auth.password_reset_requested"
	EventoPasswordChanged        = "auth.password_changed"
)

// Notificacion es una notificación registrada (y "enviada" mediante el log).
type Notificacion struct {
	ID           string    `json:"id"`
	Tipo         Tipo      `json:"tipo"`
	Destinatario string    `json:"destinatario"`
	Mensaje      string    `json:"mensaje"`
	FechaEnvio   time.Time `json:"fechaEnvio"`
	EmpleadoID   string    `json:"empleadoId"`
}

var (
	// ErrTipoNoSoportado: el evento no le interesa a este servicio.
	ErrTipoNoSoportado = errors.New("tipo de evento no soportado")
	// ErrDatosInvalidos: la carga útil no cumple el Catálogo de Eventos.
	ErrDatosInvalidos = errors.New("carga útil inválida")
)

// Cargas útiles, campo por campo según el Catálogo de Eventos. Solo se declaran los
// campos que este servicio usa; el resto se ignora al decodificar.
type empleadoCreado struct { // 3.1
	EmpleadoID string `json:"empleadoId"`
	Nombre     string `json:"nombre"`
	Apellido   string `json:"apellido"`
	Email      string `json:"email"`
}

type empleadoRetirado struct { // 3.3
	EmpleadoID  string `json:"empleadoId"`
	Email       string `json:"email"`
	FechaRetiro string `json:"fechaRetiro"`
	Motivo      string `json:"motivo"`
}

type vacacionesProgramadas struct { // 3.8
	VacacionesID string `json:"vacacionesId"`
	EmpleadoID   string `json:"empleadoId"`
	Email        string `json:"email"`
	FechaInicio  string `json:"fechaInicio"`
	FechaFin     string `json:"fechaFin"`
	DiasHabiles  int    `json:"diasHabiles"`
}

// NotificacionDesdeEvento traduce un evento en la notificación que le corresponde.
//
// Nota: el Catálogo de Eventos y el Reto 5 mueven la bienvenida a usuario.creado y la
// despedida a cuenta.desactivada (que traen el token / el motivo). En el Reto 4 esos
// eventos no existen todavía, así que se aplica lo que pide el reto4.pdf: bienvenida con
// empleado.creado y desvinculación con empleado.retirado. Cambiarlo en el Reto 5 es
// agregar/quitar casos en este switch.

type passwordResetRequested struct {
	UsuarioID string `json:"usuarioId"`
	Email     string `json:"email"`
	Token     string `json:"token"`
}

type passwordChanged struct {
	UsuarioID string `json:"usuarioId"`
	Email     string `json:"email"`
}

func NotificacionDesdeEvento(tipoEvento string, data json.RawMessage, ahora time.Time) (Notificacion, error) {
	switch tipoEvento {
	case EventoEmpleadoCreado:
		var d empleadoCreado
		if err := decodificar(data, &d); err != nil {
			return Notificacion{}, err
		}
		if err := exigir(map[string]string{"empleadoId": d.EmpleadoID, "email": d.Email, "nombre": d.Nombre}); err != nil {
			return Notificacion{}, err
		}
		nombreCompleto := strings.TrimSpace(d.Nombre + " " + d.Apellido)
		return nueva(TipoBienvenida, d.Email, d.EmpleadoID, ahora,
			fmt.Sprintf("Bienvenido %s a la empresa. Tu registro como empleado quedó completo.", nombreCompleto)), nil

	case EventoEmpleadoRetirado:
		var d empleadoRetirado
		if err := decodificar(data, &d); err != nil {
			return Notificacion{}, err
		}
		if err := exigir(map[string]string{"empleadoId": d.EmpleadoID, "email": d.Email}); err != nil {
			return Notificacion{}, err
		}
		return nueva(TipoDesvinculacion, d.Email, d.EmpleadoID, ahora,
			"Su cuenta ha sido desactivada por desvinculación de la empresa. Gracias por su trabajo."), nil

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
		return nueva(TipoVacaciones, d.Email, d.EmpleadoID, ahora,
			fmt.Sprintf("Sus vacaciones del %s al %s (%d días hábiles) quedaron programadas.",
				d.FechaInicio, d.FechaFin, d.DiasHabiles)), nil


	// --- CASOS DEL RETO 5 ---
	case EventoPasswordResetRequested:
		var d passwordResetRequested
		if err := decodificar(data, &d); err != nil {
			return Notificacion{}, err
		}
		if err := exigir(map[string]string{"usuarioId": d.UsuarioID, "email": d.Email, "token": d.Token}); err != nil {
			return Notificacion{}, err
		}
		return nueva(TipoRecuperacionClave, d.Email, d.UsuarioID, ahora,
			fmt.Sprintf("Has solicitado restablecer tu contraseña. Utiliza el siguiente token para cambiarla: %s", d.Token)), nil

	case EventoPasswordChanged:
		var d passwordChanged
		if err := decodificar(data, &d); err != nil {
			return Notificacion{}, err
		}
		if err := exigir(map[string]string{"usuarioId": d.UsuarioID, "email": d.Email}); err != nil {
			return Notificacion{}, err
		}
		return nueva(TipoClaveCambiada, d.Email, d.UsuarioID, ahora,
			"Tu contraseña ha sido cambiada exitosamente. Si no realizaste esta acción, contacta al administrador."), nil
			
	default:
		return Notificacion{}, fmt.Errorf("%w: %s", ErrTipoNoSoportado, tipoEvento)
	}
}

// LineaDeLog es la "simulación" del envío que pide el reto: un log estructurado.
func (n Notificacion) LineaDeLog() string {
	return fmt.Sprintf("[NOTIFICACIÓN] Tipo: %s | Para: %s | Mensaje: %q", n.Tipo, n.Destinatario, n.Mensaje)
}

func nueva(tipo Tipo, destinatario, empleadoID string, ahora time.Time, mensaje string) Notificacion {
	return Notificacion{
		Tipo:         tipo,
		Destinatario: destinatario,
		Mensaje:      mensaje,
		FechaEnvio:   ahora.UTC(),
		EmpleadoID:   empleadoID,
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
