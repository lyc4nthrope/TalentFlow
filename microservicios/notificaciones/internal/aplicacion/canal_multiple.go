package aplicacion

import "github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/dominio"

// CanalMultiple envía la misma notificación por varios canales, en orden. Permite
// sumar el correo real (bonus: SMTP) sin quitar el log por consola que exige el reto
// y sin que el caso de uso sepa cuántos canales hay.
type CanalMultiple []Canal

func (c CanalMultiple) Enviar(n dominio.Notificacion) {
	for _, canal := range c {
		canal.Enviar(n)
	}
}
