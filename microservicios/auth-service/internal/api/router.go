package api

import (
	"net/http"
)

func ConfigurarRutas(h *AuthHandler) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /auth/login", h.Login)
	mux.HandleFunc("POST /auth/request-password-reset", h.RequestPasswordReset)
	mux.HandleFunc("POST /auth/change-password", h.ChangePassword)

	return mux
}