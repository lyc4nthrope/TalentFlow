package dominio

import "time"

type Usuario struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Rol          string    `json:"rol"` // ADMIN o USER
	EmpleadoID   string    `json:"empleadoId"`
	CreatedAt    time.Time `json:"createdAt"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type RequestPasswordResetInput struct {
	Email string `json:"email"`
}

type ChangePasswordInput struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
}

// Payloads para los eventos de RabbitMQ
type EventoAuthResetRequested struct {
	UsuarioID string `json:"usuarioId"`
	Email     string `json:"email"`
	Token     string `json:"token"`
}

type EventoAuthPasswordChanged struct {
	UsuarioID string `json:"usuarioId"`
	Email     string `json:"email"`
}