// Package dominio contiene el modelo de cuentas del auth-service y sus reglas, sin
// dependencias de infraestructura.
package dominio

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Estados de la cuenta (Reto 5, D3). No es un booleano: hay que distinguir la
// suspensión temporal (vacaciones) de la baja permanente (retiro) para que
// vacaciones.finalizadas nunca reactive a un empleado retirado.
const (
	EstadoPendienteActivacion   = "PENDIENTE_ACTIVACION"
	EstadoActiva                = "ACTIVA"
	EstadoSuspendidaTemporal    = "SUSPENDIDA_TEMPORAL"
	EstadoDesactivadaPermanente = "DESACTIVADA_PERMANENTE"
)

// Roles del sistema (claim "role" del JWT).
const (
	RolAdmin = "ADMIN"
	RolUser  = "USER"
)

// SujetoAdmin es el "sub" del administrador semilla, que no es un empleado.
const SujetoAdmin = "admin"

// TipoToken distingue el token de activación (alta) del de recuperación.
type TipoToken string

const (
	TokenActivacion   TipoToken = "ACTIVACION"
	TokenRecuperacion TipoToken = "RECUPERACION"
)

// Usuario es una cuenta del sistema. Sujeto es el "sub" del JWT: el empleadoId de
// un empleado o "admin" para el administrador semilla.
type Usuario struct {
	ID           string
	Sujeto       string
	EmpleadoID   string // vacío para el administrador
	Email        string
	PasswordHash string // vacío mientras la cuenta no se activa: nunca hay contraseña temporal
	Rol          string
	Estado       string

	// Un único token activo por cuenta; solo se guarda su hash.
	TokenHash     string
	TokenTipo     TipoToken
	TokenExpiraEn time.Time
}

// PuedeAutenticarse indica si la cuenta puede iniciar sesión o cambiar su contraseña.
func (u *Usuario) PuedeAutenticarse() bool {
	return u.Estado == EstadoActiva && u.PasswordHash != ""
}

// HashToken devuelve el SHA-256 en hexadecimal de un token de activación o
// recuperación. En la BD solo se guarda este hash: una fuga de la tabla no entrega
// tokens utilizables.
func HashToken(token string) string {
	suma := sha256.Sum256([]byte(token))
	return hex.EncodeToString(suma[:])
}
