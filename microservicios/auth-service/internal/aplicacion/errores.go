package aplicacion

import "errors"

// Errores de negocio. Sus mensajes son los que ve el cliente, por eso son genéricos
// (Reto 5, D16): no dicen si un email existe ni en qué estado está una cuenta.
var (
	ErrSolicitudInvalida     = errors.New("solicitud inválida")
	ErrCredencialesInvalidas = errors.New("credenciales inválidas")
	ErrTokenInvalido         = errors.New("token inválido o expirado")
	ErrNoAutenticado         = errors.New("token de acceso inválido o expirado")
	ErrPasswordActual        = errors.New("la contraseña actual no es correcta")
	ErrPasswordRepetida      = errors.New("la nueva contraseña debe ser distinta de la actual")
)

// ErrRechazoPermanente: el dato nunca podrá guardarse (inválido o en conflicto con
// otra cuenta). Un consumidor debe descartar el mensaje en vez de reintentarlo.
var ErrRechazoPermanente = errors.New("rechazo permanente")
