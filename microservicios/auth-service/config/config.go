// Package config lee la configuración del auth-service desde variables de entorno.
package config

import (
	"errors"
	"fmt"
	"os"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string
	AMQPURL     string
	Exchange    string

	// Reto 5, D11: obligatorio, sin valor por defecto en el código.
	JWTSecret   string
	JWTIssuer   string
	JWTVigencia time.Duration

	// Reto 5, D1: vigencia de los tokens de activación y recuperación.
	VigenciaActivacion   time.Duration
	VigenciaRecuperacion time.Duration

	// Reto 5, D15: administrador semilla (opcional; sin valores por defecto).
	AdminEmail    string
	AdminPassword string
}

// Cargar devuelve un error si falta algo sin lo cual el servicio no debe arrancar.
func Cargar() (*Config, error) {
	cfg := &Config{
		Port:          obtenerEnv("PORT", "8086"),
		Exchange:      obtenerEnv("AMQP_EXCHANGE", "talentflow.eventos"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		JWTIssuer:     obtenerEnv("JWT_ISSUER", "auth-service"),
		AdminEmail:    os.Getenv("ADMIN_EMAIL"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
	}
	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET es obligatorio")
	}

	var err error
	if cfg.JWTVigencia, err = duracion("JWT_VIGENCIA", "1h"); err != nil {
		return nil, err
	}
	if cfg.VigenciaActivacion, err = duracion("TOKEN_ACTIVACION_VIGENCIA", "24h"); err != nil {
		return nil, err
	}
	if cfg.VigenciaRecuperacion, err = duracion("TOKEN_RECUPERACION_VIGENCIA", "1h"); err != nil {
		return nil, err
	}

	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		cfg.DatabaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			obtenerEnv("DB_USER", "user_auth"), obtenerEnv("DB_PASS", "pass_auth"),
			obtenerEnv("DB_HOST", "localhost"), obtenerEnv("DB_PORT", "5432"), obtenerEnv("DB_NAME", "db_auth"))
	}
	cfg.AMQPURL = os.Getenv("AMQP_URL")
	if cfg.AMQPURL == "" {
		cfg.AMQPURL = fmt.Sprintf("amqp://%s:%s@%s:5672/",
			obtenerEnv("RABBITMQ_USER", "guest"), obtenerEnv("RABBITMQ_PASS", "guest"), obtenerEnv("RABBITMQ_HOST", "localhost"))
	}
	return cfg, nil
}

func duracion(clave, porDefecto string) (time.Duration, error) {
	valor := obtenerEnv(clave, porDefecto)
	d, err := time.ParseDuration(valor)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s debe ser una duración positiva (p. ej. 1h, 30m): %q", clave, valor)
	}
	return d, nil
}

func obtenerEnv(clave, valorPorDefecto string) string {
	if valor, existe := os.LookupEnv(clave); existe && valor != "" {
		return valor
	}
	return valorPorDefecto
}
