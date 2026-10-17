package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/aplicacion"
	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/dominio"
)

// Pruebas de integración contra PostgreSQL real con el esquema de init.sql. Solo
// corren si NOTIF_TEST_DATABASE_URL apunta a una BD desechable (se vacían las tablas).
func repositorioDePrueba(t *testing.T) (*Repositorio, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("NOTIF_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("NOTIF_TEST_DATABASE_URL no definida: se omiten las pruebas contra PostgreSQL")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("conectar: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(context.Background(), `TRUNCATE notificaciones, eventos_procesados`); err != nil {
		t.Fatalf("vaciar tablas: %v", err)
	}
	return NuevoRepositorio(pool), pool
}

func notificacion(tipo dominio.Tipo, empleadoID string) dominio.Notificacion {
	return dominio.Notificacion{
		Tipo: tipo, Destinatario: "juan@empresa.com", Mensaje: "m",
		FechaEnvio: time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC), EmpleadoID: empleadoID,
	}
}

func TestSinEmpleadoIDSeGuardaNULLYNoApareceEnElHistorialDeNadie(t *testing.T) {
	repo, pool := repositorioDePrueba(t)
	ctx := context.Background()

	if _, nueva, err := repo.RegistrarSiEsNuevo(ctx, "ev-rec", notificacion(dominio.TipoSeguridad, "")); err != nil || !nueva {
		t.Fatalf("registrar sin empleado: nueva=%v err=%v", nueva, err)
	}
	if _, _, err := repo.RegistrarSiEsNuevo(ctx, "ev-e1", notificacion(dominio.TipoCuenta, "E001")); err != nil {
		t.Fatalf("registrar con empleado: %v", err)
	}

	var nulos int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notificaciones WHERE empleado_id IS NULL`).Scan(&nulos); err != nil || nulos != 1 {
		t.Fatalf("se esperaba 1 fila con empleado_id NULL (no ''), hay %d (err %v)", nulos, err)
	}
	todas, err := repo.Listar(ctx)
	if err != nil || len(todas) != 2 {
		t.Fatalf("listar: %d %v", len(todas), err)
	}
	porTipo := map[dominio.Tipo]dominio.Notificacion{}
	for _, n := range todas {
		porTipo[n.Tipo] = n
	}
	if rec := porTipo[dominio.TipoSeguridad]; rec.EmpleadoID != "" {
		t.Fatalf("la recuperación (sin empleado) debe leerse con empleadoId vacío: %+v", rec)
	}
	if cta := porTipo[dominio.TipoCuenta]; cta.EmpleadoID != "E001" {
		t.Fatalf("la notificación de E001 debe conservar su empleadoId: %+v", cta)
	}
	deE001, err := repo.ListarPorEmpleado(ctx, "E001")
	if err != nil || len(deE001) != 1 || deE001[0].EmpleadoID != "E001" {
		t.Fatalf("historial de E001 inesperado: %+v %v", deE001, err)
	}
	vacio, _ := repo.ListarPorEmpleado(ctx, "")
	if len(vacio) != 0 {
		t.Fatalf("el historial de un empleado vacío no debe traer la notificación sin empleado: %+v", vacio)
	}
}

// El token vigente nunca llega a la BD: se guarda el mensaje enmascarado.
func TestElTokenNoSeGuardaEnLaBD(t *testing.T) {
	repo, pool := repositorioDePrueba(t)
	ctx := context.Background()
	const token = "9c1d4e7a-2b8f-4a30-b6e5-1f207d9a4c88"
	n, err := dominio.NotificacionDesdeEvento("usuario.recuperacion",
		[]byte(`{"email":"juan@empresa.com","tokenRecuperacion":"`+token+`","expiraEn":"2026-05-14T11:00:00Z"}`), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	guardada, _, err := repo.RegistrarSiEsNuevo(ctx, "ev-token", n)
	if err != nil {
		t.Fatal(err)
	}
	var enLaBD int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notificaciones WHERE mensaje LIKE '%' || $1 || '%'`, token).Scan(&enLaBD); err != nil || enLaBD != 0 {
		t.Fatalf("el token completo quedó en la BD (%d filas, err %v)", enLaBD, err)
	}
	leidas, _ := repo.Listar(ctx)
	if len(leidas) != 1 || leidas[0].Mensaje != n.Mensaje {
		t.Fatalf("se esperaba leer el mensaje enmascarado: %+v", leidas)
	}
	// Lo que vuelve del registro (y va a los canales) conserva el texto de entrega.
	if guardada.TextoAEntregar() != n.TextoAEntregar() {
		t.Fatalf("el registro perdió el texto de entrega: %q", guardada.TextoAEntregar())
	}
}

func TestLaBDSoloAceptaLosTiposDelReto5(t *testing.T) {
	repo, _ := repositorioDePrueba(t)
	ctx := context.Background()

	for i, tipo := range []dominio.Tipo{dominio.TipoSeguridad, dominio.TipoCuenta, dominio.TipoVacaciones} {
		if _, _, err := repo.RegistrarSiEsNuevo(ctx, "ok-"+string(rune('a'+i)), notificacion(tipo, "E001")); err != nil {
			t.Fatalf("%s debe aceptarse: %v", tipo, err)
		}
	}
	for _, viejo := range []dominio.Tipo{"BIENVENIDA", "DESVINCULACION", "RECUPERACION_CLAVE"} {
		_, _, err := repo.RegistrarSiEsNuevo(ctx, "viejo-"+string(viejo), notificacion(viejo, "E001"))
		if !errors.Is(err, aplicacion.ErrRechazoPermanente) {
			t.Fatalf("%s debe rechazarse de forma permanente, se obtuvo %v", viejo, err)
		}
	}
}

func TestElMismoEventoNoSeRegistraDosVeces(t *testing.T) {
	repo, _ := repositorioDePrueba(t)
	ctx := context.Background()

	_, primera, err1 := repo.RegistrarSiEsNuevo(ctx, "dup", notificacion(dominio.TipoCuenta, "E001"))
	_, segunda, err2 := repo.RegistrarSiEsNuevo(ctx, "dup", notificacion(dominio.TipoCuenta, "E001"))
	if err1 != nil || err2 != nil || !primera || segunda {
		t.Fatalf("primera=%v segunda=%v err1=%v err2=%v", primera, segunda, err1, err2)
	}
}
