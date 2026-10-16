package api

import "net/http"

// ConfigurarRutas registra los cuatro endpoints del Reto 5, su documentación y, si se
// entrega, GET /health. No hay registro público: las cuentas nacen de empleado.creado.
func ConfigurarRutas(h *AuthHandler, salud *SaludHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/login", h.Login)
	mux.HandleFunc("POST /auth/recover-password", h.RecoverPassword)
	mux.HandleFunc("POST /auth/reset-password", h.ResetPassword)
	mux.HandleFunc("POST /auth/change-password", h.ChangePassword)
	// Documentación bajo el prefijo del servicio: el Gateway solo enruta /auth/*.
	mux.HandleFunc("GET /auth/docs", servirSwaggerUI)
	mux.HandleFunc("GET /auth/openapi.json", servirOpenAPI)
	if salud != nil {
		mux.HandleFunc("GET /health", salud.Salud)
	}
	return mux
}
