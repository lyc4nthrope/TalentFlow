package eventos

import (
	"encoding/json"
	"time"
)

type Envelope struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Version    int             `json:"version"`
	OccurredAt time.Time       `json:"occurredAt"`
	Producer   string          `json:"producer"`
	Data       json.RawMessage `json:"data"`
}