// Package api expone el REST del servicio de perfiles.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"talentflow/perfiles-service/internal/dominio"
	"talentflow/perfiles-service/internal/store"
)

const nombreServicio = "perfiles-service"

// camposSoloLectura se replican desde empleados-service; este servicio no los edita.
var camposSoloLectura = map[string]bool{
	"id": true, "empleadoId": true, "nombre": true, "apellido": true, "email": true,
	"cargo": true, "area": true, "departamentoId": true, "archivado": true,
	"fechaArchivado": true, "fechaCreacion": true, "fechaActualizacion": true,
}

var longitudMaxima = map[string]int{"telefono": 30, "direccion": 200, "ciudad": 100, "biografia": 2000}

type API struct {
	Store           *store.Store
	BrokerConectado func() bool
	Log             *slog.Logger
}

type campoError struct {
	Campo   string `json:"campo"`
	Mensaje string `json:"mensaje"`
}

type cuerpoError struct {
	Status    int          `json:"status"`
	Error     string       `json:"error"`
	Message   string       `json:"message"`
	Timestamp string       `json:"timestamp"`
	Path      string       `json:"path"`
	Errors    []campoError `json:"errors,omitempty"`
}

func ahora() string { return time.Now().UTC().Format("2006-01-02T15:04:05Z") }

func escribirJSON(w http.ResponseWriter, status int, valor any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(valor)
}

func escribirError(w http.ResponseWriter, r *http.Request, status int, mensaje string, errs ...campoError) {
	escribirJSON(w, status, cuerpoError{
		Status: status, Error: http.StatusText(status), Message: mensaje,
		Timestamp: ahora(), Path: r.URL.Path, Errors: errs,
	})
}

// Handler arma las rutas. Swagger vive bajo /perfiles/* para ser alcanzable por el Gateway
// sin agregarle rutas (solo enruta el prefijo /perfiles).
func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", a.health)
	mux.HandleFunc("GET /perfiles/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(paginaSwagger))
	})
	mux.HandleFunc("GET /perfiles/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(especOpenAPI))
	})
	mux.HandleFunc("GET /perfiles", a.listar)
	mux.HandleFunc("GET /perfiles/{empleadoId}", a.obtener)
	mux.HandleFunc("PUT /perfiles/{empleadoId}", a.actualizar)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		escribirError(w, r, http.StatusNotFound, "Recurso no encontrado")
	})
	return a.recuperar(a.registrar(mux))
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	db, status := "UP", http.StatusOK
	if err := a.Store.Ping(r.Context()); err != nil {
		db, status = "DOWN", http.StatusServiceUnavailable
	}
	broker := "DOWN"
	if a.BrokerConectado != nil && a.BrokerConectado() {
		broker = "UP"
	}
	estado := "UP"
	if status != http.StatusOK {
		estado = "DOWN"
	}
	escribirJSON(w, status, map[string]any{
		"status": estado, "service": nombreServicio, "timestamp": ahora(),
		"components": map[string]string{"app": "UP", "db": db, "broker": broker},
	})
}

func (a *API) listar(w http.ResponseWriter, r *http.Request) {
	var filtro *bool
	if v := r.URL.Query().Get("archivado"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			escribirError(w, r, http.StatusBadRequest, "El parámetro archivado debe ser true o false",
				campoError{"archivado", "debe ser true o false"})
			return
		}
		filtro = &b
	}
	perfiles, err := a.Store.Listar(r.Context(), filtro)
	if err != nil {
		a.falloInterno(w, r, err)
		return
	}
	escribirJSON(w, http.StatusOK, perfiles)
}

func (a *API) obtener(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("empleadoId")
	p, err := a.Store.Obtener(r.Context(), id)
	if errors.Is(err, dominio.ErrNoEncontrado) {
		escribirError(w, r, http.StatusNotFound, "No existe un perfil para el empleado "+id)
		return
	}
	if err != nil {
		a.falloInterno(w, r, err)
		return
	}
	escribirJSON(w, http.StatusOK, p)
}

func (a *API) actualizar(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("empleadoId")
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var cuerpo map[string]any
	if err := json.NewDecoder(r.Body).Decode(&cuerpo); err != nil || cuerpo == nil {
		escribirError(w, r, http.StatusBadRequest, "Cuerpo JSON inválido")
		return
	}

	cambios, errs := validarCambios(cuerpo)
	if len(errs) > 0 {
		escribirError(w, r, http.StatusBadRequest, "La petición no es válida", errs...)
		return
	}
	if len(cambios) == 0 {
		escribirError(w, r, http.StatusBadRequest, "El cuerpo no incluye campos para actualizar (telefono, direccion, ciudad, biografia)")
		return
	}

	p, err := a.Store.Actualizar(r.Context(), id, cambios)
	switch {
	case errors.Is(err, dominio.ErrNoEncontrado):
		escribirError(w, r, http.StatusNotFound, "No existe un perfil para el empleado "+id)
	case errors.Is(err, dominio.ErrArchivado):
		escribirError(w, r, http.StatusConflict, "El perfil del empleado "+id+" está archivado y no se puede modificar")
	case err != nil:
		a.falloInterno(w, r, err)
	default:
		escribirJSON(w, http.StatusOK, p)
	}
}

func validarCambios(cuerpo map[string]any) (map[string]string, []campoError) {
	cambios := map[string]string{}
	var errs []campoError
	for campo, valor := range cuerpo {
		if _, editable := store.CamposEditables[campo]; !editable {
			if camposSoloLectura[campo] {
				errs = append(errs, campoError{campo, "es de solo lectura: lo gestiona empleados-service"})
			} else {
				errs = append(errs, campoError{campo, "campo desconocido"})
			}
			continue
		}
		texto, ok := valor.(string)
		if !ok {
			errs = append(errs, campoError{campo, "debe ser un texto"})
			continue
		}
		texto = strings.TrimSpace(texto)
		if len([]rune(texto)) > longitudMaxima[campo] {
			errs = append(errs, campoError{campo, "máximo " + strconv.Itoa(longitudMaxima[campo]) + " caracteres"})
			continue
		}
		cambios[campo] = texto
	}
	return cambios, errs
}

func (a *API) falloInterno(w http.ResponseWriter, r *http.Request, err error) {
	a.Log.Error("error interno", "path", r.URL.Path, "error", err)
	escribirError(w, r, http.StatusInternalServerError, "Error interno del servidor")
}

func (a *API) recuperar(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				a.Log.Error("pánico en el manejador", "path", r.URL.Path, "panic", rec)
				escribirError(w, r, http.StatusInternalServerError, "Error interno del servidor")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type grabadorEstado struct {
	http.ResponseWriter
	status int
}

func (g *grabadorEstado) WriteHeader(s int) { g.status = s; g.ResponseWriter.WriteHeader(s) }

func (a *API) registrar(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inicio := time.Now()
		g := &grabadorEstado{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(g, r)
		if r.URL.Path != "/health" {
			a.Log.Info("petición", "método", r.Method, "ruta", r.URL.Path, "estado", g.status,
				"ms", time.Since(inicio).Milliseconds())
		}
	})
}

var _ = context.Background
