package api

import "net/http"

// ConfigurarRutas registra los cuatro endpoints del Reto 5. No hay registro público:
// las cuentas nacen de empleado.creado.
func ConfigurarRutas(h *AuthHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/login", h.Login)
	mux.HandleFunc("POST /auth/recover-password", h.RecoverPassword)
	mux.HandleFunc("POST /auth/reset-password", h.ResetPassword)
	mux.HandleFunc("POST /auth/change-password", h.ChangePassword)
	return mux
}
