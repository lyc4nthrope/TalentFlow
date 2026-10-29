// Package metricas publica las métricas Prometheus del servicio (Reto 8, D5). Es
// infraestructura (D7): no entra en el umbral de cobertura de negocio, pero tiene su prueba.
package metricas

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// RutaOtra agrupa lo que no casa con ninguna ruta: sin ella, cada URL inventada por un
// cliente sería una serie nueva en Prometheus.
const RutaOtra = "other"

// Hasta 10 s: el umbral de la alerta de latencia (2 s) cae en un bucket exacto.
var bucketsSegundos = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10}

// Metricas reúne el registro propio (no el global) y el contrato común de los 7
// servicios: http_requests_total y http_request_duration_seconds.
type Metricas struct {
	registro   *prometheus.Registry
	peticiones *prometheus.CounterVec
	duracion   *prometheus.HistogramVec
}

func Nuevas() *Metricas {
	etiquetas := []string{"method", "route", "status"}
	m := &Metricas{
		registro: prometheus.NewRegistry(),
		peticiones: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Peticiones HTTP recibidas",
		}, etiquetas),
		duracion: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duración de las peticiones HTTP en segundos",
			Buckets: bucketsSegundos,
		}, etiquetas),
	}
	// CPU, memoria y descriptores del proceso, más el runtime de Go (goroutines, GC).
	m.registro.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
		m.peticiones,
		m.duracion,
	)
	return m
}

// Instrumentar sirve GET /metrics y mide todo lo demás. /metrics queda fuera del
// conteo para que los scrapes no inflen el tráfico.
func (m *Metricas) Instrumentar(app http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(m.registro, promhttp.HandlerOpts{}))
	mux.Handle("/", m.medir(app))
	return mux
}

func (m *Metricas) medir(app http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		rw := &escritorConEstado{ResponseWriter: w, estado: http.StatusOK}
		app.ServeHTTP(rw, r)
		// El ServeMux de la app deja en r.Pattern la ruta que casó.
		valores := prometheus.Labels{
			"method": r.Method,
			"route":  ruta(r.Pattern),
			"status": strconv.Itoa(rw.estado),
		}
		m.peticiones.With(valores).Inc()
		m.duracion.With(valores).Observe(time.Since(inicio).Seconds())
	})
}

// ruta convierte el patrón del ServeMux ("GET /notificaciones/{empleadoId}") en la
// etiqueta ("/notificaciones/{empleadoId}"): el patrón y no la URL, para no crear una
// serie por cada id. Sin patrón (404 del mux) o con el comodín "/", es "other".
func ruta(patron string) string {
	if _, resto, ok := strings.Cut(patron, " "); ok {
		patron = resto
	}
	if patron == "" || patron == "/" {
		return RutaOtra
	}
	return patron
}

// escritorConEstado recuerda el código HTTP que escribió el handler.
type escritorConEstado struct {
	http.ResponseWriter
	estado      int
	escribioCab bool
}

func (e *escritorConEstado) WriteHeader(codigo int) {
	if !e.escribioCab {
		e.estado, e.escribioCab = codigo, true
	}
	e.ResponseWriter.WriteHeader(codigo)
}
