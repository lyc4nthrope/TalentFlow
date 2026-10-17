// Package config lee la configuración del auth-service desde variables de entorno.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
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
	// Un valor de solo espacios cuenta como ausente.
	if strings.TrimSpace(cfg.JWTSecret) == "" {
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

	// Reto 5, D23: las credenciales no tienen valor por defecto en el código. Se arman desde
	// DB_* / RABBITMQ_* (las inyecta Compose) o llegan completas en DATABASE_URL / AMQP_URL.
	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		usuario, clave, err := credenciales("DB_USER", "DB_PASS", "DATABASE_URL")
		if err != nil {
			return nil, err
		}
		cfg.DatabaseURL = (&url.URL{
			Scheme:   "postgres",
			User:     url.UserPassword(usuario, clave),
			Host:     net.JoinHostPort(obtenerEnv("DB_HOST", "localhost"), obtenerEnv("DB_PORT", "5432")),
			Path:     "/" + obtenerEnv("DB_NAME", "db_auth"),
			RawQuery: "sslmode=disable",
		}).String()
	}
	cfg.AMQPURL = os.Getenv("AMQP_URL")
	if cfg.AMQPURL == "" {
		usuario, clave, err := credenciales("RABBITMQ_USER", "RABBITMQ_PASS", "AMQP_URL")
		if err != nil {
			return nil, err
		}
		cfg.AMQPURL = (&url.URL{
			Scheme: "amqp",
			User:   url.UserPassword(usuario, clave),
			Host:   net.JoinHostPort(obtenerEnv("RABBITMQ_HOST", "localhost"), "5672"),
			Path:   "/",
		}).String()
	}
	return cfg, nil
}

// credenciales exige usuario y contraseña cuando no se entrega la URL completa. Como con
// JWT_SECRET, un valor de solo espacios cuenta como ausente (falla aquí, con un mensaje
// claro, y no después como un error de autenticación de Postgres o RabbitMQ).
func credenciales(claveUsuario, claveClave, claveURL string) (string, string, error) {
	usuario, clave := os.Getenv(claveUsuario), os.Getenv(claveClave)
	if strings.TrimSpace(usuario) == "" || strings.TrimSpace(clave) == "" {
		return "", "", fmt.Errorf("%s y %s son obligatorias (o %s completa)", claveUsuario, claveClave, claveURL)
	}
	return usuario, clave, nil
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
