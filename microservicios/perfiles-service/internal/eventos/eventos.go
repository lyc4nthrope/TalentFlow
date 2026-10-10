// Package eventos define el envelope del Catálogo de Eventos y la carga útil de los
// eventos de empleados que consume este servicio.
package eventos

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"talentflow/perfiles-service/internal/dominio"
)

// ErrInvalido indica un mensaje que no cumple el contrato: reintentarlo no lo arregla.
var ErrInvalido = errors.New("evento inválido")

const (
	EmpleadoCreado      = "empleado.creado"
	EmpleadoActualizado = "empleado.actualizado"
	EmpleadoRetirado    = "empleado.retirado"
)

type Envelope struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Version    int             `json:"version"`
	OccurredAt string          `json:"occurredAt"`
	Producer   string          `json:"producer"`
	Data       json.RawMessage `json:"data"`
}

func invalido(formato string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalido, fmt.Sprintf(formato, args...))
}

// ParsearEnvelope valida los metadatos comunes (Catálogo, sección 2).
func ParsearEnvelope(cuerpo []byte) (*Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(cuerpo, &e); err != nil {
		return nil, invalido("no es JSON válido: %v", err)
	}
	var faltan []string
	if strings.TrimSpace(e.ID) == "" {
		faltan = append(faltan, "id")
	}
	if strings.TrimSpace(e.Type) == "" {
		faltan = append(faltan, "type")
	}
	if e.Version < 1 {
		faltan = append(faltan, "version")
	}
	if strings.TrimSpace(e.OccurredAt) == "" {
		faltan = append(faltan, "occurredAt")
	}
	if strings.TrimSpace(e.Producer) == "" {
		faltan = append(faltan, "producer")
	}
	if len(e.Data) == 0 || string(e.Data) == "null" {
		faltan = append(faltan, "data")
	}
	if len(faltan) > 0 {
		return nil, invalido("faltan campos del envelope: %s", strings.Join(faltan, ", "))
	}
	return &e, nil
}

type cargaEmpleado struct {
	EmpleadoID     string `json:"empleadoId"`
	Nombre         string `json:"nombre"`
	Apellido       string `json:"apellido"`
	Email          string `json:"email"`
	Cargo          string `json:"cargo"`
	Area           string `json:"area"`
	DepartamentoID string `json:"departamentoId"`
}

// DatosEmpleado extrae la carga de empleado.creado / empleado.actualizado (catálogo 3.1 y 3.2).
func (e *Envelope) DatosEmpleado() (dominio.DatosEmpleado, error) {
	var c cargaEmpleado
	if err := json.Unmarshal(e.Data, &c); err != nil {
		return dominio.DatosEmpleado{}, invalido("data de %s no es un objeto válido: %v", e.Type, err)
	}
	if strings.TrimSpace(c.EmpleadoID) == "" {
		return dominio.DatosEmpleado{}, invalido("data.empleadoId es obligatorio en %s", e.Type)
	}
	if strings.TrimSpace(c.Email) == "" {
		return dominio.DatosEmpleado{}, invalido("data.email es obligatorio en %s", e.Type)
	}
	return dominio.DatosEmpleado{
		EmpleadoID:     strings.TrimSpace(c.EmpleadoID),
		Nombre:         strings.TrimSpace(c.Nombre),
		Apellido:       strings.TrimSpace(c.Apellido),
		Email:          strings.TrimSpace(c.Email),
		Cargo:          strings.TrimSpace(c.Cargo),
		Area:           strings.TrimSpace(c.Area),
		DepartamentoID: strings.TrimSpace(c.DepartamentoID),
	}, nil
}

// EmpleadoIDRetirado extrae data.empleadoId de empleado.retirado (catálogo 3.3).
func (e *Envelope) EmpleadoIDRetirado() (string, error) {
	var c struct {
		EmpleadoID string `json:"empleadoId"`
	}
	if err := json.Unmarshal(e.Data, &c); err != nil {
		return "", invalido("data de %s no es un objeto válido: %v", e.Type, err)
	}
	if strings.TrimSpace(c.EmpleadoID) == "" {
		return "", invalido("data.empleadoId es obligatorio en %s", e.Type)
	}
	return strings.TrimSpace(c.EmpleadoID), nil
}
