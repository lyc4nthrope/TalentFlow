// Package store implementa la persistencia en PostgreSQL.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"talentflow/perfiles-service/internal/dominio"
)

const columnas = `id, empleado_id, nombre, apellido, email, cargo, area, departamento_id,
	telefono, direccion, ciudad, biografia, archivado, fecha_archivado, fecha_creacion, fecha_actualizacion`

// CamposEditables mapea el nombre JSON del campo a su columna (lista blanca: nunca se
// interpola texto del cliente en el SQL).
var CamposEditables = map[string]string{
	"telefono":  "telefono",
	"direccion": "direccion",
	"ciudad":    "ciudad",
	"biografia": "biografia",
}

var ordenEditables = []string{"telefono", "direccion", "ciudad", "biografia"}

type Store struct{ db *sql.DB }

func New(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return s.db.PingContext(ctx)
}

type escaner interface{ Scan(dest ...any) error }

func escanear(e escaner) (*dominio.Perfil, error) {
	var p dominio.Perfil
	var archivadoEn sql.NullTime
	var creacion, actualizacion time.Time
	err := e.Scan(&p.ID, &p.EmpleadoID, &p.Nombre, &p.Apellido, &p.Email, &p.Cargo, &p.Area,
		&p.DepartamentoID, &p.Telefono, &p.Direccion, &p.Ciudad, &p.Biografia, &p.Archivado,
		&archivadoEn, &creacion, &actualizacion)
	if err != nil {
		return nil, err
	}
	if archivadoEn.Valid {
		i := dominio.Instante(archivadoEn.Time)
		p.FechaArchivado = &i
	}
	p.FechaCreacion = dominio.Instante(creacion)
	p.FechaActualizacion = dominio.Instante(actualizacion)
	return &p, nil
}

func (s *Store) Obtener(ctx context.Context, empleadoID string) (*dominio.Perfil, error) {
	fila := s.db.QueryRowContext(ctx, `SELECT `+columnas+` FROM perfiles WHERE empleado_id = $1`, empleadoID)
	p, err := escanear(fila)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, dominio.ErrNoEncontrado
	}
	return p, err
}

// Listar devuelve todos los perfiles; si archivado != nil filtra por ese estado.
func (s *Store) Listar(ctx context.Context, archivado *bool) ([]dominio.Perfil, error) {
	consulta := `SELECT ` + columnas + ` FROM perfiles`
	args := []any{}
	if archivado != nil {
		consulta += ` WHERE archivado = $1`
		args = append(args, *archivado)
	}
	consulta += ` ORDER BY empleado_id`
	filas, err := s.db.QueryContext(ctx, consulta, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	perfiles := []dominio.Perfil{}
	for filas.Next() {
		p, err := escanear(filas)
		if err != nil {
			return nil, err
		}
		perfiles = append(perfiles, *p)
	}
	return perfiles, filas.Err()
}

// Actualizar modifica los campos editables recibidos. Un perfil archivado no se edita.
func (s *Store) Actualizar(ctx context.Context, empleadoID string, cambios map[string]string) (*dominio.Perfil, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	var archivado bool
	err = tx.QueryRowContext(ctx, `SELECT archivado FROM perfiles WHERE empleado_id = $1 FOR UPDATE`, empleadoID).Scan(&archivado)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, dominio.ErrNoEncontrado
	}
	if err != nil {
		return nil, err
	}
	if archivado {
		return nil, dominio.ErrArchivado
	}

	sets := []string{}
	args := []any{}
	for _, campo := range ordenEditables {
		if valor, ok := cambios[campo]; ok {
			args = append(args, valor)
			sets = append(sets, fmt.Sprintf("%s = $%d", CamposEditables[campo], len(args)))
		}
	}
	sets = append(sets, "fecha_actualizacion = now()")
	args = append(args, empleadoID)
	consulta := fmt.Sprintf(`UPDATE perfiles SET %s WHERE empleado_id = $%d RETURNING %s`,
		strings.Join(sets, ", "), len(args), columnas)

	p, err := escanear(tx.QueryRowContext(ctx, consulta, args...))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return p, nil
}

// ProcesarEvento ejecuta `efecto` y registra el id del mensaje en UNA transacción.
// Si el id ya estaba registrado devuelve duplicado=true y NO ejecuta el efecto.
// Si el efecto falla, el registro del id se deshace y la reentrega podrá reintentar.
func (s *Store) ProcesarEvento(ctx context.Context, eventoID string, efecto func(tx *sql.Tx) error) (duplicado bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx, `INSERT INTO eventos_procesados (id) VALUES ($1) ON CONFLICT (id) DO NOTHING`, eventoID)
	if err != nil {
		return false, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return true, nil
	}
	if err := efecto(tx); err != nil {
		return false, err
	}
	return false, tx.Commit()
}

// CrearPerfilTx crea el perfil por defecto. Si ya existe uno para ese empleado no hace nada.
func CrearPerfilTx(ctx context.Context, tx *sql.Tx, d dominio.DatosEmpleado) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO perfiles (id, empleado_id, nombre, apellido, email, cargo, area, departamento_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (empleado_id) DO NOTHING`,
		nuevoUUID(), d.EmpleadoID, d.Nombre, d.Apellido, d.Email, d.Cargo, d.Area, d.DepartamentoID)
	return err
}

// SincronizarTx actualiza los campos replicados. Si el perfil aún no existe (el
// empleado.creado no llegó), lo crea con esos datos en lugar de perder la información.
func SincronizarTx(ctx context.Context, tx *sql.Tx, d dominio.DatosEmpleado) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO perfiles (id, empleado_id, nombre, apellido, email, cargo, area, departamento_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (empleado_id) DO UPDATE SET
			nombre = EXCLUDED.nombre, apellido = EXCLUDED.apellido, email = EXCLUDED.email,
			cargo = EXCLUDED.cargo, area = EXCLUDED.area, departamento_id = EXCLUDED.departamento_id,
			fecha_actualizacion = now()`,
		nuevoUUID(), d.EmpleadoID, d.Nombre, d.Apellido, d.Email, d.Cargo, d.Area, d.DepartamentoID)
	return err
}

// ArchivarTx marca el perfil como archivado (nunca lo borra). Devuelve cuántos perfiles afectó.
func ArchivarTx(ctx context.Context, tx *sql.Tx, empleadoID string) (int64, error) {
	res, err := tx.ExecContext(ctx, `
		UPDATE perfiles
		SET archivado = TRUE,
		    fecha_archivado = COALESCE(fecha_archivado, now()),
		    fecha_actualizacion = now()
		WHERE empleado_id = $1`, empleadoID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// nuevoUUID genera un UUID v4 sin dependencias externas.
func nuevoUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
