// Package eventos define el sobre técnico (envelope) común a todos los eventos del
// ecosistema, según el Catálogo de Eventos, sección 2.
package eventos

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Envelope es el sobre de todo mensaje del broker. Data se conserva cruda: su forma
// depende del Type y la interpreta quien consume ese tipo de evento.
type Envelope struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Version    int             `json:"version"`
	OccurredAt time.Time       `json:"occurredAt"`
	Producer   string          `json:"producer"`
	Data       json.RawMessage `json:"data"`
}

// ErrMensajeInvalido marca un mensaje que nunca podrá procesarse (JSON corrupto o
// envelope incompleto). Reintentarlo sería inútil: se descarta.
var ErrMensajeInvalido = errors.New("mensaje inválido")

// LongitudMaximaID coincide con la columna eventos_procesados.id (un UUID ocupa 36).
const LongitudMaximaID = 100

// Parsear decodifica y valida un mensaje. Todos los campos del envelope son
// obligatorios en el catálogo; el id es la clave de deduplicación, así que sin él el
// mensaje no se puede procesar de forma segura.
func Parsear(cuerpo []byte) (Envelope, error) {
	var sobre Envelope
	if err := json.Unmarshal(cuerpo, &sobre); err != nil {
		return Envelope{}, fmt.Errorf("%w: JSON no válido: %v", ErrMensajeInvalido, err)
	}

	var faltantes []string
	if sobre.ID == "" {
		faltantes = append(faltantes, "id")
	} else if len(sobre.ID) > LongitudMaximaID {
		// Sin este límite, un id más largo que la columna haría fallar el INSERT en
		// cada intento y el mensaje se reintentaría para siempre.
		return Envelope{}, fmt.Errorf("%w: el id supera %d caracteres", ErrMensajeInvalido, LongitudMaximaID)
	}
	if sobre.Type == "" {
		faltantes = append(faltantes, "type")
	}
	if sobre.Version < 1 {
		faltantes = append(faltantes, "version")
	}
	if sobre.OccurredAt.IsZero() {
		faltantes = append(faltantes, "occurredAt")
	}
	if sobre.Producer == "" {
		faltantes = append(faltantes, "producer")
	}
	if len(sobre.Data) == 0 || string(sobre.Data) == "null" {
		faltantes = append(faltantes, "data")
	}
	if len(faltantes) > 0 {
		return Envelope{}, fmt.Errorf("%w: faltan campos del envelope %v", ErrMensajeInvalido, faltantes)
	}
	return sobre, nil
}
