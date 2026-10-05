// Package api expone el historial de notificaciones por HTTP (solo lectura).
// Este servicio es puramente reactivo: ningún otro servicio lo invoca por REST; estas
// rutas son para consultar el historial desde fuera (a través del API Gateway).
package api

import (
	"context"
	_ "embed"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/dominio"
)

//go:embed openapi.json
var especificacionOpenAPI []byte

// Consultas es el puerto de lectura que necesita la API.
type Consultas interface {
	Listar(ctx context.Context) ([]dominio.Notificacion, error)
	ListarPorEmpleado(ctx context.Context, empleadoID string) ([]dominio.Notificacion, error)
	Ping(ctx context.Context) error
}

// EstadoBroker informa si el consumidor está conectado a RabbitMQ.
type EstadoBroker interface {
	Estado() string
}

const timeoutPingBD = 2 * time.Second

func NuevoManejador(consultas Consultas, broker EstadoBroker, logger *slog.Logger) http.Handler {
	s := &servidor{consultas: consultas, broker: broker, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /notificaciones", s.listar)
	mux.HandleFunc("GET /notificaciones/{empleadoId}", s.listarPorEmpleado)
	// Documentación bajo el prefijo del servicio: así es alcanzable por el Gateway,
	// que solo enruta /notificaciones/*.
	mux.HandleFunc("GET /notificaciones/docs", s.swaggerUI)
	mux.HandleFunc("GET /notificaciones/openapi.json", s.openapi)
	mux.HandleFunc("GET /health", s.salud)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		responderError(w, r, http.StatusNotFound, "Recurso no encontrado")
	})
	return mux
}

type servidor struct {
	consultas Consultas
	broker    EstadoBroker
	logger    *slog.Logger
}

func (s *servidor) listar(w http.ResponseWriter, r *http.Request) {
	notificaciones, err := s.consultas.Listar(r.Context())
	s.responderLista(w, r, notificaciones, err)
}

func (s *servidor) listarPorEmpleado(w http.ResponseWriter, r *http.Request) {
	notificaciones, err := s.consultas.ListarPorEmpleado(r.Context(), r.PathValue("empleadoId"))
	s.responderLista(w, r, notificaciones, err)
}

func (s *servidor) responderLista(w http.ResponseWriter, r *http.Request, lista []dominio.Notificacion, err error) {
	if err != nil {
		// El detalle queda en el log; al cliente no se le filtran errores internos.
		s.logger.Error("error consultando notificaciones", "error", err, "ruta", r.URL.Path)
		responderError(w, r, http.StatusInternalServerError, "Error interno del servidor")
		return
	}
	if lista == nil {
		lista = []dominio.Notificacion{} // [] y no null en el JSON
	}
	responderJSON(w, http.StatusOK, lista)
}

// salud: DOWN solo si la BD propia no responde. Sin broker el servicio sigue
// sirviendo el historial (degradado): el broker se reporta como componente.
func (s *servidor) salud(w http.ResponseWriter, r *http.Request) {
	ctx, cancelar := context.WithTimeout(r.Context(), timeoutPingBD)
	defer cancelar()

	db := "UP"
	if err := s.consultas.Ping(ctx); err != nil {
		db = "DOWN"
	}
	status, codigo := "UP", http.StatusOK
	if db == "DOWN" {
		status, codigo = "DOWN", http.StatusServiceUnavailable
	}
	responderJSON(w, codigo, map[string]any{
		"status":    status,
		"service":   "notificaciones-service",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"components": map[string]string{
			"app":    "UP",
			"db":     db,
			"broker": s.broker.Estado(),
		},
	})
}

func (s *servidor) openapi(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(especificacionOpenAPI) //nolint:errcheck
}

func (s *servidor) swaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(paginaSwagger)) //nolint:errcheck
}

func responderJSON(w http.ResponseWriter, codigo int, cuerpo any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(codigo)
	json.NewEncoder(w).Encode(cuerpo) //nolint:errcheck
}

// Mismo formato de error que el resto del ecosistema.
func responderError(w http.ResponseWriter, r *http.Request, codigo int, mensaje string) {
	responderJSON(w, codigo, map[string]any{
		"status":    codigo,
		"error":     http.StatusText(codigo),
		"message":   mensaje,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"path":      r.URL.Path,
	})
}

const paginaSwagger = `<!doctype html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <title>Notificaciones - Swagger UI</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({ url: "/notificaciones/openapi.json", dom_id: "#swagger-ui" });
  </script>
</body>
</html>`
