package postgres

// Pruebas de integración contra un PostgreSQL real. Se omiten si no se define
// AUTH_TEST_DATABASE_URL (BD desechable: cada prueba recrea la tabla), p. ej.:
//
//	docker run --rm -d --name pg-auth-test -e POSTGRES_PASSWORD=test -p 55432:5432 postgres:16-alpine
//	AUTH_TEST_DATABASE_URL=postgres://postgres:test@localhost:55432/postgres?sslmode=disable go test ./internal/infra/postgres/

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/aplicacion"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
)

func repoReal(t *testing.T) *Repositorio {
	t.Helper()
	url := os.Getenv("AUTH_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("AUTH_TEST_DATABASE_URL no definida: se omiten las pruebas contra PostgreSQL")
	}
	ctx := context.Background()
	pool, err := Conectar(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	esquema, err := os.ReadFile("../../../schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "DROP TABLE IF EXISTS usuarios, eventos_procesados"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(esquema)); err != nil {
		t.Fatalf("schema.sql no se pudo aplicar: %v", err)
	}
	return NuevoRepositorio(pool)
}

var bg = context.Background()

// pendiente arma una cuenta PENDIENTE_ACTIVACION; con tokenHash vacío, sin token.
func pendiente(empleadoID, email, tokenHash string, expira time.Time) *dominio.Usuario {
	u := &dominio.Usuario{Sujeto: empleadoID, EmpleadoID: empleadoID, Email: email, Rol: dominio.RolUser,
		Estado: dominio.EstadoPendienteActivacion, TokenHash: tokenHash, TokenExpiraEn: expira}
	if tokenHash != "" {
		u.TokenTipo = dominio.TokenActivacion
	}
	return u
}

func TestCrearYBuscar(t *testing.T) {
	r := repoReal(t)
	u := pendiente("E001", "Juan@Empresa.com", dominio.HashToken("t1"), time.Now().Add(time.Hour))
	if err := r.Crear(bg, u); err != nil || u.ID == "" {
		t.Fatalf("crear: %v (id %q)", err, u.ID)
	}
	for nombre, buscar := range map[string]func() (*dominio.Usuario, error){
		"email (sin distinguir mayúsculas)": func() (*dominio.Usuario, error) { return r.BuscarPorEmail(bg, "JUAN@empresa.com") },
		"empleadoId":                        func() (*dominio.Usuario, error) { return r.BuscarPorEmpleadoID(bg, "E001") },
		"sujeto":                            func() (*dominio.Usuario, error) { return r.BuscarPorSujeto(bg, "E001") },
		"hash del token":                    func() (*dominio.Usuario, error) { return r.BuscarPorTokenHash(bg, dominio.HashToken("t1")) },
	} {
		got, err := buscar()
		if err != nil || got == nil || got.ID != u.ID || got.Email != "juan@empresa.com" || got.PasswordHash != "" ||
			got.TokenTipo != dominio.TokenActivacion {
			t.Fatalf("buscar por %s: %+v %v", nombre, got, err)
		}
	}
	if got, err := r.BuscarPorEmail(bg, "nadie@empresa.com"); got != nil || err != nil {
		t.Fatalf("inexistente: %+v %v", got, err)
	}
}

func TestRestriccionesDeLaBD(t *testing.T) {
	r := repoReal(t)
	_ = r.Crear(bg, pendiente("E001", "a@b.com", "", time.Time{}))
	if err := r.Crear(bg, pendiente("E002", "A@B.com", "", time.Time{})); !errors.Is(err, aplicacion.ErrUsuarioDuplicado) {
		t.Fatalf("email duplicado: %v", err)
	}
	if err := r.Crear(bg, pendiente(strings.Repeat("E", 37), "c@d.com", "", time.Time{})); !errors.Is(err, aplicacion.ErrRechazoPermanente) {
		t.Fatalf("empleadoId demasiado largo: %v", err)
	}
	activaSinPassword := pendiente("E003", "e@f.com", "", time.Time{})
	activaSinPassword.Estado = dominio.EstadoActiva
	if err := r.Crear(bg, activaSinPassword); !errors.Is(err, aplicacion.ErrRechazoPermanente) {
		t.Fatalf("ACTIVA sin contraseña: %v", err)
	}
	tokenIncompleto := pendiente("E004", "g@h.com", dominio.HashToken("x"), time.Time{})
	if err := r.Crear(bg, tokenIncompleto); !errors.Is(err, aplicacion.ErrRechazoPermanente) {
		t.Fatalf("token sin expiración: %v", err)
	}
}

func TestConsumirTokenEsDeUnSoloUsoYVuelveAComprobarEnLaBD(t *testing.T) {
	r := repoReal(t)
	vigente := pendiente("E001", "a@b.com", dominio.HashToken("t1"), time.Now().Add(time.Hour))
	vencido := pendiente("E002", "c@d.com", dominio.HashToken("t2"), time.Now().Add(-time.Minute))
	_ = r.Crear(bg, vigente)
	_ = r.Crear(bg, vencido)

	if ok, err := r.ConsumirToken(bg, vigente.ID, vigente.TokenHash, dominio.EstadoActiva, "hash", dominio.EstadoActiva); ok || err != nil {
		t.Fatalf("con un estado esperado distinto se aplicó: %v %v", ok, err)
	}
	if ok, err := r.ConsumirToken(bg, vencido.ID, vencido.TokenHash, dominio.EstadoPendienteActivacion, "hash", dominio.EstadoActiva); ok || err != nil {
		t.Fatalf("se consumió un token vencido: %v %v", ok, err)
	}
	if ok, err := r.ConsumirToken(bg, vigente.ID, vigente.TokenHash, dominio.EstadoPendienteActivacion, "hash", dominio.EstadoActiva); !ok || err != nil {
		t.Fatalf("no se consumió un token vigente: %v %v", ok, err)
	}
	got, _ := r.BuscarPorSujeto(bg, "E001")
	if got.Estado != dominio.EstadoActiva || got.PasswordHash != "hash" || got.TokenHash != "" || !got.TokenExpiraEn.IsZero() {
		t.Fatalf("tras consumir: %+v", got)
	}
	if ok, _ := r.ConsumirToken(bg, vigente.ID, vigente.TokenHash, dominio.EstadoActiva, "otro", dominio.EstadoActiva); ok {
		t.Fatal("el mismo token se consumió dos veces")
	}
}

func TestGuardarTokenReemplazaYActualizarPasswordLoAnula(t *testing.T) {
	r := repoReal(t)
	u := pendiente("E001", "a@b.com", dominio.HashToken("t1"), time.Now().Add(time.Hour))
	_ = r.Crear(bg, u)
	if err := r.GuardarToken(bg, u.ID, dominio.HashToken("t2"), dominio.TokenRecuperacion, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if viejo, _ := r.BuscarPorTokenHash(bg, dominio.HashToken("t1")); viejo != nil {
		t.Fatal("el token anterior sigue registrado")
	}
	if err := r.ActualizarPassword(bg, u.ID, "nuevo-hash"); err != nil {
		t.Fatal(err)
	}
	got, _ := r.BuscarPorSujeto(bg, "E001")
	if got.PasswordHash != "nuevo-hash" || got.TokenHash != "" || got.TokenTipo != "" || !got.TokenExpiraEn.IsZero() {
		t.Fatalf("ActualizarPassword no anuló el token: %+v", got)
	}
	if err := r.ActualizarPassword(bg, "00000000-0000-0000-0000-000000000000", "x"); err == nil {
		t.Fatal("actualizar una cuenta inexistente no falló")
	}
}
