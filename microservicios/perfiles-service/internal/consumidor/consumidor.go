// Package consumidor aplica los eventos de empleados sobre los perfiles.
package consumidor

import (
	"context"
	"database/sql"
	"log/slog"

	"talentflow/perfiles-service/internal/eventos"
	"talentflow/perfiles-service/internal/store"
)

var EventosConsumidos = []string{eventos.EmpleadoCreado, eventos.EmpleadoActualizado, eventos.EmpleadoRetirado}

type Consumidor struct {
	Store *store.Store
	Log   *slog.Logger
}

// Manejar procesa un mensaje. Devuelve "procesado", "duplicado" o "ignorado".
// Un error que envuelve eventos.ErrInvalido significa "descartar"; cualquier otro, "reintentar".
func (c *Consumidor) Manejar(ctx context.Context, cuerpo []byte) (string, error) {
	env, err := eventos.ParsearEnvelope(cuerpo)
	if err != nil {
		return "", err
	}

	var efecto func(tx *sql.Tx) error
	switch env.Type {
	case eventos.EmpleadoCreado:
		datos, err := env.DatosEmpleado()
		if err != nil {
			return "", err
		}
		efecto = func(tx *sql.Tx) error { return store.CrearPerfilTx(ctx, tx, datos) }

	case eventos.EmpleadoActualizado:
		datos, err := env.DatosEmpleado()
		if err != nil {
			return "", err
		}
		efecto = func(tx *sql.Tx) error { return store.SincronizarTx(ctx, tx, datos) }

	case eventos.EmpleadoRetirado:
		empleadoID, err := env.EmpleadoIDRetirado()
		if err != nil {
			return "", err
		}
		efecto = func(tx *sql.Tx) error {
			n, err := store.ArchivarTx(ctx, tx, empleadoID)
			if err == nil && n == 0 {
				c.Log.Warn("empleado.retirado sin perfil que archivar", "empleadoId", empleadoID, "eventoId", env.ID)
			}
			return err
		}

	default:
		return "ignorado", nil
	}

	duplicado, err := c.Store.ProcesarEvento(ctx, env.ID, efecto)
	if err != nil {
		return "", err
	}
	if duplicado {
		c.Log.Info("evento duplicado descartado", "type", env.Type, "eventoId", env.ID)
		return "duplicado", nil
	}
	return "procesado", nil
}
