package aplicacion

import (
	"fmt"
	"io"

	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/dominio"
)

// CanalConsola simula el envío escribiendo la línea que pide el reto4.pdf:
// [NOTIFICACIÓN] Tipo: BIENVENIDA | Para: juan@empresa.com | Mensaje: "..."
type CanalConsola struct {
	Salida io.Writer
}

func (c CanalConsola) Enviar(n dominio.Notificacion) {
	fmt.Fprintln(c.Salida, n.LineaDeLog())
}
