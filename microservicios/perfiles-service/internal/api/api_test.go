package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"talentflow/perfiles-service/internal/api"
	"talentflow/perfiles-service/internal/consumidor"
	"talentflow/perfiles-service/internal/store"
	"talentflow/perfiles-service/internal/testutil"
)

type entorno struct {
	srv  *httptest.Server
	cons *consumidor.Consumidor
}

func nuevo(t *testing.T) *entorno {
	db := testutil.Conectar(t)
	s := store.New(db)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer((&api.API{Store: s, BrokerConectado: func() bool { return true }, Log: log}).Handler())
	t.Cleanup(srv.Close)
	return &entorno{srv: srv, cons: &consumidor.Consumidor{Store: s, Log: log}}
}

func (e *entorno) crearEmpleado(t *testing.T, id, nombre string) {
	t.Helper()
	cuerpo := fmt.Sprintf(`{"id":"msg-%s","type":"empleado.creado","version":1,"occurredAt":"2026-10-10T12:00:00Z","producer":"empleados-service",
		"data":{"empleadoId":%q,"nombre":%q,"apellido":"X","email":"%s@empresa.com","cargo":"Dev","area":"TI","departamentoId":"IT"}}`, id, id, nombre, strings.ToLower(id))
	if _, err := e.cons.Manejar(context.Background(), []byte(cuerpo)); err != nil {
		t.Fatal(err)
	}
}

func (e *entorno) retirar(t *testing.T, id string) {
	t.Helper()
	cuerpo := fmt.Sprintf(`{"id":"ret-%s","type":"empleado.retirado","version":1,"occurredAt":"2026-10-10T12:00:00Z","producer":"empleados-service","data":{"empleadoId":%q,"email":"x@x.com"}}`, id, id)
	if _, err := e.cons.Manejar(context.Background(), []byte(cuerpo)); err != nil {
		t.Fatal(err)
	}
}

func (e *entorno) peticion(t *testing.T, metodo, ruta, cuerpo string) (int, map[string]any, []map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(metodo, e.srv.URL+ruta, strings.NewReader(cuerpo))
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	bytes, _ := io.ReadAll(res.Body)
	var obj map[string]any
	var lista []map[string]any
	if err := json.Unmarshal(bytes, &obj); err != nil {
		_ = json.Unmarshal(bytes, &lista)
	}
	return res.StatusCode, obj, lista
}

func TestConsultarPerfil(t *testing.T) {
	e := nuevo(t)
	e.crearEmpleado(t, "E001", "Juan")

	status, p, _ := e.peticion(t, "GET", "/perfiles/E001", "")
	if status != 200 || p["empleadoId"] != "E001" || p["nombre"] != "Juan" || p["telefono"] != "" || p["archivado"] != false {
		t.Errorf("status=%d perfil=%v", status, p)
	}
	if !strings.HasSuffix(p["fechaCreacion"].(string), "Z") {
		t.Errorf("fechaCreacion sin formato ISO UTC: %v", p["fechaCreacion"])
	}
}

func TestPerfilInexistenteDa404ConMensaje(t *testing.T) {
	e := nuevo(t)
	status, cuerpo, _ := e.peticion(t, "GET", "/perfiles/E999", "")
	if status != 404 || !strings.Contains(cuerpo["message"].(string), "E999") || cuerpo["path"] != "/perfiles/E999" || cuerpo["timestamp"] == nil {
		t.Errorf("status=%d cuerpo=%v", status, cuerpo)
	}
}

func TestListar(t *testing.T) {
	e := nuevo(t)
	e.crearEmpleado(t, "E001", "Juan")
	e.crearEmpleado(t, "E002", "Ana")
	e.retirar(t, "E002")

	_, _, todos := e.peticion(t, "GET", "/perfiles", "")
	_, _, archivados := e.peticion(t, "GET", "/perfiles?archivado=true", "")
	_, _, activos := e.peticion(t, "GET", "/perfiles?archivado=false", "")
	if len(todos) != 2 || len(archivados) != 1 || len(activos) != 1 || archivados[0]["empleadoId"] != "E002" {
		t.Errorf("todos=%d archivados=%d activos=%d", len(todos), len(archivados), len(activos))
	}
	if status, _, _ := e.peticion(t, "GET", "/perfiles?archivado=quizas", ""); status != 400 {
		t.Errorf("archivado inválido: status=%d", status)
	}
}

func TestListaVaciaEsArregloNoNull(t *testing.T) {
	e := nuevo(t)
	req, _ := http.NewRequest("GET", e.srv.URL+"/perfiles", nil)
	res, _ := http.DefaultClient.Do(req)
	b, _ := io.ReadAll(res.Body)
	if strings.TrimSpace(string(b)) != "[]" {
		t.Errorf("cuerpo=%q", b)
	}
}

func TestActualizarPerfil(t *testing.T) {
	e := nuevo(t)
	e.crearEmpleado(t, "E001", "Juan")

	status, p, _ := e.peticion(t, "PUT", "/perfiles/E001", `{"telefono":"3001234567","ciudad":"Armenia","biografia":"Ingeniero de sistemas"}`)
	if status != 200 || p["telefono"] != "3001234567" || p["ciudad"] != "Armenia" || p["direccion"] != "" {
		t.Fatalf("status=%d perfil=%v", status, p)
	}
	// Actualización parcial: un segundo PUT no borra lo anterior
	_, p, _ = e.peticion(t, "PUT", "/perfiles/E001", `{"direccion":"Calle 10 # 5-20"}`)
	if p["direccion"] != "Calle 10 # 5-20" || p["telefono"] != "3001234567" {
		t.Errorf("la actualización parcial perdió datos: %v", p)
	}
	_, p, _ = e.peticion(t, "GET", "/perfiles/E001", "")
	if p["biografia"] != "Ingeniero de sistemas" {
		t.Errorf("no persistió: %v", p)
	}
}

func TestActualizarValidaciones(t *testing.T) {
	e := nuevo(t)
	e.crearEmpleado(t, "E001", "Juan")
	casos := map[string]string{
		"JSON inválido":          `{no`,
		"JSON null":              `null`,
		"cuerpo vacío":           `{}`,
		"campo replicado":        `{"email":"hack@x.com"}`,
		"campo desconocido":      `{"color":"azul"}`,
		"no es texto":            `{"telefono":12345}`,
		"demasiado largo":        `{"telefono":"` + strings.Repeat("9", 31) + `"}`,
		"mezcla válido+inválido": `{"ciudad":"Cali","nombre":"Otro"}`,
	}
	for nombre, cuerpo := range casos {
		if status, _, _ := e.peticion(t, "PUT", "/perfiles/E001", cuerpo); status != 400 {
			t.Errorf("%s: status=%d, se esperaba 400", nombre, status)
		}
	}
	// Nada de lo rechazado debe haberse aplicado
	_, p, _ := e.peticion(t, "GET", "/perfiles/E001", "")
	if p["ciudad"] != "" || p["email"] != "e001@empresa.com" {
		t.Errorf("una petición rechazada modificó el perfil: %v", p)
	}
}

func TestActualizarInexistenteYArchivado(t *testing.T) {
	e := nuevo(t)
	if status, _, _ := e.peticion(t, "PUT", "/perfiles/E404", `{"ciudad":"Cali"}`); status != 404 {
		t.Errorf("inexistente: status=%d", status)
	}
	e.crearEmpleado(t, "E001", "Juan")
	e.retirar(t, "E001")
	status, cuerpo, _ := e.peticion(t, "PUT", "/perfiles/E001", `{"ciudad":"Cali"}`)
	if status != 409 || !strings.Contains(cuerpo["message"].(string), "archivado") {
		t.Errorf("archivado: status=%d cuerpo=%v", status, cuerpo)
	}
	// El perfil archivado sigue siendo consultable
	_, p, _ := e.peticion(t, "GET", "/perfiles/E001", "")
	if p["archivado"] != true || p["fechaArchivado"] == nil {
		t.Errorf("perfil archivado: %v", p)
	}
}

func TestDocumentacionYSalud(t *testing.T) {
	e := nuevo(t)
	res, _ := http.Get(e.srv.URL + "/perfiles/docs")
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(b), "swagger-ui") {
		t.Errorf("swagger: status=%d", res.StatusCode)
	}
	status, spec, _ := e.peticion(t, "GET", "/perfiles/openapi.json", "")
	paths, _ := spec["paths"].(map[string]any)
	if status != 200 || spec["openapi"] == nil || paths["/perfiles"] == nil || paths["/perfiles/{empleadoId}"] == nil {
		t.Errorf("openapi.json inválido: status=%d", status)
	}
	status, h, _ := e.peticion(t, "GET", "/health", "")
	comp := h["components"].(map[string]any)
	if status != 200 || h["status"] != "UP" || comp["db"] != "UP" || comp["broker"] != "UP" {
		t.Errorf("health: status=%d %v", status, h)
	}
	if status, cuerpo, _ := e.peticion(t, "GET", "/otra-cosa", ""); status != 404 || cuerpo["status"] != float64(404) {
		t.Errorf("ruta desconocida: %d %v", status, cuerpo)
	}
}
