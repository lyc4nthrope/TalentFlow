package aplicacion

import (
	"bytes"
	"testing"

	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/dominio"
)

// El canal de consola es la "simulación" del envío del reto: escribe una línea por
// notificación, con el texto completo que se entrega (token incluido).
func TestCanalConsolaEscribeLaLineaDelRetoConElTextoAEntregar(t *testing.T) {
	var salida bytes.Buffer
	n := dominio.Notificacion{Tipo: dominio.TipoSeguridad, Destinatario: "juan@empresa.com",
		Mensaje: "token ****1234", MensajeEntrega: "token abcd1234"}

	CanalConsola{Salida: &salida}.Enviar(n)

	esperada := `[NOTIFICACIÓN] Tipo: SEGURIDAD | Para: juan@empresa.com | Mensaje: "token abcd1234"` + "\n"
	if salida.String() != esperada {
		t.Fatalf("salida = %q; se esperaba %q", salida.String(), esperada)
	}
}
