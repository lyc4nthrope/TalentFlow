// Package api expone los endpoints HTTP del auth-service.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/aplicacion"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
)

// Casos de uso que usan los handlers (implementados por aplicacion.AuthService).
type CasosDeUso interface {
	Login(ctx context.Context, email, password string) (string, error)
	SolicitarRecuperacion(ctx context.Context, email string) error
	RestablecerPassword(ctx context.Context, token, nueva string) error
	CambiarPassword(ctx context.Context, tokenAcceso, actual, nueva string) error
}

// erroresConocidos traduce cada error de negocio a su código HTTP; su mensaje es
// genérico y seguro de mostrar. Lo que no esté aquí es un fallo interno: 500 con un
// mensaje genérico y el detalle solo en el log (Reto 5, D16).
var erroresConocidos = []struct {
	err    error
	codigo int
}{
	{dominio.ErrPoliticaPassword, http.StatusBadRequest},
	{aplicacion.ErrSolicitudInvalida, http.StatusBadRequest},
	{aplicacion.ErrPasswordRepetida, http.StatusBadRequest},
	// La contraseña actual incorrecta es un dato inválido de quien ya está
	// autenticado: 400. El 401 queda para el token de acceso inválido o vencido.
	{aplicacion.ErrPasswordActual, http.StatusBadRequest},
	{aplicacion.ErrCredencialesInvalidas, http.StatusUnauthorized},
	{aplicacion.ErrTokenInvalido, http.StatusUnauthorized},
	{aplicacion.ErrNoAutenticado, http.StatusUnauthorized},
}

// plazoRecuperacion limita el trabajo en segundo plano de una recuperación.
const plazoRecuperacion = 10 * time.Second

type AuthHandler struct {
	casos CasosDeUso
	// enSegundoPlano ejecuta trabajo sin bloquear la respuesta (en las pruebas se
	// reemplaza para controlar cuándo corre).
	enSegundoPlano func(func())
}

func NuevoAuthHandler(casos CasosDeUso) *AuthHandler {
	return &AuthHandler{casos: casos, enSegundoPlano: func(f func()) { go f() }}
}

const tamanoMaximoCuerpo = 64 << 10

type solicitudLogin struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type solicitudRecuperacion struct {
	Email string `json:"email"`
}

type solicitudReset struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

type solicitudCambio struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func leerJSON(w http.ResponseWriter, r *http.Request, destino any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, tamanoMaximoCuerpo)
	if err := json.NewDecoder(r.Body).Decode(destino); err != nil {
		responderError(w, r, http.StatusBadRequest, "Cuerpo JSON inválido")
		return false
	}
	return true
}

func (h *AuthHandler) responderFallo(w http.ResponseWriter, r *http.Request, err error) {
	for _, conocido := range erroresConocidos {
		if errors.Is(err, conocido.err) {
			responderError(w, r, conocido.codigo, conocido.err.Error())
			return
		}
	}
	registrarError(r.Context(), "error interno en %s: %v", r.URL.Path, err)
	responderError(w, r, http.StatusInternalServerError, "Error interno del servidor")
}

// Login: POST /auth/login {email (o username), password} → {token}.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req solicitudLogin
	if !leerJSON(w, r, &req) {
		return
	}
	email := req.Email
	if email == "" {
		email = req.Username
	}
	token, err := h.casos.Login(r.Context(), email, req.Password)
	if err != nil {
		h.responderFallo(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, map[string]string{"token": token, "tipo": "Bearer"})
}

// RecoverPassword: POST /auth/recover-password {email}. Responde 202 de inmediato,
// exista o no la cuenta, y hace la búsqueda, el token y la publicación en segundo
// plano: así ni la respuesta ni su tiempo revelan qué emails existen (D16).
func (h *AuthHandler) RecoverPassword(w http.ResponseWriter, r *http.Request) {
	var req solicitudRecuperacion
	if !leerJSON(w, r, &req) {
		return
	}
	// El trabajo sigue aunque el cliente cierre la conexión, con un plazo propio.
	base := context.WithoutCancel(r.Context())
	h.enSegundoPlano(func() {
		ctx, cancelar := context.WithTimeout(base, plazoRecuperacion)
		defer cancelar()
		if err := h.casos.SolicitarRecuperacion(ctx, req.Email); err != nil {
			registrarError(r.Context(), "error procesando recuperación: %v", err)
		}
	})
	responderJSON(w, http.StatusAccepted, map[string]string{
		"message": "Si el correo corresponde a una cuenta, recibirá las instrucciones para restablecer la contraseña",
	})
}

// ResetPassword: POST /auth/reset-password {token, newPassword}.
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req solicitudReset
	if !leerJSON(w, r, &req) {
		return
	}
	if err := h.casos.RestablecerPassword(r.Context(), req.Token, req.NewPassword); err != nil {
		h.responderFallo(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, map[string]string{"message": "Contraseña establecida correctamente"})
}

// ChangePassword: POST /auth/change-password {currentPassword, newPassword}; requiere
// Authorization: Bearer <token de acceso>.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || strings.TrimSpace(token) == "" {
		responderError(w, r, http.StatusUnauthorized, "Token de acceso requerido")
		return
	}
	var req solicitudCambio
	if !leerJSON(w, r, &req) {
		return
	}
	if err := h.casos.CambiarPassword(r.Context(), strings.TrimSpace(token), req.CurrentPassword, req.NewPassword); err != nil {
		h.responderFallo(w, r, err)
		return
	}
	responderJSON(w, http.StatusOK, map[string]string{"message": "Contraseña actualizada correctamente"})
}

func responderJSON(w http.ResponseWriter, codigo int, cuerpo any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(codigo)
	_ = json.NewEncoder(w).Encode(cuerpo)
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
