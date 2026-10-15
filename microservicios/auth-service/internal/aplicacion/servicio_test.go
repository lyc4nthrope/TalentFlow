package aplicacion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
)

const passwordValida = "Clave2026"

var inicio = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

type entorno struct {
	svc   *AuthService
	repo  *repoMemoria
	pub   *publicadorEspia
	reloj *relojManual
}

func nuevoEntorno(t *testing.T) *entorno {
	t.Helper()
	e := &entorno{repo: nuevoRepo(), pub: &publicadorEspia{}, reloj: &relojManual{t: inicio}}
	svc, err := NuevoAuthService(e.repo, e.pub, emisorFalso{}, generadorSecuencial(), e.reloj.ahora,
		Opciones{VigenciaActivacion: 24 * time.Hour, VigenciaRecuperacion: time.Hour, CostoBcrypt: bcrypt.MinCost})
	if err != nil {
		t.Fatal(err)
	}
	e.svc = svc
	return e
}

var ctx = context.Background()

// altaYActivacion crea la cuenta de E001 por evento y la activa con su token (tok-1).
func (e *entorno) altaYActivacion(t *testing.T) {
	t.Helper()
	if err := e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "Juan@Empresa.com"}); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RestablecerPassword(ctx, "tok-1", passwordValida); err != nil {
		t.Fatal(err)
	}
}

func datos(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func claves(m map[string]any) string {
	var k []string
	for c := range m {
		k = append(k, c)
	}
	sort.Strings(k)
	return strings.Join(k, ",")
}

// --- Alta por empleado.creado ---

func TestEmpleadoCreadoCreaCuentaPendienteSinPasswordYPublicaUsuarioCreado(t *testing.T) {
	e := nuevoEntorno(t)
	if err := e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: " Juan@Empresa.com "}); err != nil {
		t.Fatal(err)
	}
	u, _ := e.repo.BuscarPorSujeto(ctx, "E001")
	if u == nil || u.Estado != dominio.EstadoPendienteActivacion || u.Rol != dominio.RolUser || u.PasswordHash != "" {
		t.Fatalf("cuenta = %+v", u)
	}
	if u.TokenHash != dominio.HashToken("tok-1") || u.TokenTipo != dominio.TokenActivacion {
		t.Fatalf("token guardado = %q/%q; se esperaba el hash de tok-1 de activación", u.TokenHash, u.TokenTipo)
	}
	if u.Email != "juan@empresa.com" {
		t.Fatalf("email = %q", u.Email)
	}

	s := e.pub.ultimo()
	if s.Type != "usuario.creado" || s.Producer != "auth-service" || s.Version != 1 || len(s.ID) != 36 {
		t.Fatalf("envelope = %+v", s)
	}
	d := datos(t, s.Data)
	if got := claves(d); got != "email,empleadoId,expiraEn,tokenActivacion" {
		t.Fatalf("campos de usuario.creado = %s", got)
	}
	if d["tokenActivacion"] != "tok-1" || d["expiraEn"] != "2026-10-16T12:00:00Z" || d["empleadoId"] != "E001" {
		t.Fatalf("data = %v", d)
	}
}

func TestEmpleadoCreadoRepetidoNoDuplicaLaCuentaNiElEvento(t *testing.T) {
	e := nuevoEntorno(t)
	evento := dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "juan@empresa.com"}
	_ = e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), evento)
	if err := e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), evento); err != nil {
		t.Fatal(err)
	}
	if len(e.repo.usuarios) != 1 || len(e.pub.tipos()) != 1 {
		t.Fatalf("cuentas=%d eventos=%v", len(e.repo.usuarios), e.pub.tipos())
	}
}

func TestEmailDeOtraCuentaEsRechazoPermanenteYSeRegistraLaCausa(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "juan@empresa.com"})
	buf := capturarLog(t)
	err := e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E009", Email: "JUAN@empresa.com"})
	if !errors.Is(err, ErrRechazoPermanente) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(buf.String(), "el email ya pertenece a otra cuenta") {
		t.Fatalf("el log no explica la causa: %s", buf.String())
	}
	if len(e.repo.usuarios) != 1 || len(e.pub.tipos()) != 1 {
		t.Fatalf("cuentas=%d eventos=%v", len(e.repo.usuarios), e.pub.tipos())
	}
}

func TestDatosQueNoCabenOSinFormatoSonSolicitudInvalida(t *testing.T) {
	casos := []dominio.EmpleadoCreado{
		{EmpleadoID: strings.Repeat("E", 37), Email: "a@b.com"},
		{EmpleadoID: "E001", Email: strings.Repeat("a", 250) + "@b.com"},
		{EmpleadoID: "E001", Email: "sin-arroba.com"},
		{EmpleadoID: "E001", Email: "a@sin-punto"},
		{EmpleadoID: "E001", Email: "a@@b.com"},
		{EmpleadoID: "E001", Email: "con espacio@b.com"},
		{EmpleadoID: "E001", Email: "@b.com"},
		{EmpleadoID: "E001", Email: "a@.com"},
	}
	for _, c := range casos {
		e := nuevoEntorno(t)
		if err := e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), c); !errors.Is(err, ErrSolicitudInvalida) {
			t.Fatalf("%q/%q: err = %v", c.EmpleadoID, c.Email, err)
		}
		if len(e.repo.usuarios) != 0 {
			t.Fatalf("%q: creó la cuenta", c.Email)
		}
	}
	e := nuevoEntorno(t)
	if err := e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: strings.Repeat("E", 36), Email: "juan.perez@empresa.com.co"}); err != nil {
		t.Fatalf("datos válidos en el límite: %v", err)
	}
}

func TestRechazoPermanenteDeLaBDSePropaga(t *testing.T) {
	e := nuevoEntorno(t)
	e.repo.fallar = fmt.Errorf("crear cuenta: %w: valor demasiado largo", ErrRechazoPermanente)
	err := e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
	if !errors.Is(err, ErrRechazoPermanente) {
		t.Fatalf("err = %v", err)
	}
}

func TestEmpleadoCreadoSinEmailEsSolicitudInvalida(t *testing.T) {
	e := nuevoEntorno(t)
	err := e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001"})
	if !errors.Is(err, ErrSolicitudInvalida) {
		t.Fatalf("err = %v", err)
	}
}

func TestFalloDelRepositorioEsTransitorio(t *testing.T) {
	e := nuevoEntorno(t)
	e.repo.fallar = errors.New("bd caída")
	err := e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
	if err == nil || errors.Is(err, ErrSolicitudInvalida) {
		t.Fatalf("err = %v; se esperaba un error transitorio", err)
	}
}

// --- Activación y reset ---

func TestActivacionPasaAActivaYPublicaCuentaActivada(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	u, _ := e.repo.BuscarPorSujeto(ctx, "E001")
	if u.Estado != dominio.EstadoActiva || u.TokenHash != "" {
		t.Fatalf("cuenta = %+v", u)
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(passwordValida)) != nil {
		t.Fatal("la contraseña guardada no corresponde")
	}
	s := e.pub.ultimo()
	d := datos(t, s.Data)
	if s.Type != "cuenta.activada" || claves(d) != "email,empleadoId,motivo" || d["motivo"] != "ACTIVACION_INICIAL" || d["empleadoId"] != "E001" {
		t.Fatalf("evento = %s %v", s.Type, d)
	}
}

func TestElTokenEsDeUnSoloUso(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	if err := e.svc.RestablecerPassword(ctx, "tok-1", "OtraClave99"); !errors.Is(err, ErrTokenInvalido) {
		t.Fatalf("reutilizar el token: err = %v", err)
	}
}

func TestTokenExpiradoSeRechaza(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
	e.reloj.avanzar(24 * time.Hour)
	if err := e.svc.RestablecerPassword(ctx, "tok-1", passwordValida); !errors.Is(err, ErrTokenInvalido) {
		t.Fatalf("token vencido: err = %v", err)
	}
	u, _ := e.repo.BuscarPorSujeto(ctx, "E001")
	if u.Estado != dominio.EstadoPendienteActivacion {
		t.Fatalf("estado = %s", u.Estado)
	}
}

func TestTokenVigenteUnSegundoAntesDeVencer(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
	e.reloj.avanzar(24*time.Hour - time.Second)
	if err := e.svc.RestablecerPassword(ctx, "tok-1", passwordValida); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestTokenDesconocidoOVacioSeRechaza(t *testing.T) {
	e := nuevoEntorno(t)
	for _, tok := range []string{"", "   ", "no-existe"} {
		if err := e.svc.RestablecerPassword(ctx, tok, passwordValida); !errors.Is(err, ErrTokenInvalido) {
			t.Fatalf("token %q: err = %v", tok, err)
		}
	}
}

func TestTokenConTipoDesconocidoSeRechaza(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
	u, _ := e.repo.BuscarPorSujeto(ctx, "E001")
	_ = e.repo.GuardarToken(ctx, u.ID, dominio.HashToken("tok-1"), "OTRO", inicio.Add(time.Hour))
	if err := e.svc.RestablecerPassword(ctx, "tok-1", passwordValida); !errors.Is(err, ErrTokenInvalido) {
		t.Fatalf("err = %v", err)
	}
}

func TestResetSobreCuentaSuspendidaMantieneLaSuspension(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	e.repo.fijarEstado("E001", dominio.EstadoSuspendidaTemporal)
	_ = e.svc.SolicitarRecuperacion(ctx, "juan@empresa.com") // tok-2
	eventosAntes := len(e.pub.tipos())
	if err := e.svc.RestablecerPassword(ctx, "tok-2", "NuevaClave77"); err != nil {
		t.Fatal(err)
	}
	u, _ := e.repo.BuscarPorSujeto(ctx, "E001")
	if u.Estado != dominio.EstadoSuspendidaTemporal {
		t.Fatalf("estado = %s; un reset no debe levantar la suspensión", u.Estado)
	}
	if len(e.pub.tipos()) != eventosAntes {
		t.Fatalf("publicó %v; un reset sobre una cuenta suspendida no activa nada", e.pub.tipos()[eventosAntes:])
	}
}

func TestResetSobreCuentaRetiradaSeRechaza(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
	e.repo.fijarEstado("E001", dominio.EstadoDesactivadaPermanente)
	if err := e.svc.RestablecerPassword(ctx, "tok-1", passwordValida); !errors.Is(err, ErrTokenInvalido) {
		t.Fatalf("err = %v", err)
	}
	u, _ := e.repo.BuscarPorSujeto(ctx, "E001")
	if u.Estado != dominio.EstadoDesactivadaPermanente || u.PasswordHash != "" {
		t.Fatalf("cuenta = %+v", u)
	}
}

func TestResetValidaLaPoliticaAntesDeConsumirElToken(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
	if err := e.svc.RestablecerPassword(ctx, "tok-1", "corta"); !errors.Is(err, dominio.ErrPoliticaPassword) {
		t.Fatalf("err = %v", err)
	}
	if err := e.svc.RestablecerPassword(ctx, "tok-1", passwordValida); err != nil {
		t.Fatalf("el token debió seguir disponible: %v", err)
	}
}

func TestEstadoCambiadoEntreLecturaYConsumoNoSeSobrescribe(t *testing.T) {
	// ConsumirToken exige el estado leído: si un retiro llega en medio, no se aplica.
	e := nuevoEntorno(t)
	_ = e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
	u, _ := e.repo.BuscarPorSujeto(ctx, "E001")
	ok, _ := e.repo.ConsumirToken(ctx, u.ID, u.TokenHash, dominio.EstadoActiva, "hash", dominio.EstadoActiva)
	if ok {
		t.Fatal("el doble aplicó un consumo con un estado esperado distinto del real")
	}
}

func TestSiOtroUsoConsumioElTokenNoSeActivaNiSePublica(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
	e.repo.consumoGanadoPorOtro = true
	antes := len(e.pub.tipos())
	if err := e.svc.RestablecerPassword(ctx, "tok-1", passwordValida); !errors.Is(err, ErrTokenInvalido) {
		t.Fatalf("err = %v", err)
	}
	if len(e.pub.tipos()) != antes {
		t.Fatalf("publicó %v aunque el token ya se había consumido", e.pub.tipos()[antes:])
	}
}

func TestEnLaBDSoloQuedaElHashDelToken(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
	_ = e.svc.SolicitarRecuperacion(ctx, "a@b.com")
	u, _ := e.repo.BuscarPorSujeto(ctx, "E001")
	if u.TokenHash == "tok-2" || u.TokenHash != dominio.HashToken("tok-2") {
		t.Fatalf("token guardado = %q; debe ser el hash, no el token", u.TokenHash)
	}
}

// --- Login ---

func TestLoginActivaDevuelveTokenConSujetoYRol(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	token, err := e.svc.Login(ctx, " JUAN@empresa.com", passwordValida)
	if err != nil {
		t.Fatal(err)
	}
	if token != "token:E001:USER:juan@empresa.com" {
		t.Fatalf("token = %q", token)
	}
}

func TestLoginFallaEnTodoEstadoNoActivo(t *testing.T) {
	for _, estado := range []string{dominio.EstadoPendienteActivacion, dominio.EstadoSuspendidaTemporal, dominio.EstadoDesactivadaPermanente} {
		e := nuevoEntorno(t)
		e.altaYActivacion(t)
		e.repo.fijarEstado("E001", estado)
		if _, err := e.svc.Login(ctx, "juan@empresa.com", passwordValida); !errors.Is(err, ErrCredencialesInvalidas) {
			t.Fatalf("estado %s: err = %v", estado, err)
		}
	}
}

func TestLoginSinActivarFalla(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
	if _, err := e.svc.Login(ctx, "a@b.com", ""); !errors.Is(err, ErrCredencialesInvalidas) {
		t.Fatalf("err = %v", err)
	}
	if _, err := e.svc.Login(ctx, "a@b.com", passwordValida); !errors.Is(err, ErrCredencialesInvalidas) {
		t.Fatalf("err = %v", err)
	}
}

func TestLoginConPasswordIncorrectaOEmailDesconocidoFalla(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	casos := [][2]string{{"juan@empresa.com", "Clave2027"}, {"nadie@empresa.com", passwordValida}, {"", passwordValida}}
	for _, c := range casos {
		if _, err := e.svc.Login(ctx, c[0], c[1]); !errors.Is(err, ErrCredencialesInvalidas) {
			t.Fatalf("%v: err = %v", c, err)
		}
	}
}

func TestLaPasswordNoSeRecorta(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"})
	if err := e.svc.RestablecerPassword(ctx, "tok-1", " Clave2026 "); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Login(ctx, "a@b.com", "Clave2026"); err == nil {
		t.Fatal("aceptó la contraseña sin los espacios: se recortó en algún camino")
	}
	if _, err := e.svc.Login(ctx, "a@b.com", " Clave2026 "); err != nil {
		t.Fatalf("la contraseña con espacios no sirve para iniciar sesión: %v", err)
	}
}

// --- Recuperación ---

func TestRecuperacionPublicaUsuarioRecuperacionYReemplazaElTokenAnterior(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	_ = e.svc.SolicitarRecuperacion(ctx, "juan@empresa.com")                     // tok-2
	if err := e.svc.SolicitarRecuperacion(ctx, "JUAN@empresa.com"); err != nil { // tok-3
		t.Fatal(err)
	}
	s := e.pub.ultimo()
	d := datos(t, s.Data)
	if s.Type != "usuario.recuperacion" || claves(d) != "email,expiraEn,tokenRecuperacion" ||
		d["tokenRecuperacion"] != "tok-3" || d["expiraEn"] != "2026-10-15T13:00:00Z" {
		t.Fatalf("evento = %s %v", s.Type, d)
	}
	if err := e.svc.RestablecerPassword(ctx, "tok-2", "NuevaClave77"); !errors.Is(err, ErrTokenInvalido) {
		t.Fatalf("el token anterior sigue sirviendo: %v", err)
	}
	if err := e.svc.RestablecerPassword(ctx, "tok-3", "NuevaClave77"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Login(ctx, "juan@empresa.com", "NuevaClave77"); err != nil {
		t.Fatalf("login con la contraseña nueva: %v", err)
	}
}

func TestTokenDeRecuperacionVenceEnUnaHora(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	_ = e.svc.SolicitarRecuperacion(ctx, "juan@empresa.com")
	e.reloj.avanzar(time.Hour)
	if err := e.svc.RestablecerPassword(ctx, "tok-2", "NuevaClave77"); !errors.Is(err, ErrTokenInvalido) {
		t.Fatalf("err = %v", err)
	}
}

func TestRecuperacionDeEmailDesconocidoOCuentaRetiradaNoHaceNada(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	e.repo.fijarEstado("E001", dominio.EstadoDesactivadaPermanente)
	antes := len(e.pub.tipos())
	for _, email := range []string{"nadie@empresa.com", "juan@empresa.com", ""} {
		if err := e.svc.SolicitarRecuperacion(ctx, email); err != nil {
			t.Fatalf("%q: err = %v", email, err)
		}
	}
	if len(e.pub.tipos()) != antes {
		t.Fatalf("publicó %v", e.pub.tipos()[antes:])
	}
}

func TestFalloAlPublicarNoRevierteElCambio(t *testing.T) {
	e := nuevoEntorno(t)
	e.pub.fallar = errors.New("broker caído")
	if err := e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "a@b.com"}); err != nil {
		t.Fatalf("err = %v", err)
	}
	if u, _ := e.repo.BuscarPorSujeto(ctx, "E001"); u == nil {
		t.Fatal("la cuenta no quedó creada")
	}
}

// --- Cambio de contraseña ---

func TestCambioDePassword(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	token, _ := e.svc.Login(ctx, "juan@empresa.com", passwordValida)
	if err := e.svc.CambiarPassword(ctx, token, passwordValida, "Distinta88"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.Login(ctx, "juan@empresa.com", passwordValida); err == nil {
		t.Fatal("la contraseña antigua sigue funcionando")
	}
	if _, err := e.svc.Login(ctx, "juan@empresa.com", "Distinta88"); err != nil {
		t.Fatalf("la nueva no funciona: %v", err)
	}
}

func TestCambioDePasswordAnulaElTokenDeRecuperacionPendiente(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	_ = e.svc.SolicitarRecuperacion(ctx, "juan@empresa.com") // tok-2
	token, _ := e.svc.Login(ctx, "juan@empresa.com", passwordValida)
	if err := e.svc.CambiarPassword(ctx, token, passwordValida, "Distinta88"); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RestablecerPassword(ctx, "tok-2", "OtraMas777"); !errors.Is(err, ErrTokenInvalido) {
		t.Fatalf("un enlace de recuperación previo al cambio sigue sirviendo: %v", err)
	}
}

func TestCambioDePasswordRechazos(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	token, _ := e.svc.Login(ctx, "juan@empresa.com", passwordValida)
	casos := []struct {
		nombre, token, actual, nueva string
		esperado                     error
	}{
		{"token inválido", "basura", passwordValida, "Distinta88", ErrNoAutenticado},
		{"actual incorrecta", token, "Incorrecta1", "Distinta88", ErrPasswordActual},
		{"actual vacía", token, "", "Distinta88", ErrPasswordActual},
		{"nueva sin dígito", token, passwordValida, "SinDigitos", dominio.ErrPoliticaPassword},
		{"nueva igual", token, passwordValida, passwordValida, ErrPasswordRepetida},
	}
	for _, c := range casos {
		if err := e.svc.CambiarPassword(ctx, c.token, c.actual, c.nueva); !errors.Is(err, c.esperado) {
			t.Fatalf("%s: err = %v", c.nombre, err)
		}
	}
}

func TestCambioDePasswordConCuentaSuspendidaFalla(t *testing.T) {
	e := nuevoEntorno(t)
	e.altaYActivacion(t)
	token, _ := e.svc.Login(ctx, "juan@empresa.com", passwordValida)
	e.repo.fijarEstado("E001", dominio.EstadoSuspendidaTemporal)
	if err := e.svc.CambiarPassword(ctx, token, passwordValida, "Distinta88"); !errors.Is(err, ErrNoAutenticado) {
		t.Fatalf("err = %v", err)
	}
}

// --- Administrador semilla ---

func capturarLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	anterior := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(anterior) })
	return &buf
}

func TestSembrarAdminCreaAdminActivoSinEscribirLaPasswordEnElLog(t *testing.T) {
	e := nuevoEntorno(t)
	buf := capturarLog(t)
	if err := e.svc.SembrarAdmin(ctx, "admin@empresa.com", "SemillaSegura9"); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.SembrarAdmin(ctx, "admin@empresa.com", "SemillaSegura9"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "SemillaSegura9") {
		t.Fatalf("la contraseña apareció en el log: %s", buf.String())
	}
	u, _ := e.repo.BuscarPorSujeto(ctx, dominio.SujetoAdmin)
	if u == nil || u.Rol != dominio.RolAdmin || u.Estado != dominio.EstadoActiva || len(e.repo.usuarios) != 1 {
		t.Fatalf("admin = %+v (cuentas %d)", u, len(e.repo.usuarios))
	}
	token, err := e.svc.Login(ctx, "admin@empresa.com", "SemillaSegura9")
	if err != nil || token != "token:admin:ADMIN:admin@empresa.com" {
		t.Fatalf("login admin: %q %v", token, err)
	}
}

func TestSembrarAdminConEmailDeOtraCuentaNoLoCreaYLoDiceAsi(t *testing.T) {
	e := nuevoEntorno(t)
	_ = e.svc.CrearCuentaDesdeEmpleado(ctx, nuevoEventoID(), dominio.EmpleadoCreado{EmpleadoID: "E001", Email: "admin@empresa.com"})
	buf := capturarLog(t)
	if err := e.svc.SembrarAdmin(ctx, "admin@empresa.com", "SemillaSegura9"); err != nil {
		t.Fatal(err)
	}
	if u, _ := e.repo.BuscarPorSujeto(ctx, dominio.SujetoAdmin); u != nil {
		t.Fatal("creó un administrador con un email ajeno")
	}
	if !strings.Contains(buf.String(), "ADMIN_EMAIL ya pertenece a una cuenta que no es el administrador") ||
		strings.Contains(buf.String(), "el administrador semilla ya existe") {
		t.Fatalf("mensaje engañoso: %s", buf.String())
	}
}

func TestSembrarAdminSinConfiguracionOConPasswordDebilNoCreaNada(t *testing.T) {
	e := nuevoEntorno(t)
	buf := capturarLog(t)
	for _, c := range [][2]string{{"", "SemillaSegura9"}, {"admin@empresa.com", ""}, {"admin@empresa.com", "debil"}} {
		if err := e.svc.SembrarAdmin(ctx, c[0], c[1]); err != nil {
			t.Fatal(err)
		}
	}
	if len(e.repo.usuarios) != 0 {
		t.Fatal("creó un administrador sin configuración válida")
	}
	if strings.Contains(buf.String(), "debil") {
		t.Fatal("la contraseña débil apareció en el log")
	}
}
