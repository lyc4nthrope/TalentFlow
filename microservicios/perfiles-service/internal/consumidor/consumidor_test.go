package consumidor_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"

	"talentflow/perfiles-service/internal/consumidor"
	"talentflow/perfiles-service/internal/eventos"
	"talentflow/perfiles-service/internal/store"
	"talentflow/perfiles-service/internal/testutil"
)

func nuevo(t *testing.T) (*consumidor.Consumidor, *store.Store, *sql.DB) {
	db := testutil.Conectar(t)
	s := store.New(db)
	return &consumidor.Consumidor{Store: s, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}, s, db
}

func evento(id, tipo, data string) []byte {
	return []byte(fmt.Sprintf(`{"id":%q,"type":%q,"version":1,"occurredAt":"2026-10-10T12:00:00Z","producer":"empleados-service","data":%s}`, id, tipo, data))
}

const dataCreado = `{"empleadoId":"E001","nombre":"Juan","apellido":"Pérez","email":"juan.perez@empresa.com",
	"numeroEmpleado":"EMP-2026-001","cargo":"Desarrollador Senior","area":"Tecnología","departamentoId":"IT",
	"fechaIngreso":"2026-02-10","estado":"ACTIVO"}`

func TestCreadoGeneraPerfilPorDefecto(t *testing.T) {
	c, s, _ := nuevo(t)
	ctx := context.Background()

	res, err := c.Manejar(ctx, evento("m-1", "empleado.creado", dataCreado))
	if err != nil || res != "procesado" {
		t.Fatalf("resultado=%q err=%v", res, err)
	}
	p, err := s.Obtener(ctx, "E001")
	if err != nil {
		t.Fatal(err)
	}
	if p.Nombre != "Juan" || p.Email != "juan.perez@empresa.com" || p.Cargo != "Desarrollador Senior" {
		t.Errorf("campos replicados incorrectos: %+v", p)
	}
	if p.Telefono != "" || p.Direccion != "" || p.Ciudad != "" || p.Biografia != "" {
		t.Errorf("el perfil por defecto debe tener teléfono/dirección/ciudad/biografía vacíos: %+v", p)
	}
	if p.Archivado || p.FechaArchivado != nil || p.ID == "" {
		t.Errorf("estado inicial incorrecto: %+v", p)
	}
}

func TestDeduplicacionMismoIdUnSoloEfecto(t *testing.T) {
	c, s, db := nuevo(t)
	ctx := context.Background()

	if res, _ := c.Manejar(ctx, evento("mismo-id", "empleado.creado", dataCreado)); res != "procesado" {
		t.Fatalf("primera entrega: %q", res)
	}
	// Reentrega del MISMO mensaje, incluso con otro contenido: debe descartarse.
	otro := `{"empleadoId":"E001","nombre":"OTRO","email":"otro@empresa.com"}`
	res, err := c.Manejar(ctx, evento("mismo-id", "empleado.creado", otro))
	if err != nil || res != "duplicado" {
		t.Fatalf("segunda entrega: res=%q err=%v", res, err)
	}

	var perfiles, registrados int
	_ = db.QueryRow(`SELECT count(*) FROM perfiles`).Scan(&perfiles)
	_ = db.QueryRow(`SELECT count(*) FROM eventos_procesados WHERE id='mismo-id'`).Scan(&registrados)
	if perfiles != 1 || registrados != 1 {
		t.Errorf("perfiles=%d registrados=%d, se esperaba 1 y 1", perfiles, registrados)
	}
	p, _ := s.Obtener(ctx, "E001")
	if p.Nombre != "Juan" {
		t.Errorf("el duplicado repitió el efecto: nombre=%q", p.Nombre)
	}
}

func TestCreadoConIdsDistintosParaElMismoEmpleadoNoDuplicaPerfil(t *testing.T) {
	c, _, db := nuevo(t)
	ctx := context.Background()
	c.Manejar(ctx, evento("a", "empleado.creado", dataCreado))
	c.Manejar(ctx, evento("b", "empleado.creado", dataCreado))
	var n int
	_ = db.QueryRow(`SELECT count(*) FROM perfiles`).Scan(&n)
	if n != 1 {
		t.Errorf("perfiles=%d, se esperaba 1", n)
	}
}

func TestActualizadoSincronizaSinPisarCamposDelPerfil(t *testing.T) {
	c, s, _ := nuevo(t)
	ctx := context.Background()
	c.Manejar(ctx, evento("1", "empleado.creado", dataCreado))
	if _, err := s.Actualizar(ctx, "E001", map[string]string{"telefono": "3001234567", "ciudad": "Armenia"}); err != nil {
		t.Fatal(err)
	}

	actualizado := `{"empleadoId":"E001","nombre":"Juan","apellido":"Pérez Gómez","email":"juan.perez@empresa.com",
		"cargo":"Tech Lead","area":"Tecnología","departamentoId":"IT"}`
	res, err := c.Manejar(ctx, evento("2", "empleado.actualizado", actualizado))
	if err != nil || res != "procesado" {
		t.Fatalf("res=%q err=%v", res, err)
	}

	p, _ := s.Obtener(ctx, "E001")
	if p.Apellido != "Pérez Gómez" || p.Cargo != "Tech Lead" {
		t.Errorf("no sincronizó: %+v", p)
	}
	if p.Telefono != "3001234567" || p.Ciudad != "Armenia" {
		t.Errorf("la sincronización pisó datos propios del perfil: %+v", p)
	}
}

func TestActualizadoSinPerfilLoCrea(t *testing.T) {
	c, s, _ := nuevo(t)
	ctx := context.Background()
	d := `{"empleadoId":"E050","nombre":"Ana","apellido":"Ruiz","email":"ana@empresa.com","cargo":"Dev","area":"TI","departamentoId":"IT"}`
	if _, err := c.Manejar(ctx, evento("1", "empleado.actualizado", d)); err != nil {
		t.Fatal(err)
	}
	if p, err := s.Obtener(ctx, "E050"); err != nil || p.Nombre != "Ana" {
		t.Errorf("p=%+v err=%v", p, err)
	}
}

func TestRetiradoArchivaYNoBorra(t *testing.T) {
	c, s, db := nuevo(t)
	ctx := context.Background()
	c.Manejar(ctx, evento("1", "empleado.creado", dataCreado))

	retirado := `{"empleadoId":"E001","email":"juan.perez@empresa.com","fechaRetiro":"2026-11-30T16:45:00Z","motivo":"RENUNCIA"}`
	res, err := c.Manejar(ctx, evento("2", "empleado.retirado", retirado))
	if err != nil || res != "procesado" {
		t.Fatalf("res=%q err=%v", res, err)
	}
	p, err := s.Obtener(ctx, "E001")
	if err != nil {
		t.Fatalf("el perfil desapareció: %v", err)
	}
	if !p.Archivado || p.FechaArchivado == nil || p.Nombre != "Juan" {
		t.Errorf("no quedó archivado con sus datos intactos: %+v", p)
	}
	var n int
	_ = db.QueryRow(`SELECT count(*) FROM perfiles`).Scan(&n)
	if n != 1 {
		t.Errorf("perfiles=%d", n)
	}
}

func TestRetiradoSinPerfilNoFalla(t *testing.T) {
	c, _, _ := nuevo(t)
	res, err := c.Manejar(context.Background(), evento("1", "empleado.retirado", `{"empleadoId":"FANTASMA","email":"x@x.com"}`))
	if err != nil || res != "procesado" {
		t.Errorf("res=%q err=%v", res, err)
	}
}

func TestEventosInvalidosSeMarcanParaDescartar(t *testing.T) {
	c, _, _ := nuevo(t)
	ctx := context.Background()
	casos := map[string][]byte{
		"no es json":        []byte("hola"),
		"sin envelope":      []byte(`{"type":"empleado.creado"}`),
		"sin empleadoId":    evento("1", "empleado.creado", `{"email":"a@a.com"}`),
		"sin email":         evento("2", "empleado.creado", `{"empleadoId":"E1"}`),
		"retirado sin id":   evento("3", "empleado.retirado", `{"email":"a@a.com"}`),
		"data no es objeto": evento("4", "empleado.actualizado", `"texto"`),
	}
	for nombre, cuerpo := range casos {
		if _, err := c.Manejar(ctx, cuerpo); !errors.Is(err, eventos.ErrInvalido) {
			t.Errorf("%s: err=%v, se esperaba ErrInvalido", nombre, err)
		}
	}
}

func TestEventoAjenoSeIgnora(t *testing.T) {
	c, _, _ := nuevo(t)
	res, err := c.Manejar(context.Background(), evento("1", "vacaciones.programadas", `{"x":1}`))
	if err != nil || res != "ignorado" {
		t.Errorf("res=%q err=%v", res, err)
	}
}

func TestSiElEfectoFallaElIdNoQuedaRegistrado(t *testing.T) {
	_, s, db := nuevo(t)
	ctx := context.Background()
	boom := errors.New("BD caída a mitad de camino")

	_, err := s.ProcesarEvento(ctx, "reintentable", func(tx *sql.Tx) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
	var n int
	_ = db.QueryRow(`SELECT count(*) FROM eventos_procesados WHERE id='reintentable'`).Scan(&n)
	if n != 0 {
		t.Fatalf("el id quedó registrado pese al fallo")
	}
	// La reentrega sí se procesa
	dup, err := s.ProcesarEvento(ctx, "reintentable", func(tx *sql.Tx) error { return nil })
	if err != nil || dup {
		t.Errorf("dup=%v err=%v", dup, err)
	}
}
