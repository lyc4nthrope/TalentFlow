package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/aplicacion"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
)

type AuthHandler struct {
	service *aplicacion.AuthService
}

func NuevoAuthHandler(service *aplicacion.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req dominio.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "solicitud inválida", http.StatusBadRequest)
		return
	}

	token, err := h.service.Login(r.Context(), req)
	if err != nil {
		http.Error(w, "credenciales incorrectas", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dominio.LoginResponse{Token: token})
}

func (h *AuthHandler) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req dominio.RequestPasswordResetInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "solicitud inválida", http.StatusBadRequest)
		return
	}

	if err := h.service.SolicitudRecuperacionClave(r.Context(), req.Email); err != nil {
		http.Error(w, "error al procesar la solicitud", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"mensaje":"Solicitud de recuperación enviada"}`))
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
    var req dominio.ResetPasswordInput
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "solicitud inválida", http.StatusBadRequest)
        return
    }

    if err := h.service.ConfirmarCambioClave(r.Context(), req.Token, req.NewPassword); err != nil {
        http.Error(w, "error al cambiar la contraseña", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte(`{"mensaje":"Contraseña restablecida correctamente"}`))
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req dominio.ChangePasswordInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "solicitud inválida", http.StatusBadRequest)
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		http.Error(w, "currentPassword y newPassword son obligatorios", http.StatusBadRequest)
		return
	}

	authorization := r.Header.Get("Authorization")
	if authorization == "" || len(authorization) < 7 || !strings.HasPrefix(authorization, "Bearer ") {
		http.Error(w, "token faltante", http.StatusUnauthorized)
		return
	}

	if err := h.service.CambiarPasswordAutenticada(r.Context(), strings.TrimSpace(authorization[7:]), req.CurrentPassword, req.NewPassword); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"mensaje":"Contraseña actualizada exitosamente"}`))
}