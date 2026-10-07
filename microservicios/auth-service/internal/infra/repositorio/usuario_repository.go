package infraestructura

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
	"gorm.io/gorm"
)

type UsuarioRepository struct {
	db *gorm.DB
}

// NuevoUsuarioRepository crea una nueva instancia del repositorio
func NuevoUsuarioRepository(db *gorm.DB) *UsuarioRepository {
	return &UsuarioRepository{db: db}
}

// ObtenerPorEmail busca un usuario en la BD por su correo electrónico.
// Retorna nil si el usuario no existe.
func (r *UsuarioRepository) ObtenerPorEmail(ctx context.Context, email string) (*dominio.Usuario, error) {
	var usuario dominio.Usuario

	err := r.db.WithContext(ctx).
		Where("LOWER(email) = LOWER(?)", email).
		First(&usuario).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // Email no existe
		}
		return nil, err
	}

	return &usuario, nil
}

// ObtenerPorID busca un usuario por su ID
func (r *UsuarioRepository) ObtenerPorID(ctx context.Context, usuarioID string) (*dominio.Usuario, error) {
	var usuario dominio.Usuario

	err := r.db.WithContext(ctx).
		Where("id = ?", usuarioID).
		First(&usuario).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &usuario, nil
}

// ObtenerPorTokenRecuperacion busca el usuario que tenga activo el token de reset
func (r *UsuarioRepository) ObtenerPorTokenRecuperacion(ctx context.Context, token string) (*dominio.Usuario, error) {
	var usuario dominio.Usuario

	err := r.db.WithContext(ctx).
		Where("token_recuperacion = ?", token).
		First(&usuario).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &usuario, nil
}

// ActualizarPasswordYEstado guarda el hash de la clave, actualiza el estado (ej: "ACTIVA") 
// y limpia el token de recuperación consumido.
func (r *UsuarioRepository) GuardarTokenRecuperacion(ctx context.Context, usuarioID, token string) error {
	resultado := r.db.WithContext(ctx).
		Model(&dominio.Usuario{}).
		Where("id = ?", usuarioID).
		Updates(map[string]interface{}{
			"token_recuperacion": token,
			"updated_at":         time.Now().UTC(),
		})

	if resultado.Error != nil {
		return resultado.Error
	}
	if resultado.RowsAffected == 0 {
		return errors.New("no se encontró el usuario para guardar el token")
	}

	return nil
}

func (r *UsuarioRepository) ActualizarPasswordYEstado(ctx context.Context, usuarioID, nuevoHash, nuevoEstado string) error {
	resultado := r.db.WithContext(ctx).
		Model(&dominio.Usuario{}).
		Where("id = ?", usuarioID).
		Updates(map[string]interface{}{
			"password_hash":      nuevoHash,
			"estado":             nuevoEstado,
			"token_recuperacion": sql.NullString{}, // Limpia el token consumido
			"updated_at":         time.Now().UTC(),
		})

	if resultado.Error != nil {
		return resultado.Error
	}
	if resultado.RowsAffected == 0 {
		return errors.New("no se encontró el usuario para actualizar")
	}

	return nil
}

// ActualizarPassword actualiza únicamente el hash de la contraseña para /auth/change-password
func (r *UsuarioRepository) ActualizarPassword(ctx context.Context, usuarioID, nuevoHash string) error {
	resultado := r.db.WithContext(ctx).
		Model(&dominio.Usuario{}).
		Where("id = ?", usuarioID).
		Updates(map[string]interface{}{
			"password_hash": nuevoHash,
			"updated_at":    time.Now().UTC(),
		})

	if resultado.Error != nil {
		return resultado.Error
	}
	if resultado.RowsAffected == 0 {
		return errors.New("no se encontró el usuario para actualizar")
	}

	return nil
}