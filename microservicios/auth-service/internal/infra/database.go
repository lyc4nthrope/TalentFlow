package infraestructura

import (
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/dominio"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ConectarBD(databaseURL string) (*gorm.DB, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL no configurada")
	}

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("error conectando a Postgres: %w", err)
	}

	return db, nil
}

func InitDatabase(db *gorm.DB) error {
	if hasTable := db.Migrator().HasTable(&dominio.Usuario{}); !hasTable {
		if err := db.AutoMigrate(&dominio.Usuario{}); err != nil {
			return fmt.Errorf("error ejecutando AutoMigrate: %w", err)
		}
	}

	if err := SeedAdminUser(db); err != nil {
		return fmt.Errorf("error ejecutando seed de admin: %w", err)
	}

	return nil
}

func SeedAdminUser(db *gorm.DB) error {
	adminEmail := "admin@empresa.com"
	adminPass := "Admin123*" // Contraseña que usas en el script curl

	var count int64
	db.Model(&dominio.Usuario{}).Where("email = ?", adminEmail).Count(&count)

	// Si ya existe el admin, no lo volvemos a crear
	if count > 0 {
		log.Println("==> El usuario Admin ya existe en la base de datos.")
		return nil
	}

	// Generar Hash bcrypt real de la contraseña
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("error al hashear la contraseña del admin: %w", err)
	}

	// Crear struct del admin con los campos requeridos
	adminUser := dominio.Usuario{
		ID:           uuid.New().String(),
		Email:        adminEmail,
		PasswordHash: string(hashedPassword),
		Roles:        []string{"ADMIN"},
		Estado:       dominio.EstadoActiva, // "ACTIVA"
	}

	if err := db.Create(&adminUser).Error; err != nil {
		return fmt.Errorf("error insertando el usuario admin: %w", err)
	}

	log.Println("==> Seed completado: Usuario Admin creado exitosamente (admin@empresa.com / Admin123*)")
	return nil
}