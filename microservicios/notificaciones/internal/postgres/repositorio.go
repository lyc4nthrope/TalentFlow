// Package postgres implementa la persistencia del servicio sobre PostgreSQL.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/aplicacion"
	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/dominio"
)

// clasificar distingue errores permanentes (clase SQLSTATE 22: dato inválido; 23:
// restricción violada) de los transitorios (conexión, timeout...), que sí vale la
// pena reintentar.
func clasificar(err error, contexto string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (strings.HasPrefix(pgErr.Code, "22") || strings.HasPrefix(pgErr.Code, "23")) {
		return fmt.Errorf("%s: %w: %v", contexto, aplicacion.ErrRechazoPermanente, err)
	}
	return fmt.Errorf("%s: %w", contexto, err)
}

type Repositorio struct {
	pool *pgxpool.Pool
}

func NuevoRepositorio(pool *pgxpool.Pool) *Repositorio {
	return &Repositorio{pool: pool}
}

// RegistrarSiEsNuevo aplica la deduplicación y el efecto en UNA transacción: o quedan
// registrados el id del evento y la notificación, o no queda ninguno. Si el registro
// del id y la notificación fueran operaciones separadas, una caída entre ambas dejaría
// un evento "procesado" sin notificación (o una notificación que se repetiría).
func (r *Repositorio) RegistrarSiEsNuevo(ctx context.Context, eventoID string, n dominio.Notificacion) (dominio.Notificacion, bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return dominio.Notificacion{}, false, fmt.Errorf("iniciar transacción: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // sin efecto si ya se hizo commit

	// ON CONFLICT DO NOTHING: si el id ya existe no inserta y no falla. Con dos
	// transacciones concurrentes sobre el mismo id, la segunda espera a la primera y
	// luego no inserta: nunca hay doble efecto.
	resultado, err := tx.Exec(ctx,
		`INSERT INTO eventos_procesados (id) VALUES ($1) ON CONFLICT (id) DO NOTHING`, eventoID)
	if err != nil {
		return dominio.Notificacion{}, false, clasificar(err, "registrar evento procesado")
	}
	if resultado.RowsAffected() == 0 {
		return dominio.Notificacion{}, false, nil // duplicado
	}

	err = tx.QueryRow(ctx,
		`INSERT INTO notificaciones (tipo, destinatario, mensaje, fecha_envio, empleado_id, evento_id)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		n.Tipo, n.Destinatario, n.Mensaje, n.FechaEnvio, n.EmpleadoID, eventoID,
	).Scan(&n.ID)
	if err != nil {
		return dominio.Notificacion{}, false, clasificar(err, "guardar notificación")
	}

	if err := tx.Commit(ctx); err != nil {
		return dominio.Notificacion{}, false, fmt.Errorf("confirmar transacción: %w", err)
	}
	return n, true, nil
}

func (r *Repositorio) Listar(ctx context.Context) ([]dominio.Notificacion, error) {
	return r.consultar(ctx, `SELECT id, tipo, destinatario, mensaje, fecha_envio, empleado_id
	                           FROM notificaciones ORDER BY fecha_envio, id`)
}

func (r *Repositorio) ListarPorEmpleado(ctx context.Context, empleadoID string) ([]dominio.Notificacion, error) {
	return r.consultar(ctx, `SELECT id, tipo, destinatario, mensaje, fecha_envio, empleado_id
	                           FROM notificaciones WHERE empleado_id = $1 ORDER BY fecha_envio, id`, empleadoID)
}

func (r *Repositorio) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

func (r *Repositorio) consultar(ctx context.Context, sql string, args ...any) ([]dominio.Notificacion, error) {
	filas, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("consultar notificaciones: %w", err)
	}
	notificaciones, err := pgx.CollectRows(filas, func(fila pgx.CollectableRow) (dominio.Notificacion, error) {
		var n dominio.Notificacion
		err := fila.Scan(&n.ID, &n.Tipo, &n.Destinatario, &n.Mensaje, &n.FechaEnvio, &n.EmpleadoID)
		n.FechaEnvio = n.FechaEnvio.UTC()
		return n, err
	})
	if err != nil {
		return nil, fmt.Errorf("leer notificaciones: %w", err)
	}
	return notificaciones, nil
}
