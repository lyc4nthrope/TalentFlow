package dominio

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

var ahora = time.Date(2026, 9, 28, 14, 0, 0, 0, time.UTC)

func TestNotificacionDesdeEvento(t *testing.T) {
	casos := []struct {
		nombre     string
		tipoEvento string
		data       string
		tipo       Tipo
		empleadoID string
		empiezaCon string
		contiene   string
		asunto     string
	}{
		{
			nombre:     "usuario.creado es la bienvenida: SEGURIDAD con el token de activación",
			tipoEvento: "usuario.creado",
			data:       `{"empleadoId":"E001","email":"juan.perez@empresa.com","tokenActivacion":"a3f8b2c9-1e4d-07f6-50a1-823bc79e14d5","expiraEn":"2026-02-10T15:30:00Z"}`,
			tipo:       TipoSeguridad,
			empleadoID: "E001",
			empiezaCon: "Para establecer o recuperar su contraseña use el token ****14d5 (vence 2026-02-10T15:30:00Z).",
			contiene:   "Bienvenido a TalentFlow",
			asunto:     "Bienvenido a TalentFlow: active su cuenta",
		},
		{
			nombre:     "usuario.recuperacion: SEGURIDAD con el token de recuperación y sin empleadoId",
			tipoEvento: "usuario.recuperacion",
			data:       `{"email":"juan.perez@empresa.com","tokenRecuperacion":"9c1d4e7a-2b8f-4a30-b6e5-1f207d9a4c88","expiraEn":"2026-05-14T11:00:00Z"}`,
			tipo:       TipoSeguridad,
			empleadoID: "",
			empiezaCon: "Para establecer o recuperar su contraseña use el token ****4c88 (vence 2026-05-14T11:00:00Z).",
			asunto:     "Recuperación de contraseña",
		},
		{
			nombre:     "cuenta.desactivada por VACACIONES: CUENTA temporal",
			tipoEvento: "cuenta.desactivada",
			data:       `{"empleadoId":"E001","email":"juan.perez@empresa.com","motivo":"VACACIONES","permanente":false}`,
			tipo:       TipoCuenta,
			empleadoID: "E001",
			empiezaCon: "Su cuenta fue desactivada temporalmente por vacaciones.",
			asunto:     "Estado de su cuenta",
		},
		{
			nombre:     "cuenta.desactivada por RETIRO: CUENTA permanente (la despedida)",
			tipoEvento: "cuenta.desactivada",
			data:       `{"empleadoId":"E001","email":"juan.perez@empresa.com","motivo":"RETIRO","permanente":true}`,
			tipo:       TipoCuenta,
			empleadoID: "E001",
			empiezaCon: "Su cuenta fue desactivada de forma permanente por retiro.",
			asunto:     "Estado de su cuenta",
		},
		{
			nombre:     "cuenta.activada por FIN_VACACIONES: bienvenida de regreso",
			tipoEvento: "cuenta.activada",
			data:       `{"empleadoId":"E001","email":"juan.perez@empresa.com","motivo":"FIN_VACACIONES"}`,
			tipo:       TipoCuenta,
			empleadoID: "E001",
			empiezaCon: "Bienvenido de regreso. Su cuenta fue reactivada.",
			asunto:     "Estado de su cuenta",
		},
		{
			nombre:     "cuenta.activada por ACTIVACION_INICIAL",
			tipoEvento: "cuenta.activada",
			data:       `{"empleadoId":"E001","email":"juan.perez@empresa.com","motivo":"ACTIVACION_INICIAL"}`,
			tipo:       TipoCuenta,
			empleadoID: "E001",
			empiezaCon: "Su cuenta fue activada. Ya puede iniciar sesión.",
			asunto:     "Estado de su cuenta",
		},
		{
			nombre:     "vacaciones.programadas: VACACIONES con las fechas (Reto 4)",
			tipoEvento: "vacaciones.programadas",
			data:       `{"vacacionesId":"V-2026-0042","empleadoId":"E001","email":"juan.perez@empresa.com","fechaInicio":"2026-12-15T08:00:00-05:00","fechaFin":"2026-12-30T18:00:00-05:00","diasHabiles":12}`,
			tipo:       TipoVacaciones,
			empleadoID: "E001",
			empiezaCon: "Sus vacaciones del 2026-12-15T08:00:00-05:00 al 2026-12-30T18:00:00-05:00 (12 días hábiles) quedaron programadas.",
			asunto:     "Vacaciones programadas",
		},
		{
			nombre:     "vacaciones.iniciadas: VACACIONES con el inicio y el fin del período",
			tipoEvento: "vacaciones.iniciadas",
			data:       `{"vacacionesId":"V-2026-0042","empleadoId":"E001","email":"juan.perez@empresa.com","fechaInicio":"2026-12-15T08:00:00-05:00","fechaFin":"2026-12-30T18:00:00-05:00"}`,
			tipo:       TipoVacaciones,
			empleadoID: "E001",
			empiezaCon: "Sus vacaciones V-2026-0042 comenzaron (2026-12-15T08:00:00-05:00) y terminan el 2026-12-30T18:00:00-05:00.",
			asunto:     "Inicio de sus vacaciones",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			n, err := NotificacionDesdeEvento(c.tipoEvento, json.RawMessage(c.data), ahora)
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if n.Tipo != c.tipo {
				t.Fatalf("tipo = %s, se esperaba %s", n.Tipo, c.tipo)
			}
			if n.Destinatario != "juan.perez@empresa.com" {
				t.Fatalf("destinatario = %q", n.Destinatario)
			}
			if n.EmpleadoID != c.empleadoID {
				t.Fatalf("empleadoId = %q, se esperaba %q", n.EmpleadoID, c.empleadoID)
			}
			if !strings.HasPrefix(n.Mensaje, c.empiezaCon) {
				t.Fatalf("mensaje %q no empieza con %q", n.Mensaje, c.empiezaCon)
			}
			if !strings.Contains(n.Mensaje, c.contiene) {
				t.Fatalf("mensaje %q no contiene %q", n.Mensaje, c.contiene)
			}
			if n.Asunto != c.asunto {
				t.Fatalf("asunto = %q, se esperaba %q", n.Asunto, c.asunto)
			}
			if !n.FechaEnvio.Equal(ahora) {
				t.Fatalf("fechaEnvio = %v, se esperaba %v", n.FechaEnvio, ahora)
			}
		})
	}
}

// El token vigente permite tomar la cuenta: el mensaje que se guarda y devuelve la API
// lo lleva enmascarado; solo la línea [NOTIFICACIÓN] y el correo llevan el completo.
func TestElTokenSoloViajaEnLaEntrega(t *testing.T) {
	casos := map[string]struct{ data, token, visible string }{
		"usuario.creado": {`{"empleadoId":"E001","email":"a@b.co","tokenActivacion":"a3f8b2c9-1e4d-07f6-50a1-823bc79e14d5","expiraEn":"2026-02-10T15:30:00Z"}`,
			"a3f8b2c9-1e4d-07f6-50a1-823bc79e14d5", "****14d5"},
		"usuario.recuperacion": {`{"email":"a@b.co","tokenRecuperacion":"9c1d4e7a-2b8f-4a30-b6e5-1f207d9a4c88","expiraEn":"2026-05-14T11:00:00Z"}`,
			"9c1d4e7a-2b8f-4a30-b6e5-1f207d9a4c88", "****4c88"},
	}
	for evento, c := range casos {
		t.Run(evento, func(t *testing.T) {
			n, err := NotificacionDesdeEvento(evento, json.RawMessage(c.data), ahora)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(n.Mensaje, c.token) || !strings.Contains(n.Mensaje, c.visible) {
				t.Fatalf("el mensaje persistido debe llevar el token enmascarado (%s): %q", c.visible, n.Mensaje)
			}
			api, _ := json.Marshal(n)
			if strings.Contains(string(api), c.token) {
				t.Fatalf("el JSON de la API expone el token: %s", api)
			}
			if !strings.Contains(n.TextoAEntregar(), c.token) || !strings.Contains(n.LineaDeLog(), c.token) {
				t.Fatalf("la entrega debe llevar el token completo: %q / %q", n.TextoAEntregar(), n.LineaDeLog())
			}
			if !strings.HasPrefix(n.TextoAEntregar(), "Para establecer o recuperar su contraseña use el token "+c.token) {
				t.Fatalf("texto de entrega inesperado: %q", n.TextoAEntregar())
			}
		})
	}
}

func TestEnmascarar(t *testing.T) {
	for token, esperado := range map[string]string{"abcd1234": "****1234", "abcd": "****", "": "****"} {
		if obtenido := enmascarar(token); obtenido != esperado {
			t.Errorf("enmascarar(%q) = %q, se esperaba %q", token, obtenido, esperado)
		}
	}
}

// Sin texto de entrega propio, los canales entregan el mensaje.
func TestTextoAEntregarSinMensajeEntrega(t *testing.T) {
	n := Notificacion{Tipo: TipoCuenta, Destinatario: "a@b.co", Mensaje: "Bienvenido de regreso. Su cuenta fue reactivada."}
	if n.TextoAEntregar() != n.Mensaje || !strings.Contains(n.LineaDeLog(), n.Mensaje) {
		t.Fatalf("entrega inesperada: %q", n.LineaDeLog())
	}
}

// Los mensajes de seguridad empiezan exactamente como el ejemplo del reto5.pdf.
func TestLosMensajesDeSeguridadEmpiezanComoElPDF(t *testing.T) {
	for _, evento := range []struct{ tipo, data string }{
		{"usuario.creado", `{"empleadoId":"E001","email":"a@b.co","tokenActivacion":"t","expiraEn":"x"}`},
		{"usuario.recuperacion", `{"email":"a@b.co","tokenRecuperacion":"t","expiraEn":"x"}`},
	} {
		n, err := NotificacionDesdeEvento(evento.tipo, json.RawMessage(evento.data), ahora)
		if err != nil {
			t.Fatalf("%s: %v", evento.tipo, err)
		}
		if !strings.HasPrefix(n.Mensaje, "Para establecer o re") {
			t.Errorf("%s: el mensaje %q no empieza como el PDF", evento.tipo, n.Mensaje)
		}
	}
}

// Desde el Reto 5 la bienvenida y la despedida salen de los eventos del auth-service;
// vacaciones.finalizadas no se consume (la bienvenida de regreso sale de cuenta.activada).
func TestEventosQueYaNoGeneranNotificacion(t *testing.T) {
	for _, tipo := range []string{
		"empleado.creado", "empleado.retirado", "empleado.actualizado", "vacaciones.finalizadas",
		"auth.password_reset_requested", "auth.password_changed",
	} {
		data := `{"empleadoId":"E001","nombre":"Juan","apellido":"Pérez","email":"juan@empresa.com","token":"t","usuarioId":"U1"}`
		if _, err := NotificacionDesdeEvento(tipo, json.RawMessage(data), ahora); !errors.Is(err, ErrTipoNoSoportado) {
			t.Errorf("%s: se esperaba ErrTipoNoSoportado, se obtuvo %v", tipo, err)
		}
	}
}

func TestCargasInvalidasSeRechazan(t *testing.T) {
	casos := map[string]struct{ tipo, data, menciona string }{
		"usuario.creado sin token": {"usuario.creado",
			`{"empleadoId":"E001","email":"a@b.co","expiraEn":"x"}`, "tokenActivacion"},
		"usuario.creado con el campo viejo token": {"usuario.creado",
			`{"empleadoId":"E001","email":"a@b.co","token":"t","expiraEn":"x"}`, "tokenActivacion"},
		"usuario.creado sin expiraEn": {"usuario.creado",
			`{"empleadoId":"E001","email":"a@b.co","tokenActivacion":"t"}`, "expiraEn"},
		"usuario.recuperacion sin email": {"usuario.recuperacion",
			`{"tokenRecuperacion":"t","expiraEn":"x"}`, "email"},
		"usuario.recuperacion sin token": {"usuario.recuperacion",
			`{"email":"a@b.co","expiraEn":"x"}`, "tokenRecuperacion"},
		"cuenta.activada con motivo desconocido": {"cuenta.activada",
			`{"empleadoId":"E001","email":"a@b.co","motivo":"OTRO"}`, "motivo"},
		"cuenta.activada sin motivo": {"cuenta.activada",
			`{"empleadoId":"E001","email":"a@b.co"}`, "motivo"},
		"cuenta.desactivada con motivo desconocido": {"cuenta.desactivada",
			`{"empleadoId":"E001","email":"a@b.co","motivo":"OTRO","permanente":true}`, "motivo"},
		"cuenta.desactivada sin permanente": {"cuenta.desactivada",
			`{"empleadoId":"E001","email":"a@b.co","motivo":"RETIRO"}`, "permanente"},
		"cuenta.desactivada RETIRO no permanente": {"cuenta.desactivada",
			`{"empleadoId":"E001","email":"a@b.co","motivo":"RETIRO","permanente":false}`, "permanente"},
		"cuenta.desactivada VACACIONES permanente": {"cuenta.desactivada",
			`{"empleadoId":"E001","email":"a@b.co","motivo":"VACACIONES","permanente":true}`, "permanente"},
		"cuenta.desactivada sin email": {"cuenta.desactivada",
			`{"empleadoId":"E001","motivo":"RETIRO","permanente":true}`, "email"},
		"vacaciones.iniciadas sin vacacionesId": {"vacaciones.iniciadas",
			`{"empleadoId":"E001","email":"a@b.co","fechaInicio":"i","fechaFin":"f"}`, "vacacionesId"},
		"vacaciones.programadas sin email": {"vacaciones.programadas",
			`{"empleadoId":"E001","fechaInicio":"i","fechaFin":"f","diasHabiles":1}`, "email"},
		"JSON corrupto": {"usuario.creado", `{"empleadoId":`, ""},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			_, err := NotificacionDesdeEvento(c.tipo, json.RawMessage(c.data), ahora)
			if !errors.Is(err, ErrDatosInvalidos) {
				t.Fatalf("se esperaba ErrDatosInvalidos, se obtuvo %v", err)
			}
			if !strings.Contains(err.Error(), c.menciona) {
				t.Fatalf("el error %q debe mencionar %q", err, c.menciona)
			}
		})
	}
}

func TestLosCamposFaltantesSeListanEnOrden(t *testing.T) {
	_, err := NotificacionDesdeEvento("usuario.creado", json.RawMessage(`{"empleadoId":"E001"}`), ahora)
	if !strings.Contains(err.Error(), "[email expiraEn tokenActivacion]") {
		t.Fatalf("los campos faltantes deben listarse en orden: %v", err)
	}
}

func TestLineaDeLogTieneElFormatoDelReto(t *testing.T) {
	n := Notificacion{Tipo: TipoSeguridad, Destinatario: "juan@empresa.com", Mensaje: "Hola"}
	esperada := `[NOTIFICACIÓN] Tipo: SEGURIDAD | Para: juan@empresa.com | Mensaje: "Hola"`
	if got := n.LineaDeLog(); got != esperada {
		t.Fatalf("línea = %s", got)
	}
}

// Sin empleadoId (usuario.recuperacion) el campo se omite en el JSON; el asunto del
// correo nunca se expone.
func TestJSONDeLaNotificacion(t *testing.T) {
	sin, _ := json.Marshal(Notificacion{ID: "1", Tipo: TipoSeguridad, Asunto: "x", MensajeEntrega: "secreto"})
	if strings.Contains(string(sin), "empleadoId") || strings.Contains(string(sin), "x\"") || strings.Contains(string(sin), "secreto") {
		t.Fatalf("JSON sin empleadoId inesperado: %s", sin)
	}
	con, _ := json.Marshal(Notificacion{ID: "1", Tipo: TipoCuenta, EmpleadoID: "E001"})
	if !strings.Contains(string(con), `"empleadoId":"E001"`) {
		t.Fatalf("JSON con empleadoId inesperado: %s", con)
	}
}

// La cola notificaciones.eventos de la topología debe estar ligada exactamente a los
// eventos que este servicio sabe procesar: ni uno de más (llegaría y se descartaría)
// ni uno de menos (nunca llegaría).
func TestLaTopologiaLigaExactamenteLosEventosConsumidos(t *testing.T) {
	contenido, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "infra", "rabbitmq", "definitions.json"))
	if err != nil {
		t.Fatalf("leer definitions.json: %v", err)
	}
	var def struct {
		Bindings []struct {
			Destination string `json:"destination"`
			RoutingKey  string `json:"routing_key"`
		} `json:"bindings"`
	}
	if err := json.Unmarshal(contenido, &def); err != nil {
		t.Fatalf("definitions.json inválido: %v", err)
	}
	var ligados []string
	for _, b := range def.Bindings {
		if b.Destination == "notificaciones.eventos" {
			ligados = append(ligados, b.RoutingKey)
		}
	}
	esperados := append([]string(nil), EventosConsumidos...)
	sort.Strings(ligados)
	sort.Strings(esperados)
	if strings.Join(ligados, ",") != strings.Join(esperados, ",") {
		t.Fatalf("bindings de notificaciones.eventos = %v, el código consume %v", ligados, esperados)
	}
	// Cada evento ligado se procesa (no cae en ErrTipoNoSoportado).
	for _, tipo := range EventosConsumidos {
		if _, err := NotificacionDesdeEvento(tipo, json.RawMessage(`{}`), ahora); errors.Is(err, ErrTipoNoSoportado) {
			t.Errorf("%s está en EventosConsumidos pero no se procesa", tipo)
		}
	}
}
