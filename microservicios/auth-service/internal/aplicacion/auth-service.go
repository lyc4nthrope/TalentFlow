package aplicacion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	jwtinfra "github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/infra/jwt"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/eventos"
	"golang.org/x/crypto/bcrypt"
)

type EventPublisher interface {
	Publicar(ctx context.Context, routingKey string, sobre eventos.Envelope) error
}

type TokenGenerator interface {
	GenerarToken(usuarioID, email string, roles []string) (string, error)
	ValidarToken(tokenString string) (*jwtinfra.TokenClaims, error)
}

// UsuarioRepositorio ampliado para persistencia real en DB
type UsuarioRepositorio interface {
	ObtenerPorEmail(ctx context.Context, email string) (*dominio.Usuario, error)
	ObtenerPorID(ctx context.Context, usuarioID string) (*dominio.Usuario, error)
	ObtenerPorTokenRecuperacion(ctx context.Context, token string) (*dominio.Usuario, error)
	GuardarTokenRecuperacion(ctx context.Context, usuarioID, token string) error
	ActualizarPasswordYEstado(ctx context.Context, usuarioID, nuevoHash, nuevoEstado string) error
	ActualizarPassword(ctx context.Context, usuarioID, nuevoHash string) error
}

type AuthService struct {
	publisher EventPublisher
	tokenProv TokenGenerator
	repo      UsuarioRepositorio
}

func NuevoAuthService(publisher EventPublisher, tokenProv TokenGenerator, repo UsuarioRepositorio) *AuthService {
	return &AuthService{
		publisher: publisher,
		tokenProv: tokenProv,
		repo:      repo,
	}
}

// Login valida credenciales reales contra la BD, el hash bcrypt y el estado de la cuenta.
func (s *AuthService) Login(ctx context.Context, req dominio.LoginRequest) (string, error) {
	email := strings.TrimSpace(req.Email)
	if email == "" {
		email = strings.TrimSpace(req.Username)
	}
	password := req.Password

	if email == "" || password == "" {
		return "", errors.New("credenciales incorrectas")
	}

	usuario, err := s.repo.ObtenerPorEmail(ctx, email)
	if err != nil || usuario == nil {
		return "", errors.New("credenciales incorrectas")
	}

	if usuario.Estado != dominio.EstadoActiva && usuario.Estado != "ACTIVO" {
		return "", errors.New("cuenta inactiva o suspendida")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(usuario.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("credenciales incorrectas")
	}

	return s.tokenProv.GenerarToken(usuario.ID, usuario.Email, []string(usuario.Roles))
}

// SolicitudRecuperacionClave publica el evento: usuario.recuperacion
func (s *AuthService) SolicitudRecuperacionClave(ctx context.Context, email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return errors.New("email obligatorio")
	}

	usuario, err := s.repo.ObtenerPorEmail(ctx, email)
	if err != nil || usuario == nil {
		return errors.New("usuario no encontrado")
	}

	tokenRecuperacion := fmt.Sprintf("reset_%d", time.Now().UnixNano())
	if err := s.repo.GuardarTokenRecuperacion(ctx, usuario.ID, tokenRecuperacion); err != nil {
		return fmt.Errorf("error guardando token de recuperación: %w", err)
	}

	payload, err := json.Marshal(dominio.EventoUsuarioRecuperacion{
		UsuarioID: usuario.ID,
		Email:     usuario.Email,
		Token:     tokenRecuperacion,
	})
	if err != nil {
		return fmt.Errorf("error serializando payload: %w", err)
	}

	sobre := eventos.Envelope{
		ID:         fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Type:       "usuario.recuperacion",
		Version:    1,
		OccurredAt: time.Now().UTC(),
		Producer:   "auth-service",
		Data:       payload,
	}

	return s.publisher.Publicar(ctx, "usuario.recuperacion", sobre)
}

// ConfirmarCambioClave actualiza realmente en la BD y publica cuenta.activada
func (s *AuthService) ConfirmarCambioClave(ctx context.Context, token, nuevaClave string) error {
	token = strings.TrimSpace(token)
	nuevaClave = strings.TrimSpace(nuevaClave)

	if token == "" || nuevaClave == "" {
		return errors.New("token o contraseña inválidos")
	}

	// 1. Buscar usuario asociado al token en BD
	usuario, err := s.repo.ObtenerPorTokenRecuperacion(ctx, token)
	if err != nil || usuario == nil {
		return errors.New("token inválido o expirado")
	}

	// 2. Generar hash real
	hash, err := bcrypt.GenerateFromPassword([]byte(nuevaClave), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("error generando hash: %w", err)
	}

	if err := s.repo.ActualizarPasswordYEstado(ctx, usuario.ID, string(hash), dominio.EstadoActiva); err != nil {
		return fmt.Errorf("error actualizando base de datos: %w", err)
	}

	// 4. Publicar evento con datos reales del usuario
	payload, err := json.Marshal(dominio.EventoCuentaActivada{
		UsuarioID: usuario.ID,
		Email:     usuario.Email,
	})
	if err != nil {
		return fmt.Errorf("error serializando payload: %w", err)
	}

	sobre := eventos.Envelope{
		ID:         fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Type:       "cuenta.activada",
		Version:    1,
		OccurredAt: time.Now().UTC(),
		Producer:   "auth-service",
		Data:       payload,
	}

	return s.publisher.Publicar(ctx, "cuenta.activada", sobre)
}

func (s *AuthService) CambiarPasswordAutenticada(ctx context.Context, token, currentPassword, newPassword string) error {
	token = strings.TrimSpace(token)
	currentPassword = strings.TrimSpace(currentPassword)
	newPassword = strings.TrimSpace(newPassword)

	if token == "" || currentPassword == "" || newPassword == "" {
		return errors.New("token, contraseña actual o nueva inválidos")
	}

	claims, err := s.tokenProv.ValidarToken(token)
	if err != nil {
		return fmt.Errorf("token inválido: %w", err)
	}

	usuario, err := s.repo.ObtenerPorID(ctx, claims.UsuarioID)
	if err != nil || usuario == nil {
		return errors.New("usuario no encontrado")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(usuario.PasswordHash), []byte(currentPassword)); err != nil {
		return errors.New("contraseña actual incorrecta")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("error generando hash: %w", err)
	}

	if err := s.repo.ActualizarPassword(ctx, usuario.ID, string(hash)); err != nil {
		return fmt.Errorf("error actualizando contraseña: %w", err)
	}

	return nil
}