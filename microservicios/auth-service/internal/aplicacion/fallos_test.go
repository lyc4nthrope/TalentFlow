package aplicacion

import (
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
)

// Fallos de infraestructura (BD, firma del token, generación de tokens) dentro de
// los casos de uso. La regla es la misma en todos: el error llega como un error
// técnico (el handler responde 500 y el consumidor reintenta), nunca como un error de
// negocio que el cliente vería como "credenciales inválidas" o "token inválido", y
// no queda ningún efecto a medias (ni cuenta, ni evento, ni token consumido).

var errBD = errors.New("bd caída")

// exigirTransitorio falla si err es nil o es uno de los errores de negocio.
func exigirTransitorio(t *testing.T, err error) {
	t.Helper()
	negocio := []error{ErrSolicitudInvalida, ErrCredencialesInvalidas, ErrTokenInvalido, ErrNoAutenticado,
		ErrPasswordActual, ErrPasswordRepetida, ErrRechazoPermanente, dominio.ErrPoliticaPassword}
	if err == nil {
		t.Fatal("se esperaba un error técnico, no hubo error")
	}
	for _, n := range negocio {
		if errors.Is(err, n) {
			t.Fatalf("se esperaba un error técnico, se obtuvo el de negocio %v", err)
		}
	}
}

// --- Construcción del servicio ---

func TestSinCostoBcryptSeUsaElCostoPorDefecto(t *testing.T) {
	repo := nuevoRepo()
	svc, err := NuevoAuthService(repo, &publicadorEspia{}, emisorFalso{}, generadorSecuencial(),
		func() time.Time { return inicio }, Opciones{VigenciaActivacion: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.RestablecerPassword(ctx, "tok-1", passwordValida); err != nil {
		t.Fatal(err)
	}
	u := repo.cuenta("E001")
	if costo, err := bcrypt.Cost([]byte(u.PasswordHash)); err != nil || costo != bcrypt.DefaultCost {
		t.Fatalf("costo = %d (%v); se esperaba %d", costo, err, bcrypt.DefaultCost)
	}
}

func TestUnCostoBcryptFueraDeRangoImpideCrearElServicio(t *testing.T) {
	svc, err := NuevoAuthService(nuevoRepo(), &publicadorEspia{}, emisorFalso{}, generadorSecuencial(),
		func() time.Time { return inicio }, Opciones{CostoBcrypt: bcrypt.MaxCost + 1})
	if err == nil || svc != nil {
		t.Fatalf("un costo inválido debe fallar al arrancar: svc=%v err=%v", svc, err)
	}
}

// --- Alta por empleado.creado ---

func TestSiNoSePuedeGenerarElTokenDeActivacionNoSeCreaLaCuenta(t *testing.T) {
	e := nuevoEntorno(t)
	e.svc.generar = generadorQueFalla
	err := e.svc.CrearCuentaDesdeEmpleado(ctx, "c1", dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
	exigirTransitorio(t, err)
	if e.repo.cuenta("E001") != nil || e.repo.procesados["c1"] || len(e.pub.tipos()) != 0 {
		t.Fatalf("no debe quedar cuenta, id registrado ni evento: procesado=%v eventos=%v",
			e.repo.procesados["c1"], e.pub.tipos())
	}
}

// Dos entregas casi simultáneas del alta (con ids distintos): la otra crea la cuenta
// entre la búsqueda y el INSERT. La cuenta ya es del empleado, así que no es un error
// y no se publica un segundo usuario.creado.
func TestSiOtraAltaDelMismoEmpleadoGanaLaCarreraNoEsError(t *testing.T) {
	e := nuevoEntorno(t)
	e.repo.competidor = &dominio.Usuario{Sujeto: "E001", EmpleadoID: "E001", Email: "a@b.com",
		Rol: dominio.RolUser, Estado: dominio.EstadoPendienteActivacion}
	if err := e.svc.CrearCuentaDesdeEmpleado(ctx, "c1", dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"}); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(e.repo.usuarios) != 1 || len(e.pub.tipos()) != 0 || !e.repo.procesados["c1"] {
		t.Fatalf("cuentas=%d eventos=%v procesado=%v", len(e.repo.usuarios), e.pub.tipos(), e.repo.procesados["c1"])
	}
}

func TestSiOtraCuentaTomaElEmailDuranteElAltaEsRechazoPermanente(t *testing.T) {
	e := nuevoEntorno(t)
	e.repo.competidor = &dominio.Usuario{Sujeto: "E002", EmpleadoID: "E002", Email: "a@b.com",
		Rol: dominio.RolUser, Estado: dominio.EstadoPendienteActivacion}
	err := e.svc.CrearCuentaDesdeEmpleado(ctx, "c1", dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
	if !errors.Is(err, ErrRechazoPermanente) {
		t.Fatalf("err = %v", err)
	}
	if e.repo.cuenta("E001") != nil || len(e.pub.tipos()) != 0 {
		t.Fatal("no debe crearse la cuenta ni publicarse nada")
	}
}

func TestFallosDeLaBDDuranteElAltaSonTransitorios(t *testing.T) {
	for _, op := range []string{"BuscarPorEmail", "Crear"} {
		t.Run(op, func(t *testing.T) {
			e := nuevoEntorno(t)
			e.repo.fallarEn = map[string]error{op: errBD}
			err := e.svc.CrearCuentaDesdeEmpleado(ctx, "c1", dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
			exigirTransitorio(t, err)
			if !errors.Is(err, errBD) || e.repo.procesados["c1"] || len(e.pub.tipos()) != 0 {
				t.Fatalf("err=%v procesado=%v eventos=%v", err, e.repo.procesados["c1"], e.pub.tipos())
			}
		})
	}
}

// --- Login ---

func TestLoginConLaBDCaidaNoEsCredencialesInvalidas(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	e.repo.fallar = errBD
	_, err := e.svc.Login(ctx, "juan@empresa.com", passwordValida)
	exigirTransitorio(t, err)
}

func TestLoginSinPoderFirmarElTokenNoEsCredencialesInvalidas(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	e.svc.emisor = emisorQueFalla{}
	token, err := e.svc.Login(ctx, "juan@empresa.com", passwordValida)
	exigirTransitorio(t, err)
	if token != "" {
		t.Fatalf("no debe devolverse token: %q", token)
	}
}

// --- Recuperación ---

func TestFallosAlSolicitarRecuperacionSeInformanSinPublicar(t *testing.T) {
	casos := map[string]func(e *entorno){
		"buscar la cuenta": func(e *entorno) { e.repo.fallarEn = map[string]error{"BuscarPorEmail": errBD} },
		"generar el token": func(e *entorno) { e.svc.generar = generadorQueFalla },
		"guardar el token": func(e *entorno) { e.repo.fallarEn = map[string]error{"GuardarToken": errBD} },
	}
	for nombre, preparar := range casos {
		t.Run(nombre, func(t *testing.T) {
			e := nuevoEntorno(t)
			e.altaYActivacion(t)
			n := len(e.pub.tipos())
			preparar(e)
			exigirTransitorio(t, e.svc.SolicitarRecuperacion(ctx, "juan@empresa.com"))
			exigirTipos(t, e.publicadosDesde(n))
		})
	}
}

// --- Activación / reset ---

func TestResetConLaBDCaidaNoEsTokenInvalido(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
	e.repo.fallarEn = map[string]error{"BuscarPorTokenHash": errBD}
	exigirTransitorio(t, e.svc.RestablecerPassword(ctx, "tok-1", passwordValida))
}

// Si el UPDATE que consume el token falla, la cuenta sigue pendiente, no se publica
// cuenta.activada y el mismo token sirve en el reintento.
func TestSiFallaConsumirElTokenLaActivacionSePuedeReintentar(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
	n := len(e.pub.tipos())
	e.repo.fallarEn = map[string]error{"ConsumirToken": errBD}

	exigirTransitorio(t, e.svc.RestablecerPassword(ctx, "tok-1", passwordValida))
	if u := e.repo.cuenta("E001"); u.Estado != dominio.EstadoPendienteActivacion || u.PasswordHash != "" {
		t.Fatalf("la cuenta cambió: %+v", u)
	}
	exigirTipos(t, e.publicadosDesde(n))

	e.repo.fallarEn = nil
	if err := e.svc.RestablecerPassword(ctx, "tok-1", passwordValida); err != nil {
		t.Fatalf("el reintento con el mismo token debe funcionar: %v", err)
	}
	exigirTipos(t, e.publicadosDesde(n), dominio.EventoCuentaActivada)
}

// --- Cambio de contraseña ---

func TestCambioDePasswordConLaBDCaidaNoEsNoAutenticado(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	token, _ := e.svc.Login(ctx, "juan@empresa.com", passwordValida)
	e.repo.fallarEn = map[string]error{"BuscarPorSujeto": errBD}
	exigirTransitorio(t, e.svc.CambiarPassword(ctx, token, passwordValida, "Distinta88"))
}

func TestSiFallaGuardarLaNuevaPasswordSigueValiendoLaAnterior(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	token, _ := e.svc.Login(ctx, "juan@empresa.com", passwordValida)
	e.repo.fallarEn = map[string]error{"ActualizarPassword": errBD}

	exigirTransitorio(t, e.svc.CambiarPassword(ctx, token, passwordValida, "Distinta88"))
	e.repo.fallarEn = nil
	if _, err := e.svc.Login(ctx, "juan@empresa.com", passwordValida); err != nil {
		t.Fatalf("la contraseña anterior debe seguir funcionando: %v", err)
	}
}

// --- Administrador semilla ---

// Un fallo de la BD al sembrar el administrador se devuelve (el arranque decide);
// nunca se toma como "ya existe".
func TestFallosDeLaBDAlSembrarElAdministradorSeDevuelven(t *testing.T) {
	for _, op := range []string{"BuscarPorSujeto", "BuscarPorEmail", "Crear"} {
		t.Run(op, func(t *testing.T) {
			e := nuevoEntorno(t)
			e.repo.fallarEn = map[string]error{op: errBD}
			if err := e.svc.SembrarAdmin(ctx, "admin@empresa.com", passwordValida); !errors.Is(err, errBD) {
				t.Fatalf("err = %v", err)
			}
			if len(e.repo.usuarios) != 0 {
				t.Fatal("no debe quedar ninguna cuenta")
			}
		})
	}
}

// Dos instancias arrancan a la vez: la otra crea el administrador entre la búsqueda y
// el INSERT. No es un error de arranque y queda un solo administrador.
func TestSiOtraInstanciaSiembraElAdministradorALaVezNoEsError(t *testing.T) {
	e := nuevoEntorno(t)
	e.repo.competidor = &dominio.Usuario{Sujeto: dominio.SujetoAdmin, Email: "admin@empresa.com",
		PasswordHash: "otro", Rol: dominio.RolAdmin, Estado: dominio.EstadoActiva}
	if err := e.svc.SembrarAdmin(ctx, "admin@empresa.com", passwordValida); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(e.repo.usuarios) != 1 || e.repo.cuenta(dominio.SujetoAdmin).PasswordHash != "otro" {
		t.Fatalf("debe quedar solo el administrador de la otra instancia: %d cuentas", len(e.repo.usuarios))
	}
}

// --- Ciclo de vida ---

func TestFalloAlBuscarLaCuentaEnUnEventoDelCicloEsTransitorio(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	e.repo.fallarEn = map[string]error{"BuscarPorEmpleadoID": errBD}
	exigirTransitorio(t, e.svc.RetirarCuenta(ctx, "r1", dominio.EmpleadoRetirado{EmpleadoID: "E001"}))
	if e.repo.procesados["r1"] {
		t.Fatal("el id no debe quedar registrado: la reentrega debe aplicarse")
	}
	e.exigirEstado(t, dominio.EstadoActiva, "")
}

func TestSiFallaLaReactivacionLaCuentaSigueSuspendidaYLaReentregaSeAplica(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	e.iniciar(t, "i1", "V-1")
	n := len(e.pub.tipos())
	e.repo.fallarEfecto = errBD

	exigirTransitorio(t, e.svc.ReactivarPorFinDeVacaciones(ctx, "f1",
		dominio.VacacionesFinalizadas{VacacionesID: "V-1", EmpleadoID: "E001"}))
	e.exigirEstado(t, dominio.EstadoSuspendidaTemporal, "V-1")
	exigirTipos(t, e.publicadosDesde(n))
	if e.repo.procesados["f1"] {
		t.Fatal("el id no debe quedar registrado")
	}

	e.finalizar(t, "f1", "V-1")
	e.exigirEstado(t, dominio.EstadoActiva, "")
}
