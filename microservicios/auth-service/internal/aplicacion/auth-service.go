package aplicacion

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/eventos"
)

type EventPublisher interface {
	Publicar(ctx context.Context, routingKey string, sobre eventos.Envelope) error
}

type TokenGenerator interface {
	GenerarToken(usuarioID, email, rol string) (string, error)
}

type AuthService struct {
	publisher EventPublisher
	tokenProv TokenGenerator
}

func NuevoAuthService(publisher EventPublisher, tokenProv TokenGenerator) *AuthService {
	return &AuthService{
		publisher: publisher,
		tokenProv: tokenProv,
	}
}

// Login valida el acceso y retorna el JWT firmado
func (s *AuthService) Login(ctx context.Context, req dominio.LoginRequest) (string, error) {
	// TODO: Validar hash de contraseña contra la BD
	// Ejemplo mock de usuario autenticado
	usuarioID := "usr_12345"
	rol := "USER"

	return s.tokenProv.GenerarToken(usuarioID, req.Email, rol)
}

// SolicitudRecuperacionClave crea el evento de reseteo para RabbitMQ
func (s *AuthService) SolicitudRecuperacionClave(ctx context.Context, email string) error {
	usuarioID := "usr_12345" // ID obtenido de la BD según el email
	tokenRecuperacion := "tok_reset_abc123"

	payload, err := json.Marshal(dominio.EventoAuthResetRequested{
		UsuarioID: usuarioID,
		Email:     email,
		Token:     tokenRecuperacion,
	})
	if err != nil {
		return fmt.Errorf("error serializando payload: %w", err)
	}

	sobre := eventos.Envelope{
		ID:         fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Type:       "auth.password_reset_requested",
		Version:    1,
		OccurredAt: time.Now().UTC(),
		Producer:   "auth-service",
		Data:       payload,
	}

	return s.publisher.Publicar(ctx, "auth.password_reset_requested", sobre)
}

// ConfirmarCambioClave actualiza la contraseña y notifica el cambio
func (s *AuthService) ConfirmarCambioClave(ctx context.Context, token, nuevaClave string) error {
	// TODO: Validar token de reseteo y actualizar hash en BD
	usuarioID := "usr_12345"
	email := "usuario@empresa.com"

	payload, err := json.Marshal(dominio.EventoAuthPasswordChanged{
		UsuarioID: usuarioID,
		Email:     email,
	})
	if err != nil {
		return fmt.Errorf("error serializando payload: %w", err)
	}

	sobre := eventos.Envelope{
		ID:         fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Type:       "auth.password_changed",
		Version:    1,
		OccurredAt: time.Now().UTC(),
		Producer:   "auth-service",
		Data:       payload,
	}

	return s.publisher.Publicar(ctx, "auth.password_changed", sobre)
}