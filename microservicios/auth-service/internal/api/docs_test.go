package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func pedirDocs(t *testing.T, ruta string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	ConfigurarRutas(NuevoAuthHandler(&casosFalsos{}), nil).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, ruta, nil))
	return rec
}

func TestSwaggerUIBajoElPrefijoDelServicio(t *testing.T) {
	rec := pedirDocs(t, "/auth/docs")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "swagger-ui") ||
		!strings.Contains(rec.Body.String(), `url: "/auth/openapi.json"`) {
		t.Fatalf("docs inesperado: %d", rec.Code)
	}
}

// D13: bearerAuth es global y solo las tres operaciones que dan o recuperan el
// acceso lo anulan con security: [].
func TestEspecificacionDeclaraBearerYOperacionesPublicas(t *testing.T) {
	rec := pedirDocs(t, "/auth/openapi.json")
	if rec.Code != http.StatusOK || !json.Valid(rec.Body.Bytes()) {
		t.Fatalf("openapi.json inesperado: %d", rec.Code)
	}

	type operacion struct {
		Security *[]map[string][]string `json:"security"`
	}
	var doc struct {
		Security   []map[string][]string           `json:"security"`
		Paths      map[string]map[string]operacion `json:"paths"`
		Components struct {
			SecuritySchemes map[string]map[string]string `json:"securitySchemes"`
		} `json:"components"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	esquema := doc.Components.SecuritySchemes["bearerAuth"]
	if esquema["type"] != "http" || esquema["scheme"] != "bearer" || esquema["bearerFormat"] != "JWT" {
		t.Fatalf("esquema bearerAuth inesperado: %v", esquema)
	}
	if len(doc.Security) != 1 {
		t.Fatalf("se esperaba seguridad global bearerAuth: %v", doc.Security)
	}
	if _, ok := doc.Security[0]["bearerAuth"]; !ok {
		t.Fatalf("se esperaba seguridad global bearerAuth: %v", doc.Security)
	}

	for _, ruta := range []string{"/auth/login", "/auth/recover-password", "/auth/reset-password"} {
		s := doc.Paths[ruta]["post"].Security
		if s == nil || len(*s) != 0 {
			t.Errorf("POST %s debe declarar security: [] (pública)", ruta)
		}
	}
	// La operación debe existir: si faltara, Security sería nil y la prueba pasaría sola.
	cambio, existe := doc.Paths["/auth/change-password"]["post"]
	if !existe {
		t.Fatal("falta POST /auth/change-password en la especificación")
	}
	if cambio.Security != nil {
		t.Errorf("POST /auth/change-password debe heredar bearerAuth")
	}
	if _, documentado := doc.Paths["/health"]; documentado {
		t.Errorf("/health no se documenta: el Gateway responde su propio /health")
	}
}
