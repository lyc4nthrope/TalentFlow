package aplicacion

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/dominio"
)

// repositorioEnMemoria reproduce la deduplicación: un id ya visto no se procesa.
type repositorioEnMemoria struct {
	procesados     map[string]bool
	notificaciones []dominio.Notificacion
	fallo          error
}

func (r *repositorioEnMemoria) RegistrarSiEsNuevo(_ context.Context, id string, n dominio.Notificacion) (dominio.Notificacion, bool, error) {
	if r.fallo != nil {
		return dominio.Notificacion{}, false, r.fallo
	}
	if r.procesados[id] {
		return dominio.Notificacion{}, false, nil
	}
	r.procesados[id] = true
	n.ID = "n-" + id
	r.notificaciones = append(r.notificaciones, n)
	return n, true, nil
}

type canalEspia struct{ enviadas []dominio.Notificacion }

func (c *canalEspia) Enviar(n dominio.Notificacion) { c.enviadas = append(c.enviadas, n) }

func nuevoEntorno(repoFallo error) (*Procesador, *repositorioEnMemoria, *canalEspia) {
	repo := &repositorioEnMemoria{procesados: map[string]bool{}, fallo: repoFallo}
	canal := &canalEspia{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	reloj := func() time.Time { return time.Date(2026, 9, 28, 14, 0, 0, 0, time.UTC) }
	return NuevoProcesador(repo, canal, logger, reloj), repo, canal
}

const eventoCreado = `{"id":"a1","type":"usuario.creado","version":1,"occurredAt":"2026-09-28T14:00:00Z",` +
	`"producer":"auth-service","data":{"empleadoId":"E001","email":"juan@empresa.com","tokenActivacion":"tok-1","expiraEn":"2026-09-29T14:00:00Z"}}`

func TestProcesaUnEventoYEnviaLaNotificacion(t *testing.T) {
	procesador, repo, canal := nuevoEntorno(nil)

	if r := procesador.Procesar(context.Background(), []byte(eventoCreado)); r != Procesado {
		t.Fatalf("resultado = %v, se esperaba Procesado", r)
	}
	if len(repo.notificaciones) != 1 || len(canal.enviadas) != 1 {
		t.Fatalf("se esperaba 1 notificación registrada y 1 enviada: %d / %d", len(repo.notificaciones), len(canal.enviadas))
	}
	// Lo enviado es lo registrado: tipo de la bienvenida y el asunto para el correo.
	if enviada := canal.enviadas[0]; enviada.Tipo != dominio.TipoSeguridad || enviada.Asunto == "" || enviada.EmpleadoID != "E001" {
		t.Fatalf("notificación enviada inesperada: %+v", enviada)
	}
}

func TestElMismoEventoDosVecesProduceUnSoloEfecto(t *testing.T) {
	procesador, repo, canal := nuevoEntorno(nil)

	primero := procesador.Procesar(context.Background(), []byte(eventoCreado))
	segundo := procesador.Procesar(context.Background(), []byte(eventoCreado))

	if primero != Procesado || segundo != Duplicado {
		t.Fatalf("resultados = %v, %v; se esperaba Procesado, Duplicado", primero, segundo)
	}
	if len(repo.notificaciones) != 1 || len(canal.enviadas) != 1 {
		t.Fatalf("el duplicado no debe repetir el efecto: %d registradas, %d enviadas",
			len(repo.notificaciones), len(canal.enviadas))
	}
}

func TestDescartaMensajesQueNuncaPodranProcesarse(t *testing.T) {
	casos := map[string]string{
		"JSON corrupto":     `{roto`,
		"tipo no soportado": `{"id":"b1","type":"empleado.actualizado","version":1,"occurredAt":"2026-09-28T14:00:00Z","producer":"x","data":{}}`,
		"carga incompleta":  `{"id":"b2","type":"usuario.creado","version":1,"occurredAt":"2026-09-28T14:00:00Z","producer":"x","data":{"empleadoId":"E1"}}`,
		// Desde el Reto 5 la bienvenida sale de usuario.creado: empleado.creado ya no notifica.
		"empleado.creado": `{"id":"b3","type":"empleado.creado","version":1,"occurredAt":"2026-09-28T14:00:00Z","producer":"x","data":{"empleadoId":"E1","nombre":"J","apellido":"P","email":"j@x.co"}}`,
	}
	for nombre, cuerpo := range casos {
		t.Run(nombre, func(t *testing.T) {
			procesador, _, canal := nuevoEntorno(nil)
			if r := procesador.Procesar(context.Background(), []byte(cuerpo)); r != Descartado {
				t.Fatalf("resultado = %v, se esperaba Descartado", r)
			}
			if len(canal.enviadas) != 0 {
				t.Fatal("un mensaje descartado no debe enviar notificación")
			}
		})
	}
}

func TestErrorDeBaseDeDatosTransitorioSeReintentaSinEnviar(t *testing.T) {
	procesador, _, canal := nuevoEntorno(errors.New("connection refused"))

	if r := procesador.Procesar(context.Background(), []byte(eventoCreado)); r != ErrorTransitorio {
		t.Fatalf("resultado = %v, se esperaba ErrorTransitorio", r)
	}
	if len(canal.enviadas) != 0 {
		t.Fatal("no se debe enviar una notificación que no quedó registrada")
	}
}

func TestRechazoPermanenteDeLaBaseDeDatosSeDescarta(t *testing.T) {
	procesador, _, _ := nuevoEntorno(ErrRechazoPermanente)

	if r := procesador.Procesar(context.Background(), []byte(eventoCreado)); r != Descartado {
		t.Fatalf("resultado = %v, se esperaba Descartado (evita el reintento infinito)", r)
	}
}

// Lo que queda en el historial (y devuelve la API) lleva el token enmascarado; lo que
// reciben los canales de envío (log y correo) lleva el token completo.
func TestElHistorialNoGuardaElTokenYLaEntregaSi(t *testing.T) {
	procesador, repo, canal := nuevoEntorno(nil)

	if r := procesador.Procesar(context.Background(), []byte(eventoCreado)); r != Procesado {
		t.Fatalf("resultado = %v", r)
	}
	guardada, enviada := repo.notificaciones[0], canal.enviadas[0]
	if strings.Contains(guardada.Mensaje, "tok-1") {
		t.Fatalf("el historial guarda el token completo: %q", guardada.Mensaje)
	}
	if !strings.Contains(enviada.TextoAEntregar(), "tok-1") || !strings.Contains(enviada.LineaDeLog(), "tok-1") {
		t.Fatalf("la entrega debe llevar el token completo: %q", enviada.LineaDeLog())
	}
}
