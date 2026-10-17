package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// main conecta el consumidor con amqp.ConfigConsumidorAuth (prefetch 1, para
// conservar el orden de los eventos ante reintentos) y no con una configuración
// armada a mano.
func TestMainUsaLaConfiguracionOrdenadaDelConsumidor(t *testing.T) {
	fuente, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	codigo := string(fuente)
	if !strings.Contains(codigo, "amqp.NuevoConsumidor(amqp.ConfigConsumidorAuth(cfg.AMQPURL, cfg.Exchange), servicio)") {
		t.Fatal("main debe crear el consumidor con amqp.ConfigConsumidorAuth")
	}
	if strings.Contains(codigo, "Prefetch:") {
		t.Fatal("main no debe fijar el prefetch a mano")
	}
}

func TestComprobarSalud(t *testing.T) {
	sano := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer sano.Close()
	enfermo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer enfermo.Close()
	caido := httptest.NewServer(http.NotFoundHandler())
	urlCaida := caido.URL
	caido.Close()

	cliente := &http.Client{Timeout: timeoutChequeo}
	casos := []struct {
		nombre string
		url    string
		codigo int
	}{
		{"200 -> sano", sano.URL, 0},
		{"503 -> enfermo", enfermo.URL, 1},
		{"sin servidor -> enfermo", urlCaida, 1},
	}
	for _, c := range casos {
		if got := codigoSalida(comprobarSalud(c.url, cliente)); got != c.codigo {
			t.Errorf("%s: código de salida %d, esperado %d", c.nombre, got, c.codigo)
		}
	}
}

func TestComprobarSaludRespetaElPlazo(t *testing.T) {
	colgado := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer colgado.Close()
	inicio := time.Now()
	if comprobarSalud(colgado.URL, &http.Client{Timeout: 200 * time.Millisecond}) {
		t.Fatal("un servidor que no responde no puede contar como sano")
	}
	if time.Since(inicio) > 2*time.Second {
		t.Fatal("no respetó el plazo")
	}
}

func TestURLSaludLocalUsaPORT(t *testing.T) {
	t.Setenv("PORT", "9999")
	if got := urlSaludLocal(); got != "http://127.0.0.1:9999/health" {
		t.Fatalf("url = %q", got)
	}
	t.Setenv("PORT", "")
	if got := urlSaludLocal(); got != "http://127.0.0.1:8086/health" {
		t.Fatalf("url por defecto = %q", got)
	}
}

// TestMain permite ejecutar el main real en un proceso hijo (ver ejecutarMain).
func TestMain(m *testing.M) {
	if os.Getenv("AUTH_EJECUTAR_MAIN") == "1" {
		os.Args = append([]string{"auth-service"}, strings.Fields(os.Getenv("AUTH_ARGS"))...)
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// ejecutarMain corre main() en un proceso hijo con un entorno mínimo (sin JWT_SECRET ni
// credenciales) y devuelve su código de salida y su salida combinada.
func ejecutarMain(t *testing.T, args string, entorno ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append([]string{"AUTH_EJECUTAR_MAIN=1", "AUTH_ARGS=" + args, "PATH=" + os.Getenv("PATH")}, entorno...)
	salida, err := cmd.CombinedOutput()
	codigo := 0
	if err != nil {
		var errSalida *exec.ExitError
		if !errors.As(err, &errSalida) {
			t.Fatalf("no se pudo ejecutar main: %v", err)
		}
		codigo = errSalida.ExitCode()
	}
	return codigo, string(salida)
}

// El healthcheck de Docker no debe depender de la configuración del servicio: con -salud
// main solo consulta /health, aunque falten JWT_SECRET y las credenciales.
func TestSaludNoCargaLaConfiguracion(t *testing.T) {
	for _, c := range []struct {
		nombre string
		estado int
		codigo int
	}{
		{"servicio sano", http.StatusOK, 0},
		{"servicio con la BD caída", http.StatusServiceUnavailable, 1},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/health" {
					t.Errorf("ruta consultada = %q", r.URL.Path)
				}
				w.WriteHeader(c.estado)
			}))
			defer servidor.Close()
			puerto := servidor.URL[strings.LastIndex(servidor.URL, ":")+1:]

			codigo, salida := ejecutarMain(t, "-salud", "PORT="+puerto)
			if codigo != c.codigo {
				t.Fatalf("código de salida %d, esperado %d; salida: %s", codigo, c.codigo, salida)
			}
			if strings.Contains(salida, "configuración inválida") || strings.Contains(salida, "obligatori") {
				t.Fatalf("-salud cargó la configuración: %s", salida)
			}
		})
	}
}

// Contraste: sin -salud, main sí exige la configuración y termina con 1.
func TestSinSaludMainExigeLaConfiguracion(t *testing.T) {
	codigo, salida := ejecutarMain(t, "")
	if codigo != 1 || !strings.Contains(salida, "JWT_SECRET es obligatorio") {
		t.Fatalf("código %d, salida: %s", codigo, salida)
	}
}
