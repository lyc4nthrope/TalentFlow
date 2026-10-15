// Package postgres implementa el repositorio de cuentas sobre PostgreSQL (pgx).
package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/aplicacion"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
)

const columnas = `id::text, sujeto, COALESCE(empleado_id, ''), email, COALESCE(password_hash, ''),
	rol, estado, COALESCE(token_hash, ''), COALESCE(token_tipo, ''), token_expira_en,
	COALESCE(vacaciones_suspension, '')`

// clasificar traduce los errores de Postgres: 23505 (único) es una cuenta duplicada;
// el resto de las clases 22 (dato inválido) y 23 (restricción violada) son rechazos
// permanentes, que reintentar no arregla. Lo demás (conexión, timeout) es transitorio.
func clasificar(err error, contexto string) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23505":
			return aplicacion.ErrUsuarioDuplicado
		case strings.HasPrefix(pgErr.Code, "22"), strings.HasPrefix(pgErr.Code, "23"):
			return fmt.Errorf("%s: %w: %v", contexto, aplicacion.ErrRechazoPermanente, err)
		}
	}
	return fmt.Errorf("%s: %w", contexto, err)
}

// consultor es lo común entre el pool y una transacción: el mismo repositorio
// trabaja sobre cualquiera de los dos. Begin sobre una transacción abre un
// SAVEPOINT, lo que permite que un INSERT fallido no aborte la transacción entera.
type consultor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

type Repositorio struct {
	pool *pgxpool.Pool
	q    consultor
	// enTransaccion: las lecturas bloquean la fila (SELECT ... FOR UPDATE), así dos
	// manejadores concurrentes de la misma cuenta (réplicas, Reto 11) se serializan y
	// cada uno decide con el estado que dejó el otro.
	enTransaccion bool
}

func NuevoRepositorio(pool *pgxpool.Pool) *Repositorio {
	return &Repositorio{pool: pool, q: pool}
}

// ProcesarUnaVez registra el id del evento y ejecuta fn con un repositorio ligado a
// la misma transacción (D4). Un id ya registrado → duplicado, sin ejecutar fn. Si fn
// falla se hace rollback: ni el efecto ni el id quedan guardados.
func (r *Repositorio) ProcesarUnaVez(ctx context.Context, eventoID string, fn func(aplicacion.Repositorio) error) (bool, error) {
	tx, err := r.q.Begin(ctx)
	if err != nil {
		return false, clasificar(err, "abrir transacción")
	}
	defer func() { _ = tx.Rollback(ctx) }() // sin efecto si ya hubo commit

	resultado, err := tx.Exec(ctx,
		`INSERT INTO eventos_procesados (id) VALUES ($1) ON CONFLICT (id) DO NOTHING`, eventoID)
	if err != nil {
		return false, clasificar(err, "registrar evento procesado")
	}
	if resultado.RowsAffected() == 0 {
		return true, nil
	}
	if err := fn(&Repositorio{pool: r.pool, q: tx, enTransaccion: true}); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, clasificar(err, "confirmar transacción")
	}
	return false, nil
}

// Conectar abre el pool y verifica la conexión.
func Conectar(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("configurar pool de Postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("conectar a Postgres: %w", err)
	}
	return pool, nil
}

func (r *Repositorio) buscarUno(ctx context.Context, condicion string, valor any) (*dominio.Usuario, error) {
	consulta := "SELECT " + columnas + " FROM usuarios WHERE " + condicion
	if r.enTransaccion {
		consulta += " FOR UPDATE"
	}
	fila := r.q.QueryRow(ctx, consulta, valor)
	var u dominio.Usuario
	var tipo string
	var expira *time.Time
	err := fila.Scan(&u.ID, &u.Sujeto, &u.EmpleadoID, &u.Email, &u.PasswordHash,
		&u.Rol, &u.Estado, &u.TokenHash, &tipo, &expira, &u.VacacionesSuspension)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, clasificar(err, "buscar cuenta")
	}
	u.TokenTipo = dominio.TipoToken(tipo)
	if expira != nil {
		u.TokenExpiraEn = *expira
	}
	return &u, nil
}

func (r *Repositorio) BuscarPorEmail(ctx context.Context, email string) (*dominio.Usuario, error) {
	return r.buscarUno(ctx, "email = lower($1)", email)
}

func (r *Repositorio) BuscarPorEmpleadoID(ctx context.Context, empleadoID string) (*dominio.Usuario, error) {
	return r.buscarUno(ctx, "empleado_id = $1", empleadoID)
}

func (r *Repositorio) BuscarPorSujeto(ctx context.Context, sujeto string) (*dominio.Usuario, error) {
	return r.buscarUno(ctx, "sujeto = $1", sujeto)
}

func (r *Repositorio) BuscarPorTokenHash(ctx context.Context, tokenHash string) (*dominio.Usuario, error) {
	return r.buscarUno(ctx, "token_hash = $1", tokenHash)
}

func vacioANulo(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func (r *Repositorio) Crear(ctx context.Context, u *dominio.Usuario) error {
	var expira any
	if !u.TokenExpiraEn.IsZero() {
		expira = u.TokenExpiraEn
	}
	// SAVEPOINT: si el INSERT choca (23505) dentro de una transacción, solo se
	// deshace él y se puede seguir consultando quién es el dueño del dato.
	sp, err := r.q.Begin(ctx)
	if err != nil {
		return clasificar(err, "crear cuenta")
	}
	defer func() { _ = sp.Rollback(ctx) }()
	err = sp.QueryRow(ctx, `
		INSERT INTO usuarios (sujeto, empleado_id, email, password_hash, rol, estado,
		                      token_hash, token_tipo, token_expira_en)
		VALUES ($1, $2, lower($3), $4, $5, $6, $7, $8, $9)
		RETURNING id::text`,
		u.Sujeto, vacioANulo(u.EmpleadoID), u.Email, vacioANulo(u.PasswordHash), u.Rol, u.Estado,
		vacioANulo(u.TokenHash), vacioANulo(string(u.TokenTipo)), expira,
	).Scan(&u.ID)
	if err != nil {
		return clasificar(err, "crear cuenta")
	}
	return clasificar(sp.Commit(ctx), "crear cuenta")
}

func (r *Repositorio) GuardarToken(ctx context.Context, usuarioID, tokenHash string, tipo dominio.TipoToken, expiraEn time.Time) error {
	resultado, err := r.q.Exec(ctx, `
		UPDATE usuarios SET token_hash = $2, token_tipo = $3, token_expira_en = $4, actualizado_en = now()
		WHERE id = $1`, usuarioID, tokenHash, string(tipo), expiraEn)
	if err != nil {
		return clasificar(err, "guardar token")
	}
	if resultado.RowsAffected() == 0 {
		return fmt.Errorf("cuenta %s no encontrada", usuarioID)
	}
	return nil
}

// ConsumirToken aplica el cambio solo si el token sigue siendo el activo, vigente y
// de un tipo conocido, y el estado no cambió: dos usos simultáneos del mismo token no
// pueden aplicarse ambos. La BD vuelve a comprobar lo que ya validó la aplicación.
func (r *Repositorio) ConsumirToken(ctx context.Context, usuarioID, tokenHash, estadoEsperado, passwordHash, nuevoEstado string) (bool, error) {
	resultado, err := r.q.Exec(ctx, `
		UPDATE usuarios
		SET password_hash = $4, estado = $5,
		    token_hash = NULL, token_tipo = NULL, token_expira_en = NULL, actualizado_en = now()
		WHERE id = $1 AND token_hash = $2 AND estado = $3
		  AND token_tipo IN ('ACTIVACION', 'RECUPERACION') AND token_expira_en > now()`,
		usuarioID, tokenHash, estadoEsperado, passwordHash, nuevoEstado)
	if err != nil {
		return false, clasificar(err, "consumir token")
	}
	return resultado.RowsAffected() == 1, nil
}

// ActualizarPassword cambia la contraseña y anula cualquier token de activación o
// recuperación pendiente: tras un cambio, un enlace de recuperación viejo no sirve.
func (r *Repositorio) ActualizarPassword(ctx context.Context, usuarioID, passwordHash string) error {
	resultado, err := r.q.Exec(ctx, `
		UPDATE usuarios
		SET password_hash = $2, token_hash = NULL, token_tipo = NULL, token_expira_en = NULL, actualizado_en = now()
		WHERE id = $1`, usuarioID, passwordHash)
	if err != nil {
		return clasificar(err, "actualizar contraseña")
	}
	if resultado.RowsAffected() == 0 {
		return fmt.Errorf("cuenta %s no encontrada", usuarioID)
	}
	return nil
}

// actualizarSiCambia ejecuta un UPDATE condicional y devuelve si afectó la fila.
func (r *Repositorio) actualizarSiCambia(ctx context.Context, contexto, sql string, args ...any) (bool, error) {
	resultado, err := r.q.Exec(ctx, sql, args...)
	if err != nil {
		return false, clasificar(err, contexto)
	}
	return resultado.RowsAffected() == 1, nil
}

func (r *Repositorio) Retirar(ctx context.Context, usuarioID string) (bool, error) {
	return r.actualizarSiCambia(ctx, "retirar cuenta", `
		UPDATE usuarios
		SET estado = 'DESACTIVADA_PERMANENTE', vacaciones_suspension = NULL,
		    token_hash = NULL, token_tipo = NULL, token_expira_en = NULL, actualizado_en = now()
		WHERE id = $1 AND estado <> 'DESACTIVADA_PERMANENTE'`, usuarioID)
}

func (r *Repositorio) Suspender(ctx context.Context, usuarioID, vacacionesID string) (bool, error) {
	return r.actualizarSiCambia(ctx, "suspender cuenta", `
		UPDATE usuarios
		SET estado = 'SUSPENDIDA_TEMPORAL', vacaciones_suspension = $2, actualizado_en = now()
		WHERE id = $1 AND estado = 'ACTIVA'`, usuarioID, vacacionesID)
}

func (r *Repositorio) CambiarSuspension(ctx context.Context, usuarioID, vacacionesID string) (bool, error) {
	return r.actualizarSiCambia(ctx, "cambiar suspensión", `
		UPDATE usuarios SET vacaciones_suspension = $2, actualizado_en = now()
		WHERE id = $1 AND estado = 'SUSPENDIDA_TEMPORAL'`, usuarioID, vacacionesID)
}

func (r *Repositorio) Reactivar(ctx context.Context, usuarioID, vacacionesID string) (bool, error) {
	return r.actualizarSiCambia(ctx, "reactivar cuenta", `
		UPDATE usuarios
		SET estado = 'ACTIVA', vacaciones_suspension = NULL, actualizado_en = now()
		WHERE id = $1 AND estado = 'SUSPENDIDA_TEMPORAL' AND vacaciones_suspension = $2`, usuarioID, vacacionesID)
}
