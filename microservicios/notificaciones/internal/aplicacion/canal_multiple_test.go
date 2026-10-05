package aplicacion

import (
	"testing"

	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/dominio"
)

func TestCanalMultipleEnviaPorTodosLosCanales(t *testing.T) {
	primero, segundo := &canalEspia{}, &canalEspia{}
	notificacion := dominio.Notificacion{ID: "n-1", Tipo: dominio.TipoBienvenida, Destinatario: "juan@empresa.com"}

	CanalMultiple{primero, segundo}.Enviar(notificacion)

	for nombre, canal := range map[string]*canalEspia{"primero": primero, "segundo": segundo} {
		if len(canal.enviadas) != 1 || canal.enviadas[0] != notificacion {
			t.Fatalf("el canal %s debía recibir la notificación una vez: %+v", nombre, canal.enviadas)
		}
	}
}
