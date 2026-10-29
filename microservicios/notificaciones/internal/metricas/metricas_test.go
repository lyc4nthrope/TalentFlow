package metricas

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// app de prueba con las mismas formas de ruta que los servicios reales.
func appDePrueba() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /recursos/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok")) //nolint:errcheck
	})
	mux.HandleFunc("POST /recursos", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.WriteHeader(http.StatusInternalServerError) // un segundo WriteHeader no cuenta
	})
	return mux
}

func pedir(t *testing.T, h http.Handler, metodo, url string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(metodo, url, nil))
	return rec
}

func leerMetricas(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := pedir(t, h, http.MethodGet, "/metrics")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics = %d", rec.Code)
	}
	cuerpo, _ := io.ReadAll(rec.Body)
	return string(cuerpo)
}

func TestCuentaPorPatronMetodoYEstado(t *testing.T) {
	h := Nuevas().Instrumentar(appDePrueba())
	pedir(t, h, http.MethodGet, "/recursos/1")
	pedir(t, h, http.MethodGet, "/recursos/2")
	pedir(t, h, http.MethodPost, "/recursos")
	pedir(t, h, http.MethodGet, "/no-existe")

	texto := leerMetricas(t, h)
	esperadas := []string{
		`http_requests_total{method="GET",route="/recursos/{id}",status="200"} 2`,
		`http_requests_total{method="POST",route="/recursos",status="201"} 1`,
		`http_requests_total{method="GET",route="other",status="404"} 1`,
		`http_request_duration_seconds_bucket{method="GET",route="/recursos/{id}",status="200",le="2"} 2`,
	}
	for _, linea := range esperadas {
		if !strings.Contains(texto, linea) {
			t.Errorf("falta %q en /metrics", linea)
		}
	}
	if strings.Contains(texto, `route="/metrics"`) {
		t.Error("los scrapes de /metrics no deben contarse")
	}
	for _, m := range []string{"process_cpu_seconds_total", "process_resident_memory_bytes", "go_goroutines"} {
		if !strings.Contains(texto, m) {
			t.Errorf("falta la métrica por defecto %s", m)
		}
	}
}

func TestRuta(t *testing.T) {
	casos := map[string]string{
		"GET /notificaciones/{empleadoId}": "/notificaciones/{empleadoId}",
		"/health":                          "/health",
		"/":                                RutaOtra,
		"":                                 RutaOtra,
	}
	for patron, esperada := range casos {
		if got := ruta(patron); got != esperada {
			t.Errorf("ruta(%q) = %q, se esperaba %q", patron, got, esperada)
		}
	}
}
