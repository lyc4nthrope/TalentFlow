package api

import (
	"context"
	"net/http"
	"time"
)

// timeoutPingBD es menor que el tiempo que el Gateway espera por cada /health (3 s): así,
// con la BD caída el Gateway recibe la explicación ("db DOWN") y no un "No responde".
const timeoutPingBD = 2 * time.Second

// Pinger comprueba la conexión con la base de datos (pgxpool.Pool lo cumple).
type Pinger interface {
	Ping(ctx context.Context) error
}

// EstadoBroker informa si el consumidor está conectado al broker ("UP" / "DOWN").
type EstadoBroker interface {
	Estado() string
}

// SaludHandler responde GET /health con el formato del ecosistema (el mismo que leen el
// /health agregado del Gateway y el healthcheck de Docker).
type SaludHandler struct {
	bd     Pinger
	broker EstadoBroker
	ahora  func() time.Time
}

func NuevoSaludHandler(bd Pinger, broker EstadoBroker) *SaludHandler {
	return &SaludHandler{bd: bd, broker: broker, ahora: time.Now}
}

// Salud: 200 si la BD responde, 503 si no. Como en el resto del ecosistema, el broker se
// reporta pero no tumba el estado: sin broker el servicio sigue atendiendo login y
// contraseñas, y el consumidor se reconecta solo.
func (s *SaludHandler) Salud(w http.ResponseWriter, r *http.Request) {
	ctx, cancelar := context.WithTimeout(r.Context(), timeoutPingBD)
	defer cancelar()

	db := "UP"
	if err := s.bd.Ping(ctx); err != nil {
		db = "DOWN"
	}
	status, codigo := "UP", http.StatusOK
	if db == "DOWN" {
		status, codigo = "DOWN", http.StatusServiceUnavailable
	}
	responderJSON(w, codigo, map[string]any{
		"status":    status,
		"service":   "auth-service",
		"timestamp": s.ahora().UTC().Format(time.RFC3339),
		"components": map[string]string{
			"app":    "UP",
			"db":     db,
			"broker": s.broker.Estado(),
		},
	})
}
