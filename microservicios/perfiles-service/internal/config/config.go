// Package config lee la configuración desde variables de entorno.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port        string
	DBHost      string
	DBPort      string
	DBName      string
	DBUser      string
	DBPass      string
	DatabaseURL string // opcional: sobreescribe los DB_* (útil en pruebas)
	BrokerURL   string
	Exchange    string
	Queue       string
}

func env(clave, porDefecto string) string {
	if v := os.Getenv(clave); v != "" {
		return v
	}
	return porDefecto
}

func Cargar() Config {
	return Config{
		Port:        env("PORT", "8083"),
		DBHost:      env("DB_HOST", "localhost"),
		DBPort:      env("DB_PORT", "5432"),
		DBName:      env("DB_NAME", "db_perfiles"),
		DBUser:      env("DB_USER", "user_perfiles"),
		DBPass:      env("DB_PASS", "pass_perfiles"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		BrokerURL:   env("BROKER_URL", "amqp://talentflow:talentflow_dev@message-broker:5672/"),
		Exchange:    env("BROKER_EXCHANGE", "talentflow.events"),
		Queue:       env("BROKER_QUEUE", "perfiles-service.q"),
	}
}

func (c Config) DSN() string {
	if c.DatabaseURL != "" {
		return c.DatabaseURL
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBUser, c.DBPass, c.DBName)
}
