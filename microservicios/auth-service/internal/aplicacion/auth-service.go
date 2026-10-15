package aplicacion

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/eventos"
	jwtinfra "github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/infra/jwt"
	"golang.org/x/crypto/bcrypt"
)

type EventPublisher interface {
	Publicar(ctx context.Context, routingKey string, sobre eventos.Envelope) error
}

type TokenGenerator interface {
	GenerarToken(usuarioID, email string, roles []string, empleadoID string) (string, error)
	ValidarToken(tokenString string) (*jwtinfra.TokenClaims, error)
}

// UsuarioRepositorio ampliado para persistencia real en DB
type UsuarioRepositorio interface {
	ObtenerPorEmail(ctx context.Context, email string) (*dominio.Usuario, error)
	ObtenerPorID(ctx context.Context, usuarioID string) (*dominio.Usuario, error)
	ObtenerPorEmpleadoID(ctx context.Context, empleadoID string) (*dominio.Usuario, error)
	ObtenerPorTokenRecuperacion(ctx context.Context, token string) (*dominio.Usuario, error)
	CrearUsuario(ctx context.Context, usuario *dominio.Usuario) error
	GuardarTokenRecuperacion(ctx context.Context, usuarioID, token string) error
	ActualizarPasswordYEstado(ctx context.Context, usuarioID, nuevoHash, nuevoEstado string) error
	ActualizarPassword(ctx context.Context, usuarioID, nuevoHash string) error
	ActualizarEstado(ctx context.Context, usuarioID, nuevoEstado string) error
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
func (s *AuthService) CrearCuentaDesdeEmpleado(ctx context.Context, email, nombre, apellido, empleadoID string) error {
	email = strings.TrimSpace(email)
	empleadoID = strings.TrimSpace(empleadoID)
	if email == "" || empleadoID == "" {
		return errors.New("email y empleadoId obligatorios")
	}

	usuarioExistente, err := s.repo.ObtenerPorEmail(ctx, email)
	if err != nil {
		return fmt.Errorf("error buscando usuario: %w", err)
	}
	if usuarioExistente != nil {
		if strings.TrimSpace(usuarioExistente.Estado) == dominio.EstadoActiva {
			return nil
		}
		if strings.TrimSpace(usuarioExistente.TokenRecuperacion) == "" {
			token := fmt.Sprintf("reset_%d", time.Now().UnixNano())
			if err := s.repo.GuardarTokenRecuperacion(ctx, usuarioExistente.ID, token); err != nil {
				return fmt.Errorf("error regenerando token del usuario: %w", err)
			}
			usuarioExistente.TokenRecuperacion = token
		}
		return s.publicarUsuarioCreado(ctx, usuarioExistente.ID, usuarioExistente.Email, usuarioExistente.TokenRecuperacion, empleadoID, nombre, apellido)
	}

	claveTemporal := "Cambio123!"
	hash, err := bcrypt.GenerateFromPassword([]byte(claveTemporal), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("error generando hash temporal: %w", err)
	}

	token := fmt.Sprintf("reset_%d", time.Now().UnixNano())
	usuario := &dominio.Usuario{
		ID:                uuid.NewString(),
		Email:             email,
		PasswordHash:      string(hash),
		Roles:             []string{"USER"},
		Estado:            dominio.EstadoInactiva,
		EmpleadoID:        empleadoID,
		TokenRecuperacion: token,
	}
	if err := s.repo.CrearUsuario(ctx, usuario); err != nil {
		return fmt.Errorf("error creando usuario para empleado: %w", err)
	}

	return s.publicarUsuarioCreado(ctx, usuario.ID, usuario.Email, usuario.TokenRecuperacion, empleadoID, nombre, apellido)
}

func (s *AuthService) publicarUsuarioCreado(ctx context.Context, usuarioID, email, token, empleadoID, nombre, apellido string) error {
	payload, err := json.Marshal(dominio.EventoUsuarioCreado{
		UsuarioID:  usuarioID,
		EmpleadoID: strings.TrimSpace(empleadoID),
		Email:      email,
		Token:      token,
		Nombre:     nombre,
		Apellido:   apellido,
	})
	if err != nil {
		return fmt.Errorf("error serializando payload: %w", err)
	}

	sobre := eventos.Envelope{
		ID:         fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Type:       "usuario.creado",
		Version:    1,
		OccurredAt: time.Now().UTC(),
		Producer:   "auth-service",
		Data:       payload,
	}

	return s.publisher.Publicar(ctx, "usuario.creado", sobre)
}

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

	return s.tokenProv.GenerarToken(usuario.ID, usuario.Email, []string(usuario.Roles), usuario.EmpleadoID)
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

const (
	// EstadoActiva indica que la cuenta está activa y puede iniciar sesión.
	EstadoActiva = "ACTIVA"
	// EstadoInactiva indica que la cuenta está inactiva y no puede iniciar sesión.
	EstadoInactiva = "INACTIVA"
	// EstadoSuspendiddaTemporal indica que la cuenta está suspendida temporalmente y no puede iniciar sesión.
	EstadoSuspendiddaTemporal = "SUSPENDIDA_TEMPORAL"
	// EstadoDesactivadaPermanente indica que la cuenta está desactivada permanentemente y no puede iniciar sesión.
	EstadoDesactivadaPermanente = "DESACTIVADA_PERMANENTE"
)

// SuspenderCuentaPorVacaciones cambia el estado a SUSPENDIDA_TEMPORAL y emite el evento cuenta.desactivada
func (s *AuthService) SuspenderCuentaPorVacaciones(ctx context.Context, empleadoID string) error {
	empleadoID = strings.TrimSpace(empleadoID)
	if empleadoID == "" {
		return errors.New("empleadoID es obligatorio")
	}

	usuario, err := s.repo.ObtenerPorEmpleadoID(ctx, empleadoID)
	if err != nil || usuario == nil {
		return fmt.Errorf("usuario no encontrado para empleado %s: %w", empleadoID, err)
	}

	// Si ya está deshabilitado permanentemente, no alteramos su estado
	if usuario.Estado == dominio.EstadoDesactivadaPermanente {
		return nil
	}

	if err := s.repo.ActualizarEstado(ctx, usuario.ID, dominio.EstadoSuspendidaTemporal); err != nil {
		return fmt.Errorf("error suspendiendo cuenta por vacaciones: %w", err)
	}

	return s.publicarCuentaDesactivada(ctx, usuario.ID, usuario.Email, "VACACIONES", false)
}

// ReactivarCuentaPorFinVacaciones reactiva la cuenta solo si no fue desactivada permanentemente
func (s *AuthService) ReactivarCuentaPorFinVacaciones(ctx context.Context, empleadoID string) error {
	empleadoID = strings.TrimSpace(empleadoID)
	if empleadoID == "" {
		return errors.New("empleadoID es obligatorio")
	}

	usuario, err := s.repo.ObtenerPorEmpleadoID(ctx, empleadoID)
	if err != nil || usuario == nil {
		return fmt.Errorf("usuario no encontrado para empleado %s: %w", empleadoID, err)
	}

	// CASO BORDE OBLIGATORIO: Si fue retirado durante las vacaciones, NO reactivar
	if usuario.Estado == dominio.EstadoDesactivadaPermanente {
		return nil
	}

	if err := s.repo.ActualizarEstado(ctx, usuario.ID, dominio.EstadoActiva); err != nil {
		return fmt.Errorf("error reactivando cuenta por fin de vacaciones: %w", err)
	}

	return s.publicarCuentaActivada(ctx, usuario.ID, usuario.Email, "FIN_VACACIONES")
}

// DesactivarCuentaPermanente cambia el estado a DESACTIVADA_PERMANENTE e inhabilita logins futuros
func (s *AuthService) DesactivarCuentaPermanente(ctx context.Context, empleadoID string) error {
	empleadoID = strings.TrimSpace(empleadoID)
	if empleadoID == "" {
		return errors.New("empleadoID es obligatorio")
	}

	usuario, err := s.repo.ObtenerPorEmpleadoID(ctx, empleadoID)
	if err != nil || usuario == nil {
		return fmt.Errorf("usuario no encontrado para empleado %s: %w", empleadoID, err)
	}

	if err := s.repo.ActualizarEstado(ctx, usuario.ID, dominio.EstadoDesactivadaPermanente); err != nil {
		return fmt.Errorf("error desactivando cuenta permanentemente: %w", err)
	}

	return s.publicarCuentaDesactivada(ctx, usuario.ID, usuario.Email, "RETIRO", true)
}

// Métodos auxiliares de publicación de eventos
func (s *AuthService) publicarCuentaDesactivada(ctx context.Context, usuarioID, email, motivo string, permanente bool) error {
	payload, err := json.Marshal(map[string]interface{}{
		"usuarioId":  usuarioID,
		"email":      email,
		"motivo":     motivo,
		"permanente": permanente,
	})
	if err != nil {
		return fmt.Errorf("error serializando payload cuenta.desactivada: %w", err)
	}

	sobre := eventos.Envelope{
		ID:         fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Type:       "cuenta.desactivada",
		Version:    1,
		OccurredAt: time.Now().UTC(),
		Producer:   "auth-service",
		Data:       payload,
	}

	return s.publisher.Publicar(ctx, "cuenta.desactivada", sobre)
}

func (s *AuthService) publicarCuentaActivada(ctx context.Context, usuarioID, email, motivo string) error {
	payload, err := json.Marshal(map[string]interface{}{
		"usuarioId": usuarioID,
		"email":     email,
		"motivo":    motivo,
	})
	if err != nil {
		return fmt.Errorf("error serializando payload cuenta.activada: %w", err)
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
