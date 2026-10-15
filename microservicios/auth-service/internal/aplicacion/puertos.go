// Package aplicacion contiene los casos de uso del auth-service. Depende solo de
// interfaces (puertos); la infraestructura (Postgres, RabbitMQ, JWT) las implementa.
package aplicacion

import (
	"context"
	"errors"
	"time"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/eventos"
)

// ErrUsuarioDuplicado: ya existe una cuenta con ese email, empleadoId o sujeto.
var ErrUsuarioDuplicado = errors.New("usuario duplicado")

// Repositorio persiste las cuentas. Los métodos Buscar* devuelven (nil, nil) si no
// encuentran nada.
type Repositorio interface {
	BuscarPorEmail(ctx context.Context, email string) (*dominio.Usuario, error)
	BuscarPorEmpleadoID(ctx context.Context, empleadoID string) (*dominio.Usuario, error)
	BuscarPorSujeto(ctx context.Context, sujeto string) (*dominio.Usuario, error)
	BuscarPorTokenHash(ctx context.Context, tokenHash string) (*dominio.Usuario, error)
	// Crear devuelve ErrUsuarioDuplicado si choca con una cuenta existente.
	Crear(ctx context.Context, u *dominio.Usuario) error
	// GuardarToken reemplaza el token activo de la cuenta (uno a la vez).
	GuardarToken(ctx context.Context, usuarioID, tokenHash string, tipo dominio.TipoToken, expiraEn time.Time) error
	// ConsumirToken fija la contraseña y el estado y borra el token, solo si el token
	// sigue siendo el activo de la cuenta y el estado no cambió desde que se leyó
	// (p. ej. un retiro concurrente). Devuelve false si no se aplicó.
	ConsumirToken(ctx context.Context, usuarioID, tokenHash, estadoEsperado, passwordHash, nuevoEstado string) (bool, error)
	// ActualizarPassword cambia la contraseña y anula el token pendiente, si lo hay.
	ActualizarPassword(ctx context.Context, usuarioID, passwordHash string) error
}

// Publicador envía un evento al broker con routing key = tipo.
type Publicador interface {
	Publicar(ctx context.Context, sobre eventos.Envelope) error
}

// Identidad son los datos que el token de acceso afirma sobre quien llama.
type Identidad struct {
	Sujeto string
	Rol    string
	Email  string
}

// EmisorTokens emite y valida tokens de acceso (JWT).
type EmisorTokens interface {
	Emitir(sujeto, rol, email string) (string, error)
	Validar(token string) (*Identidad, error)
}

// GeneradorTokens produce los tokens de activación y recuperación (UUID aleatorio).
type GeneradorTokens func() (string, error)

// Reloj devuelve la hora actual (inyectable en las pruebas).
type Reloj func() time.Time
