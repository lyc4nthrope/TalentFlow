package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/aplicacion"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
)

// casosFalsos devuelve el error configurado y registra lo que recibió.
type casosFalsos struct {
	err           error
	recibido      []string
	tokenDevuelto string
	alRecuperar   func(context.Context)
}

func (c *casosFalsos) Login(_ context.Context, email, password string) (string, error) {
	c.recibido = []string{email, password}
	return c.tokenDevuelto, c.err
}
func (c *casosFalsos) SolicitarRecuperacion(ctx context.Context, email string) error {
	c.recibido = []string{email}
	if c.alRecuperar != nil {
		c.alRecuperar(ctx)
	}
	return c.err
}
func (c *casosFalsos) RestablecerPassword(_ context.Context, token, nueva string) error {
	c.recibido = []string{token, nueva}
	return c.err
}
func (c *casosFalsos) CambiarPassword(_ context.Context, tokenAcceso, actual, nueva string) error {
	c.recibido = []string{tokenAcceso, actual, nueva}
	return c.err
}

func llamar(t *testing.T, casos *casosFalsos, ruta, cuerpo string, cabeceras map[string]string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, ruta, strings.NewReader(cuerpo))
	for k, v := range cabeceras {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h := NuevoAuthHandler(casos)
	h.enSegundoPlano = func(f func()) { f() } // síncrono: las pruebas ven el efecto
	ConfigurarRutas(h).ServeHTTP(rec, req)
	var m map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &m)
	return rec, m
}

func TestLoginExitosoDevuelveElToken(t *testing.T) {
	c := &casosFalsos{tokenDevuelto: "jwt"}
	rec, m := llamar(t, c, "/auth/login", `{"email":"a@b.com","password":"Clave2026"}`, nil)
	if rec.Code != 200 || m["token"] != "jwt" || m["tipo"] != "Bearer" {
		t.Fatalf("%d %v", rec.Code, m)
	}
}

func TestLoginAceptaUsernameComoEmail(t *testing.T) {
	c := &casosFalsos{tokenDevuelto: "jwt"}
	llamar(t, c, "/auth/login", `{"username":"a@b.com","password":"Clave2026"}`, nil)
	if c.recibido[0] != "a@b.com" {
		t.Fatalf("recibido = %v", c.recibido)
	}
}

func TestCodigosDeEstadoPorError(t *testing.T) {
	casos := []struct {
		ruta     string
		err      error
		esperado int
	}{
		{"/auth/login", aplicacion.ErrCredencialesInvalidas, 401},
		{"/auth/reset-password", aplicacion.ErrTokenInvalido, 401},
		{"/auth/reset-password", dominio.ErrPoliticaPassword, 400},
		{"/auth/reset-password", fmt.Errorf("envuelto: %w", aplicacion.ErrTokenInvalido), 401},
		{"/auth/change-password", aplicacion.ErrPasswordActual, 400},
		{"/auth/change-password", aplicacion.ErrPasswordRepetida, 400},
		{"/auth/change-password", aplicacion.ErrNoAutenticado, 401},
	}
	for _, c := range casos {
		rec, m := llamar(t, &casosFalsos{err: c.err}, c.ruta, `{}`, map[string]string{"Authorization": "Bearer x"})
		if rec.Code != c.esperado {
			t.Errorf("%s con %v: código %d, se esperaba %d", c.ruta, c.err, rec.Code, c.esperado)
		}
		if m["status"] != float64(c.esperado) || m["error"] == nil || m["message"] == nil || m["path"] != c.ruta {
			t.Errorf("%s: cuerpo sin el formato del ecosistema: %v", c.ruta, m)
		}
	}
}

func TestErrorInternoNoRevelaElDetalle(t *testing.T) {
	interno := errors.New("pq: password authentication failed for user secreto_interno")
	for _, ruta := range []string{"/auth/login", "/auth/reset-password", "/auth/change-password"} {
		rec, m := llamar(t, &casosFalsos{err: interno}, ruta, `{}`, map[string]string{"Authorization": "Bearer x"})
		if rec.Code != 500 || strings.Contains(rec.Body.String(), "secreto_interno") || m["message"] != "Error interno del servidor" {
			t.Fatalf("%s: %d %s", ruta, rec.Code, rec.Body.String())
		}
	}
}

func TestRecuperacionRespondeSiempre202(t *testing.T) {
	for _, err := range []error{nil, errors.New("bd caída")} {
		rec, m := llamar(t, &casosFalsos{err: err}, "/auth/recover-password", `{"email":"nadie@b.com"}`, nil)
		if rec.Code != 202 || m["message"] == nil || strings.Contains(rec.Body.String(), "bd caída") {
			t.Fatalf("err %v: %d %s", err, rec.Code, rec.Body.String())
		}
	}
}

func TestRecuperacionResponde202SinEsperarAlTrabajo(t *testing.T) {
	c := &casosFalsos{}
	var pendiente func()
	h := NuevoAuthHandler(c)
	h.enSegundoPlano = func(f func()) { pendiente = f } // no se ejecuta todavía
	req := httptest.NewRequest(http.MethodPost, "/auth/recover-password", strings.NewReader(`{"email":"a@b.com"}`))
	ctxReq, cancelar := context.WithCancel(req.Context())
	rec := httptest.NewRecorder()
	ConfigurarRutas(h).ServeHTTP(rec, req.WithContext(ctxReq))
	if rec.Code != 202 || c.recibido != nil || pendiente == nil {
		t.Fatalf("código %d, caso de uso llamado antes de responder: %v", rec.Code, c.recibido)
	}
	cancelar() // el cliente ya se fue: el trabajo debe seguir con su propio plazo
	var errDurante error
	var conPlazo bool
	c.alRecuperar = func(ctx context.Context) {
		errDurante = ctx.Err()
		_, conPlazo = ctx.Deadline()
	}
	pendiente()
	if len(c.recibido) != 1 || c.recibido[0] != "a@b.com" {
		t.Fatalf("recibido = %v", c.recibido)
	}
	if errDurante != nil {
		t.Fatal("el trabajo en segundo plano heredó la cancelación de la petición")
	}
	if !conPlazo {
		t.Fatal("el trabajo en segundo plano no tiene plazo")
	}
}

func TestCuerpoInvalidoEs400(t *testing.T) {
	for _, ruta := range []string{"/auth/login", "/auth/recover-password", "/auth/reset-password", "/auth/change-password"} {
		rec, m := llamar(t, &casosFalsos{}, ruta, `{no es json`, map[string]string{"Authorization": "Bearer x"})
		if rec.Code != 400 || m["message"] != "Cuerpo JSON inválido" {
			t.Fatalf("%s: %d %v", ruta, rec.Code, m)
		}
	}
}

func TestCambioDePasswordSinBearerEs401YNoLlamaAlCasoDeUso(t *testing.T) {
	for _, cab := range []map[string]string{nil, {"Authorization": "Basic abc"}, {"Authorization": "Bearer   "}} {
		c := &casosFalsos{}
		rec, _ := llamar(t, c, "/auth/change-password", `{"currentPassword":"a","newPassword":"b"}`, cab)
		if rec.Code != 401 || c.recibido != nil {
			t.Fatalf("%v: %d, recibido %v", cab, rec.Code, c.recibido)
		}
	}
}

func TestCambioDePasswordPasaElTokenYLasPasswordsSinRecortar(t *testing.T) {
	c := &casosFalsos{}
	rec, _ := llamar(t, c, "/auth/change-password", `{"currentPassword":" Vieja1a ","newPassword":" Nueva1a "}`,
		map[string]string{"Authorization": "Bearer jwt.valido"})
	if rec.Code != 200 || c.recibido[0] != "jwt.valido" || c.recibido[1] != " Vieja1a " || c.recibido[2] != " Nueva1a " {
		t.Fatalf("%d %q", rec.Code, c.recibido)
	}
}

func TestEndpointInventadoYaNoExiste(t *testing.T) {
	rec, _ := llamar(t, &casosFalsos{}, "/auth/request-password-reset", `{"email":"a@b.com"}`, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("código %d", rec.Code)
	}
}

// casosBloqueantes se queda esperando en SolicitarRecuperacion hasta que se libere.
type casosBloqueantes struct {
	casosFalsos
	entro, liberar chan struct{}
}

func (c *casosBloqueantes) SolicitarRecuperacion(ctx context.Context, email string) error {
	close(c.entro)
	<-c.liberar
	return nil
}

func TestConElConstructorRealLaRecuperacionNoBloqueaLaRespuesta(t *testing.T) {
	c := &casosBloqueantes{entro: make(chan struct{}), liberar: make(chan struct{})}
	defer close(c.liberar)
	respondio := make(chan int, 1)
	go func() {
		req := httptest.NewRequest(http.MethodPost, "/auth/recover-password", strings.NewReader(`{"email":"a@b.com"}`))
		rec := httptest.NewRecorder()
		ConfigurarRutas(NuevoAuthHandler(c)).ServeHTTP(rec, req)
		respondio <- rec.Code
	}()
	select {
	case codigo := <-respondio:
		if codigo != http.StatusAccepted {
			t.Fatalf("código %d", codigo)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("la respuesta esperó a que terminara la recuperación")
	}
	select {
	case <-c.entro:
	case <-time.After(2 * time.Second):
		t.Fatal("la recuperación nunca se ejecutó en segundo plano")
	}
}
