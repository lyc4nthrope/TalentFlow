package aplicacion

import (
	"errors"
	"strings"
	"testing"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
)

// --- Ayudas ---

func (e *entorno) retirar(t *testing.T, id string) {
	t.Helper()
	if err := e.svc.RetirarCuenta(ctx, id, dominio.EmpleadoRetirado{EmpleadoID: "E001"}); err != nil {
		t.Fatal(err)
	}
}

func (e *entorno) iniciar(t *testing.T, id, vacaciones string) {
	t.Helper()
	if err := e.svc.SuspenderPorVacaciones(ctx, id, dominio.VacacionesIniciadas{VacacionesID: vacaciones, EmpleadoID: "E001"}); err != nil {
		t.Fatal(err)
	}
}

func (e *entorno) finalizar(t *testing.T, id, vacaciones string) {
	t.Helper()
	if err := e.svc.ReactivarPorFinDeVacaciones(ctx, id, dominio.VacacionesFinalizadas{VacacionesID: vacaciones, EmpleadoID: "E001"}); err != nil {
		t.Fatal(err)
	}
}

// publicadosDesde devuelve los tipos publicados a partir de la posición n.
func (e *entorno) publicadosDesde(n int) []string { return e.pub.tipos()[n:] }

func (e *entorno) exigirEstado(t *testing.T, estado, suspension string) {
	t.Helper()
	u := e.repo.cuenta("E001")
	if u == nil || u.Estado != estado || u.VacacionesSuspension != suspension {
		t.Fatalf("cuenta = %+v; se esperaba estado %s y suspensión %q", u, estado, suspension)
	}
}

func (e *entorno) puedeIniciarSesion() bool {
	_, err := e.svc.Login(ctx, "juan@empresa.com", passwordValida)
	return err == nil
}

func exigirTipos(t *testing.T, obtenidos []string, esperados ...string) {
	t.Helper()
	if strings.Join(obtenidos, ",") != strings.Join(esperados, ",") {
		t.Fatalf("publicados = %v; se esperaba %v", obtenidos, esperados)
	}
}

// --- empleado.retirado ---

func TestRetiroDesactivaPermanenteYPublicaCuentaDesactivadaRetiro(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	n := len(e.pub.tipos())

	e.retirar(t, "r1")

	e.exigirEstado(t, dominio.EstadoDesactivadaPermanente, "")
	exigirTipos(t, e.publicadosDesde(n), dominio.EventoCuentaDesactivada)
	d := datos(t, e.pub.ultimo().Data)
	if claves(d) != "email,empleadoId,motivo,permanente" ||
		d["empleadoId"] != "E001" || d["email"] != "juan@empresa.com" || d["motivo"] != "RETIRO" || d["permanente"] != true {
		t.Fatalf("payload de cuenta.desactivada = %v", d)
	}
	if e.puedeIniciarSesion() {
		t.Fatal("un empleado retirado no debe poder iniciar sesión")
	}
}

func TestRetiroDeUnaCuentaPendienteAnulaElTokenDeActivacion(t *testing.T) {
	e := nuevoEntorno(t)
	if err := e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "juan@empresa.com"}); err != nil {
		t.Fatal(err)
	}

	e.retirar(t, "r1")

	if u := e.repo.cuenta("E001"); u.TokenHash != "" || u.TokenTipo != "" || !u.TokenExpiraEn.IsZero() {
		t.Fatalf("el retiro debe borrar el token pendiente: %+v", u)
	}
	if err := e.svc.RestablecerPassword(ctx, "tok-1", passwordValida); !errors.Is(err, ErrTokenInvalido) {
		t.Fatalf("activar con el token de un retirado: err = %v", err)
	}
	e.exigirEstado(t, dominio.EstadoDesactivadaPermanente, "")
}

func TestRetiroDeUnaCuentaYaRetiradaNoVuelveAPublicar(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	e.retirar(t, "r1")
	n := len(e.pub.tipos())

	e.retirar(t, "r2") // otro mensaje (id distinto) con el mismo retiro

	exigirTipos(t, e.publicadosDesde(n))
	e.exigirEstado(t, dominio.EstadoDesactivadaPermanente, "")
}

func TestRetiroDuranteVacacionesBorraLaSuspension(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	e.iniciar(t, "i1", "V-1")

	e.retirar(t, "r1")

	e.exigirEstado(t, dominio.EstadoDesactivadaPermanente, "")
}

func TestUnaCuentaRetiradaNoRecibeRecuperacion(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	e.retirar(t, "r1")
	n := len(e.pub.tipos())

	if err := e.svc.SolicitarRecuperacion(ctx, "juan@empresa.com"); err != nil {
		t.Fatal(err)
	}
	exigirTipos(t, e.publicadosDesde(n))
}

// --- vacaciones.iniciadas ---

func TestInicioDeVacacionesSuspendeUnaCuentaActivaYPublicaCuentaDesactivada(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	n := len(e.pub.tipos())

	e.iniciar(t, "i1", "V-1")

	e.exigirEstado(t, dominio.EstadoSuspendidaTemporal, "V-1")
	exigirTipos(t, e.publicadosDesde(n), dominio.EventoCuentaDesactivada)
	d := datos(t, e.pub.ultimo().Data)
	if claves(d) != "email,empleadoId,motivo,permanente" ||
		d["empleadoId"] != "E001" || d["email"] != "juan@empresa.com" || d["motivo"] != "VACACIONES" || d["permanente"] != false {
		t.Fatalf("payload de cuenta.desactivada = %v", d)
	}
	if e.puedeIniciarSesion() {
		t.Fatal("una cuenta de vacaciones no debe poder iniciar sesión")
	}
}

func TestInicioDeVacacionesNoTocaUnaCuentaPendienteNiUnaRetirada(t *testing.T) {
	e := nuevoEntorno(t)
	if err := e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "juan@empresa.com"}); err != nil {
		t.Fatal(err)
	}
	n := len(e.pub.tipos())

	e.iniciar(t, "i1", "V-1")
	e.exigirEstado(t, dominio.EstadoPendienteActivacion, "")

	e.retirar(t, "r1")
	m := len(e.pub.tipos())
	e.iniciar(t, "i2", "V-2")
	e.exigirEstado(t, dominio.EstadoDesactivadaPermanente, "")

	exigirTipos(t, e.pub.tipos()[n:m], dominio.EventoCuentaDesactivada) // solo el retiro
	exigirTipos(t, e.publicadosDesde(m))
}

func TestInicioDeOtrasVacacionesEstandoSuspendidaSoloCambiaElIdSinPublicar(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	e.iniciar(t, "i1", "V-1")
	n := len(e.pub.tipos())

	e.iniciar(t, "i2", "V-2")

	e.exigirEstado(t, dominio.EstadoSuspendidaTemporal, "V-2")
	exigirTipos(t, e.publicadosDesde(n))
}

// --- vacaciones.finalizadas ---

func TestFinDeVacacionesReactivaYPublicaCuentaActivadaFinVacaciones(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	e.iniciar(t, "i1", "V-1")
	n := len(e.pub.tipos())

	e.finalizar(t, "f1", "V-1")

	e.exigirEstado(t, dominio.EstadoActiva, "")
	exigirTipos(t, e.publicadosDesde(n), dominio.EventoCuentaActivada)
	d := datos(t, e.pub.ultimo().Data)
	if claves(d) != "email,empleadoId,motivo" || d["empleadoId"] != "E001" || d["email"] != "juan@empresa.com" || d["motivo"] != "FIN_VACACIONES" {
		t.Fatalf("payload de cuenta.activada = %v", d)
	}
	if !e.puedeIniciarSesion() {
		t.Fatal("al volver de vacaciones la cuenta debe poder iniciar sesión")
	}
}

func TestFinDeOtrasVacacionesNoReactiva(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	e.iniciar(t, "i1", "V-1")
	n := len(e.pub.tipos())

	e.finalizar(t, "f1", "V-OTRA")

	e.exigirEstado(t, dominio.EstadoSuspendidaTemporal, "V-1")
	exigirTipos(t, e.publicadosDesde(n))
}

func TestFinDeVacacionesSobreUnaCuentaActivaNoHaceNada(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	n := len(e.pub.tipos())

	e.finalizar(t, "f1", "V-1")

	e.exigirEstado(t, dominio.EstadoActiva, "")
	exigirTipos(t, e.publicadosDesde(n))
}

// Caso borde obligatorio del PDF (D5): retirado mientras está de vacaciones.
func TestCasoBordeRetiradoDuranteVacacionesNoSeReactivaAlFinalizar(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	n := len(e.pub.tipos())

	e.iniciar(t, "i1", "V-1")
	e.retirar(t, "r1")
	e.finalizar(t, "f1", "V-1")

	e.exigirEstado(t, dominio.EstadoDesactivadaPermanente, "")
	exigirTipos(t, e.publicadosDesde(n), dominio.EventoCuentaDesactivada, dominio.EventoCuentaDesactivada)
	if e.puedeIniciarSesion() {
		t.Fatal("el retirado no debe poder iniciar sesión al terminar sus vacaciones")
	}
}

// D18: con períodos contiguos el scheduler publica el inicio de B antes que el fin de A.
func TestPeriodosContiguosElFinDelPrimeroNoReactivaDuranteElSegundo(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	n := len(e.pub.tipos())

	e.iniciar(t, "iA", "V-A")
	e.iniciar(t, "iB", "V-B")
	e.finalizar(t, "fA", "V-A")
	e.exigirEstado(t, dominio.EstadoSuspendidaTemporal, "V-B")

	e.finalizar(t, "fB", "V-B")
	e.exigirEstado(t, dominio.EstadoActiva, "")
	exigirTipos(t, e.publicadosDesde(n), dominio.EventoCuentaDesactivada, dominio.EventoCuentaActivada)
}

// --- Deduplicación (D4) ---

func TestElMismoEventoDosVecesSeAplicaYPublicaUnaSolaVez(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	n := len(e.pub.tipos())

	e.iniciar(t, "mismo-id", "V-1")
	e.finalizar(t, "f1", "V-1")
	e.iniciar(t, "mismo-id", "V-1") // reentrega del primer mensaje

	e.exigirEstado(t, dominio.EstadoActiva, "")
	exigirTipos(t, e.publicadosDesde(n), dominio.EventoCuentaDesactivada, dominio.EventoCuentaActivada)
}

func TestEmpleadoCreadoRepetidoCreaYPublicaUnaSolaVez(t *testing.T) {
	e := nuevoEntorno(t)
	evento := dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "juan@empresa.com"}
	for range 2 {
		if err := e.svc.CrearCuentaDesdeEmpleado(ctx, "mismo-id", evento); err != nil {
			t.Fatal(err)
		}
	}
	exigirTipos(t, e.pub.tipos(), dominio.EventoUsuarioCreado)
	if !e.repo.procesados["mismo-id"] {
		t.Fatal("el id del evento debe quedar registrado")
	}
}

func TestSiElEfectoFallaElIdNoQuedaRegistradoYLaReentregaSeAplica(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	n := len(e.pub.tipos())
	e.repo.fallarEfecto = errors.New("bd caída")

	err := e.svc.SuspenderPorVacaciones(ctx, "i1", dominio.VacacionesIniciadas{VacacionesID: "V-1", EmpleadoID: "E001"})
	if err == nil || errors.Is(err, ErrSolicitudInvalida) || errors.Is(err, ErrRechazoPermanente) {
		t.Fatalf("un fallo de la BD debe ser transitorio: err = %v", err)
	}
	if e.repo.procesados["i1"] {
		t.Fatal("si el efecto falla, el id no debe quedar registrado")
	}
	e.exigirEstado(t, dominio.EstadoActiva, "")
	exigirTipos(t, e.publicadosDesde(n))

	e.iniciar(t, "i1", "V-1") // la reentrega sí se aplica
	e.exigirEstado(t, dominio.EstadoSuspendidaTemporal, "V-1")
}

func TestEventoDeUnEmpleadoSinCuentaSeRegistraSinPublicar(t *testing.T) {
	e := nuevoEntorno(t)
	if err := e.svc.RetirarCuenta(ctx, "r1", dominio.EmpleadoRetirado{EmpleadoID: "E404"}); err != nil {
		t.Fatal(err)
	}
	if len(e.pub.tipos()) != 0 || !e.repo.procesados["r1"] {
		t.Fatalf("publicados=%v procesado=%v", e.pub.tipos(), e.repo.procesados["r1"])
	}
}

func TestUnEventoNoPuedeTocarAlAdministrador(t *testing.T) {
	e := nuevoEntorno(t)
	if err := e.svc.SembrarAdmin(ctx, "admin@empresa.com", passwordValida); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RetirarCuenta(ctx, "r1", dominio.EmpleadoRetirado{EmpleadoID: dominio.SujetoAdmin}); err != nil {
		t.Fatal(err)
	}
	if u := e.repo.cuenta(dominio.SujetoAdmin); u.Estado != dominio.EstadoActiva {
		t.Fatalf("el administrador no es un empleado: estado = %s", u.Estado)
	}
}

func TestFalloAlPublicarTrasElCommitNoRevierteElEfecto(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	e.pub.fallar = errors.New("broker caído")

	e.retirar(t, "r1")

	e.exigirEstado(t, dominio.EstadoDesactivadaPermanente, "")
	if !e.repo.procesados["r1"] {
		t.Fatal("el efecto ya está confirmado: el id debe quedar registrado")
	}
}

func TestDatosInvalidosDelCicloSonSolicitudInvalida(t *testing.T) {
	e := nuevoEntorno(t)
	casos := map[string]error{
		"sin id de evento":   e.svc.RetirarCuenta(ctx, " ", dominio.EmpleadoRetirado{EmpleadoID: "E001"}),
		"id de evento largo": e.svc.RetirarCuenta(ctx, strings.Repeat("x", 101), dominio.EmpleadoRetirado{EmpleadoID: "E001"}),
		"sin empleadoId":     e.svc.RetirarCuenta(ctx, "r1", dominio.EmpleadoRetirado{}),
		"empleadoId largo":   e.svc.SuspenderPorVacaciones(ctx, "i1", dominio.VacacionesIniciadas{VacacionesID: "V-1", EmpleadoID: strings.Repeat("E", 37)}),
		"sin vacacionesId":   e.svc.SuspenderPorVacaciones(ctx, "i1", dominio.VacacionesIniciadas{EmpleadoID: "E001"}),
		"vacacionesId largo": e.svc.ReactivarPorFinDeVacaciones(ctx, "f1", dominio.VacacionesFinalizadas{VacacionesID: strings.Repeat("V", 21), EmpleadoID: "E001"}),
		"fin sin empleadoId": e.svc.ReactivarPorFinDeVacaciones(ctx, "f1", dominio.VacacionesFinalizadas{VacacionesID: "V-1"}),
		"creado sin id":      e.svc.CrearCuentaDesdeEmpleado(ctx, "", dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"}),
	}
	for nombre, err := range casos {
		if !errors.Is(err, ErrSolicitudInvalida) {
			t.Errorf("%s: err = %v", nombre, err)
		}
	}
	if len(e.repo.procesados) != 0 {
		t.Fatalf("ningún evento inválido debe registrarse: %v", e.repo.procesados)
	}
}

// Resultado observable con entregas simultáneas: un solo efecto y una sola
// publicación. La atomicidad de la deduplicación en sí (comprobar y marcar el id en
// una operación) se prueba contra PostgreSQL en
// postgres.TestElMismoIdProcesadoEnParaleloSeAplicaUnaSolaVez; aquí el UPDATE
// condicional también la cubriría.
func TestDosEntregasSimultaneasDelMismoMensajeSeAplicanUnaSolaVez(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	n := len(e.pub.tipos())

	errs := make(chan error, 8)
	for range 8 {
		go func() {
			errs <- e.svc.SuspenderPorVacaciones(ctx, "mismo-id", dominio.VacacionesIniciadas{VacacionesID: "V-1", EmpleadoID: "E001"})
		}()
	}
	for range 8 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	exigirTipos(t, e.publicadosDesde(n), dominio.EventoCuentaDesactivada)
	e.exigirEstado(t, dominio.EstadoSuspendidaTemporal, "V-1")
}

func TestEmpleadoCreadoConElIdReservadoDelAdministradorSeRechaza(t *testing.T) {
	e := nuevoEntorno(t)
	if err := e.svc.SembrarAdmin(ctx, "admin@empresa.com", passwordValida); err != nil {
		t.Fatal(err)
	}
	err := e.svc.CrearCuentaDesdeEmpleado(ctx, "c1", dominio.EmpleadoCreado{EmpleadoID: dominio.SujetoAdmin, Email: "otro@empresa.com"})
	if !errors.Is(err, ErrRechazoPermanente) {
		t.Fatalf("err = %v", err)
	}
	if len(e.pub.tipos()) != 0 || e.repo.cuenta(dominio.SujetoAdmin).Email != "admin@empresa.com" {
		t.Fatalf("no debe crear ni publicar nada: %v", e.pub.tipos())
	}
}

// Si el alta choca con una cuenta que no es la del empleado ni la de su email (p. ej.
// el sujeto de otra cuenta), es un rechazo permanente explícito, no un ack silencioso.
func TestEmpleadoCreadoQueChocaConOtraCuentaSinSerLaSuyaSeRechaza(t *testing.T) {
	e := nuevoEntorno(t)
	if err := e.repo.Crear(ctx, &dominio.Usuario{Sujeto: "E777", Email: "ajena@empresa.com", Rol: dominio.RolUser,
		Estado: dominio.EstadoPendienteActivacion}); err != nil {
		t.Fatal(err)
	}
	err := e.svc.CrearCuentaDesdeEmpleado(ctx, "c1", dominio.EmpleadoCreado{EmpleadoID: "E777", Email: "nuevo@empresa.com"})
	if !errors.Is(err, ErrRechazoPermanente) {
		t.Fatalf("err = %v", err)
	}
	if len(e.pub.tipos()) != 0 || e.repo.procesados["c1"] {
		t.Fatalf("publicados=%v procesado=%v", e.pub.tipos(), e.repo.procesados["c1"])
	}
}

// Aunque el administrador todavía no exista, un empleado no puede quedarse con su
// "sub": después la semilla ya no podría crearse y el JWT del empleado diría "admin".
func TestElIdDelAdministradorEstaReservadoAunqueNoExistaLaSemilla(t *testing.T) {
	e := nuevoEntorno(t)
	err := e.svc.CrearCuentaDesdeEmpleado(ctx, "c1", dominio.EmpleadoCreado{EmpleadoID: dominio.SujetoAdmin, Email: "x@empresa.com"})
	if !errors.Is(err, ErrRechazoPermanente) || e.repo.cuenta(dominio.SujetoAdmin) != nil {
		t.Fatalf("err = %v, cuenta = %+v", err, e.repo.cuenta(dominio.SujetoAdmin))
	}
}
