package postgres

// Pruebas de integración del ciclo de vida (F3) contra PostgreSQL real: UPDATEs
// condicionales, la restricción de vacaciones_suspension y la deduplicación en la
// misma transacción que el efecto. Ver repositorio_test.go para ejecutarlas.

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/aplicacion"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
)

func activa(t *testing.T, r *Repositorio, empleadoID string) *dominio.Usuario {
	t.Helper()
	u := &dominio.Usuario{Sujeto: empleadoID, EmpleadoID: empleadoID, Email: empleadoID + "@empresa.com",
		PasswordHash: "$2a$04$hash", Rol: dominio.RolUser, Estado: dominio.EstadoActiva}
	if err := r.Crear(bg, u); err != nil {
		t.Fatal(err)
	}
	return u
}

func leer(t *testing.T, r *Repositorio, empleadoID string) *dominio.Usuario {
	t.Helper()
	u, err := r.BuscarPorEmpleadoID(bg, empleadoID)
	if err != nil || u == nil {
		t.Fatalf("leer %s: %v", empleadoID, err)
	}
	return u
}

func exigir(t *testing.T, nombre string, obtenido, esperado bool, err error) {
	t.Helper()
	if err != nil || obtenido != esperado {
		t.Fatalf("%s: cambió=%v err=%v; se esperaba cambió=%v", nombre, obtenido, err, esperado)
	}
}

func TestCicloVacacionesConUpdatesCondicionales(t *testing.T) {
	r := repoReal(t)
	u := activa(t, r, "E001")

	ok, err := r.Suspender(bg, u.ID, "V-A")
	exigir(t, "suspender ACTIVA", ok, true, err)
	if got := leer(t, r, "E001"); got.Estado != dominio.EstadoSuspendidaTemporal || got.VacacionesSuspension != "V-A" {
		t.Fatalf("tras suspender: %+v", got)
	}
	ok, err = r.Suspender(bg, u.ID, "V-X")
	exigir(t, "suspender ya suspendida", ok, false, err)
	ok, err = r.CambiarSuspension(bg, u.ID, "V-B")
	exigir(t, "cambiar suspensión", ok, true, err)
	ok, err = r.Reactivar(bg, u.ID, "V-A")
	exigir(t, "reactivar con otro id", ok, false, err)
	ok, err = r.Reactivar(bg, u.ID, "V-B")
	exigir(t, "reactivar con su id", ok, true, err)
	if got := leer(t, r, "E001"); got.Estado != dominio.EstadoActiva || got.VacacionesSuspension != "" {
		t.Fatalf("tras reactivar: %+v", got)
	}
	ok, err = r.CambiarSuspension(bg, u.ID, "V-C")
	exigir(t, "cambiar suspensión de una activa", ok, false, err)
}

func TestRetirarEsPermanenteYBorraTokenYSuspension(t *testing.T) {
	r := repoReal(t)
	u := activa(t, r, "E001")
	if err := r.GuardarToken(bg, u.ID, dominio.HashToken("t"), dominio.TokenRecuperacion, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Suspender(bg, u.ID, "V-A"); err != nil {
		t.Fatal(err)
	}

	ok, err := r.Retirar(bg, u.ID)
	exigir(t, "retirar", ok, true, err)
	got := leer(t, r, "E001")
	if got.Estado != dominio.EstadoDesactivadaPermanente || got.VacacionesSuspension != "" || got.TokenHash != "" || got.TokenTipo != "" {
		t.Fatalf("tras retirar: %+v", got)
	}
	ok, err = r.Retirar(bg, u.ID)
	exigir(t, "retirar dos veces", ok, false, err)
	ok, err = r.Reactivar(bg, u.ID, "V-A")
	exigir(t, "reactivar a un retirado (caso borde)", ok, false, err)
	ok, err = r.Suspender(bg, u.ID, "V-B")
	exigir(t, "suspender a un retirado", ok, false, err)
}

func TestSuspenderNoTocaUnaCuentaPendiente(t *testing.T) {
	r := repoReal(t)
	u := pendiente("E002", "e2@empresa.com", "", time.Time{})
	if err := r.Crear(bg, u); err != nil {
		t.Fatal(err)
	}
	ok, err := r.Suspender(bg, u.ID, "V-A")
	exigir(t, "suspender pendiente", ok, false, err)
}

func TestLaBDSoloAdmiteSuspensionEnUnaCuentaSuspendida(t *testing.T) {
	r := repoReal(t)
	u := activa(t, r, "E001")
	_, err := r.pool.Exec(bg, `UPDATE usuarios SET vacaciones_suspension = 'V-A' WHERE id = $1`, u.ID)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "chk_suspension_solo_suspendida" {
		t.Fatalf("se esperaba violar chk_suspension_solo_suspendida, err = %v", err)
	}
}

func TestProcesarUnaVezRegistraElIdYDescartaElDuplicado(t *testing.T) {
	r := repoReal(t)
	u := activa(t, r, "E001")
	llamadas := 0
	aplicar := func(repo aplicacion.Repositorio) error {
		llamadas++
		_, err := repo.Suspender(bg, u.ID, "V-A")
		return err
	}

	dup, err := r.ProcesarUnaVez(bg, "evt-1", aplicar)
	if dup || err != nil {
		t.Fatalf("primera vez: dup=%v err=%v", dup, err)
	}
	dup, err = r.ProcesarUnaVez(bg, "evt-1", aplicar)
	if !dup || err != nil || llamadas != 1 {
		t.Fatalf("reentrega: dup=%v err=%v llamadas=%d", dup, err, llamadas)
	}
	if got := leer(t, r, "E001"); got.Estado != dominio.EstadoSuspendidaTemporal {
		t.Fatalf("el efecto debe estar confirmado: %+v", got)
	}
}

func TestSiElEfectoFallaNoQuedaNiElEfectoNiElId(t *testing.T) {
	r := repoReal(t)
	u := activa(t, r, "E001")
	fallo := errors.New("fallo después del efecto")

	_, err := r.ProcesarUnaVez(bg, "evt-1", func(repo aplicacion.Repositorio) error {
		if _, err := repo.Suspender(bg, u.ID, "V-A"); err != nil {
			return err
		}
		return fallo
	})
	if !errors.Is(err, fallo) {
		t.Fatalf("err = %v", err)
	}
	if got := leer(t, r, "E001"); got.Estado != dominio.EstadoActiva || got.VacacionesSuspension != "" {
		t.Fatalf("rollback: el efecto no debe quedar: %+v", got)
	}
	var n int
	if err := r.pool.QueryRow(bg, `SELECT count(*) FROM eventos_procesados WHERE id = 'evt-1'`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("rollback: el id no debe quedar registrado (n=%d, err=%v)", n, err)
	}

	dup, err := r.ProcesarUnaVez(bg, "evt-1", func(repo aplicacion.Repositorio) error {
		_, err := repo.Suspender(bg, u.ID, "V-A")
		return err
	})
	if dup || err != nil || leer(t, r, "E001").Estado != dominio.EstadoSuspendidaTemporal {
		t.Fatalf("la reentrega debe aplicarse: dup=%v err=%v", dup, err)
	}
}

// Un INSERT que choca dentro de la transacción no la aborta (SAVEPOINT): se puede
// seguir consultando y confirmar.
func TestCrearDuplicadoDentroDeLaTransaccionNoLaAborta(t *testing.T) {
	r := repoReal(t)
	activa(t, r, "E001")

	dup, err := r.ProcesarUnaVez(bg, "evt-1", func(repo aplicacion.Repositorio) error {
		otro := pendiente("E009", "E001@empresa.com", "", time.Time{})
		if err := repo.Crear(bg, otro); !errors.Is(err, aplicacion.ErrUsuarioDuplicado) {
			t.Errorf("crear con email repetido: err = %v", err)
		}
		propietario, err := repo.BuscarPorEmail(bg, "e001@empresa.com")
		if err != nil || propietario == nil || propietario.Sujeto != "E001" {
			t.Errorf("consultar tras el choque: %+v, %v", propietario, err)
		}
		return nil
	})
	if dup || err != nil {
		t.Fatalf("dup=%v err=%v", dup, err)
	}
}
