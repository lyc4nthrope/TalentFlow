// Package eventos define el envelope común del Catálogo de Eventos (sección 2).
package eventos

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Productor es el valor del campo "producer" de todo lo que publica este servicio.
const Productor = "auth-service"

type Envelope struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Version    int             `json:"version"`
	OccurredAt string          `json:"occurredAt"`
	Producer   string          `json:"producer"`
	Data       json.RawMessage `json:"data"`
}

// NuevoSobre arma el envelope del catálogo: id UUID del mensaje, versión 1 y
// occurredAt en UTC (ISO-8601, sin milisegundos, como el resto del ecosistema).
func NuevoSobre(tipo string, datos any, ahora time.Time) (Envelope, error) {
	cuerpo, err := json.Marshal(datos)
	if err != nil {
		return Envelope{}, fmt.Errorf("serializar data de %s: %w", tipo, err)
	}
	id, err := uuid.NewRandom()
	if err != nil {
		return Envelope{}, fmt.Errorf("generar id del evento: %w", err)
	}
	return Envelope{
		ID:         id.String(),
		Type:       tipo,
		Version:    1,
		OccurredAt: ahora.UTC().Format(time.RFC3339),
		Producer:   Productor,
		Data:       cuerpo,
	}, nil
}
