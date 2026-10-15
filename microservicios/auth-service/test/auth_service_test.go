package aplicacion_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/aplicacion"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/eventos"
	jwtinfra "github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/infra/jwt" // <-- AGREGAR ESTA LÍNEA
)

// --- MOCKS ---

type mockRepo struct {
	usuarios map[string]*dominio.Usuario
}

func newMockRepo() *mockRepo {
	return &mockRepo{usuarios: make(map[string]*dominio.Usuario)}
}

func (m *mockRepo) ObtenerPorEmail(ctx context.Context, email string) (*dominio.Usuario, error) {
	for _, u := range m.usuarios {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockRepo) ObtenerPorID(ctx context.Context, id string) (*dominio.Usuario, error) {
	if u, ok := m.usuarios[id]; ok {
		return u, nil
	}
	return nil, nil
}

func (m *mockRepo) ObtenerPorEmpleadoID(ctx context.Context, empID string) (*dominio.Usuario, error) {
	for _, u := range m.usuarios {
		if u.EmpleadoID == empID {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockRepo) ObtenerPorTokenRecuperacion(ctx context.Context, token string) (*dominio.Usuario, error) {
	for _, u := range m.usuarios {
		if u.TokenRecuperacion == token {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockRepo) CrearUsuario(ctx context.Context, usuario *dominio.Usuario) error {
	m.usuarios[usuario.ID] = usuario
	return nil
}

func (m *mockRepo) GuardarTokenRecuperacion(ctx context.Context, usuarioID, token string) error {
	if u, ok := m.usuarios[usuarioID]; ok {
		u.TokenRecuperacion = token
		return nil
	}
	return errors.New("usuario no encontrado")
}

func (m *mockRepo) ActualizarPasswordYEstado(ctx context.Context, usuarioID, nuevoHash, nuevoEstado string) error {
	if u, ok := m.usuarios[usuarioID]; ok {
		u.PasswordHash = nuevoHash
		u.Estado = nuevoEstado
		return nil
	}
	return errors.New("usuario no encontrado")
}

func (m *mockRepo) ActualizarPassword(ctx context.Context, usuarioID, nuevoHash string) error {
	if u, ok := m.usuarios[usuarioID]; ok {
		u.PasswordHash = nuevoHash
		return nil
	}
	return errors.New("usuario no encontrado")
}

func (m *mockRepo) ActualizarEstado(ctx context.Context, usuarioID, nuevoEstado string) error {
	if u, ok := m.usuarios[usuarioID]; ok {
		u.Estado = nuevoEstado
		return nil
	}
	return errors.New("usuario no encontrado")
}

type mockPublisher struct {
	eventosPublicados []eventos.Envelope
}

func (m *mockPublisher) Publicar(ctx context.Context, routingKey string, sobre eventos.Envelope) error {
	m.eventosPublicados = append(m.eventosPublicados, sobre)
	return nil
}

type mockTokenGen struct{}

func (m *mockTokenGen) GenerarToken(uID, email string, roles []string, empID string) (string, error) {
	return "jwt_token_de_prueba", nil
}

func (m *mockTokenGen) ValidarToken(token string) (*jwtinfra.TokenClaims, error) {
	return nil, nil
}

// --- SUITE DE PRUEBAS UNITARIAS ---

func TestSuspenderCuentaPorVacaciones(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	service := aplicacion.NuevoAuthService(pub, &mockTokenGen{}, repo)

	// Dado un usuario activo asociado a un empleado
	user := &dominio.Usuario{
		ID:         "u1",
		EmpleadoID: "emp-101",
		Email:      "test@empresa.com",
		Estado:     dominio.EstadoActiva,
	}
	repo.CrearUsuario(context.Background(), user)

	// Al suspender por vacaciones
	err := service.SuspenderCuentaPorVacaciones(context.Background(), "emp-101")

	// Debe actualizar el estado a SUSPENDIDA_TEMPORAL y emitir evento
	if err != nil {
		t.Fatalf("se esperaba nil y se obtuvo: %v", err)
	}
	if repo.usuarios["u1"].Estado != dominio.EstadoSuspendidaTemporal {
		t.Errorf("esperado estado %s, se obtuvo %s", dominio.EstadoSuspendidaTemporal, repo.usuarios["u1"].Estado)
	}
	if len(pub.eventosPublicados) == 0 || pub.eventosPublicados[len(pub.eventosPublicados)-1].Type != "cuenta.desactivada" {
		t.Errorf("no se publicaciones el evento cuenta.desactivada")
	}
}

func TestReactivarCuentaPorFinVacaciones_Exitoso(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	service := aplicacion.NuevoAuthService(pub, &mockTokenGen{}, repo)

	user := &dominio.Usuario{
		ID:         "u1",
		EmpleadoID: "emp-101",
		Estado:     dominio.EstadoSuspendidaTemporal,
	}
	repo.CrearUsuario(context.Background(), user)

	err := service.ReactivarCuentaPorFinVacaciones(context.Background(), "emp-101")

	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if repo.usuarios["u1"].Estado != dominio.EstadoActiva {
		t.Errorf("esperado %s, obtenido %s", dominio.EstadoActiva, repo.usuarios["u1"].Estado)
	}
}

func TestCasoBorde_NoReactivarSiFueRetirado(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	service := aplicacion.NuevoAuthService(pub, &mockTokenGen{}, repo)

	// Usuario que fue retirado permanentemente durante vacaciones
	user := &dominio.Usuario{
		ID:         "u1",
		EmpleadoID: "emp-102",
		Estado:     dominio.EstadoDesactivadaPermanente,
	}
	repo.CrearUsuario(context.Background(), user)

	// Cuando finalizan sus vacaciones
	err := service.ReactivarCuentaPorFinVacaciones(context.Background(), "emp-102")

	// NO debe fallar pero tampoco cambiar el estado a ACTIVA
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if repo.usuarios["u1"].Estado != dominio.EstadoDesactivadaPermanente {
		t.Errorf("LA CUENTA FUE REACTIVADA ERRÓNEAMENTE. Estado actual: %s", repo.usuarios["u1"].Estado)
	}
}

func TestLogin_RechazarSiEstaSuspendida(t *testing.T) {
	repo := newMockRepo()
	pub := &mockPublisher{}
	service := aplicacion.NuevoAuthService(pub, &mockTokenGen{}, repo)

	user := &dominio.Usuario{
		ID:           "u1",
		Email:        "dev@empresa.com",
		PasswordHash: "$2a$10$...", // Hash simulado
		Estado:       dominio.EstadoSuspendidaTemporal,
	}
	repo.CrearUsuario(context.Background(), user)

	_, err := service.Login(context.Background(), dominio.LoginRequest{
		Email:    "dev@empresa.com",
		Password: "Password123!",
	})

	if err == nil {
		t.Fatal("se esperaba error de acceso denegado por suspensión, pero se permitió el login")
	}
}