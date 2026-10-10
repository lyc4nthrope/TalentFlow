// Package dominio contiene el modelo del perfil y sus errores. No hace I/O.
package dominio

import (
	"errors"
	"time"
)

var (
	ErrNoEncontrado = errors.New("perfil no encontrado")
	ErrArchivado    = errors.New("perfil archivado")
)

// Instante se serializa como ISO-8601 UTC sin fracciones (2026-03-01T10:00:00Z),
// el mismo formato que usa el Catálogo de Eventos.
type Instante time.Time

func (i Instante) MarshalJSON() ([]byte, error) {
	return []byte(`"` + time.Time(i).UTC().Format("2006-01-02T15:04:05Z") + `"`), nil
}

type Perfil struct {
	ID                 string    `json:"id"`
	EmpleadoID         string    `json:"empleadoId"`
	Nombre             string    `json:"nombre"`
	Apellido           string    `json:"apellido"`
	Email              string    `json:"email"`
	Cargo              string    `json:"cargo"`
	Area               string    `json:"area"`
	DepartamentoID     string    `json:"departamentoId"`
	Telefono           string    `json:"telefono"`
	Direccion          string    `json:"direccion"`
	Ciudad             string    `json:"ciudad"`
	Biografia          string    `json:"biografia"`
	Archivado          bool      `json:"archivado"`
	FechaArchivado     *Instante `json:"fechaArchivado"`
	FechaCreacion      Instante  `json:"fechaCreacion"`
	FechaActualizacion Instante  `json:"fechaActualizacion"`
}

// DatosEmpleado son los campos que el perfil replica del evento de empleados.
type DatosEmpleado struct {
	EmpleadoID     string
	Nombre         string
	Apellido       string
	Email          string
	Cargo          string
	Area           string
	DepartamentoID string
}
