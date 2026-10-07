package dominio

import (
	"encoding/json"
	"time"

	"github.com/lib/pq"
)

// Estados de cuenta requeridos por el Reto 5 para manejar el ciclo de vida
const (
	EstadoActiva               = "ACTIVA"
	EstadoInactiva             = "INACTIVA"
	EstadoSuspendidaTemporal   = "SUSPENDIDA_TEMPORAL"   // Por vacaciones
	EstadoDesactivadaPermanente = "DESACTIVADA_PERMANENTE" // Por retiro de empleado
)

type Usuario struct {
	ID                string         `json:"id"`
	Email             string         `json:"email"`
	PasswordHash      string         `json:"-"`
	Roles             pq.StringArray `gorm:"type:text[]" json:"roles"` // Soporta ["ADMIN"], ["USER"], etc.
	Estado            string         `json:"estado"` // ACTIVA | INACTIVA | SUSPENDIDA_TEMPORAL | DESACTIVADA_PERMANENTE
	EmpleadoID        string         `json:"empleadoId"`
	TokenRecuperacion string         `json:"-"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
}

// DTOs para solicitudes y respuestas HTTP

type LoginRequest struct {
	Email    string `json:"email,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password"`
}

func (r *LoginRequest) UnmarshalJSON(data []byte) error {
	type Alias LoginRequest
	var aux struct {
		Email    string `json:"email"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	r.Email = aux.Email
	r.Username = aux.Username
	r.Password = aux.Password
	if r.Email == "" {
		r.Email = r.Username
	}
	if r.Username == "" {
		r.Username = r.Email
	}
	return nil
}

type LoginResponse struct {
	Token string `json:"token"`
}

type RequestPasswordResetInput struct {
	Email string `json:"email"`
}

type ResetPasswordInput struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

type ChangePasswordInput struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// Estructuras para Eventos del Catálogo

type EventoUsuarioRecuperacion struct {
	UsuarioID string `json:"usuarioId"`
	Email     string `json:"email"`
	Token     string `json:"token"`
}

type EventoCuentaActivada struct {
	UsuarioID string `json:"usuarioId"`
	Email     string `json:"email"`
	Motivo    string `json:"motivo,omitempty"`
}

type EventoUsuarioCreado struct {
	UsuarioID string `json:"usuarioId"`
	Email     string `json:"email"`
	Token     string `json:"token"`
}

type EventoCuentaDesactivada struct {
	UsuarioID  string `json:"usuarioId"`
	Email      string `json:"email"`
	Motivo     string `json:"motivo"`     // "RETIRO" | "VACACIONES"
	Permanente bool   `json:"permanente"` // true para retiro, false para vacaciones
}