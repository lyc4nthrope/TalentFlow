package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port               string
	DatabaseURL        string
	AMQPURL            string
	Exchange           string
	JWTSecret          string
	JWTIssuer          string
	JWTExpirationHours int
}

func CargarConfiguracion() *Config {
	horasExp, err := strconv.Atoi(obtenerEnv("JWT_EXPIRATION_HOURS", "24"))
	if err != nil {
		horasExp = 24
	}

	databaseURL := obtenerEnv("DATABASE_URL", "")
	if databaseURL == "" {
		dbHost := obtenerEnv("DB_HOST", "localhost")
		dbPort := obtenerEnv("DB_PORT", "5432")
		dbName := obtenerEnv("DB_NAME", "db_auth")
		dbUser := obtenerEnv("DB_USER", "user_auth")
		dbPass := obtenerEnv("DB_PASS", "pass_auth")
		databaseURL = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable", dbHost, dbUser, dbPass, dbName, dbPort)
	}

	amqpURL := obtenerEnv("AMQP_URL", "")
	if amqpURL == "" {
		rabbitHost := obtenerEnv("RABBITMQ_HOST", "localhost")
		rabbitUser := obtenerEnv("RABBITMQ_USER", "guest")
		rabbitPass := obtenerEnv("RABBITMQ_PASS", "guest")
		amqpURL = fmt.Sprintf("amqp://%s:%s@%s:5672/", rabbitUser, rabbitPass, rabbitHost)
	}

	return &Config{
		Port:               obtenerEnv("PORT", "8086"),
		DatabaseURL:        databaseURL,
		AMQPURL:            amqpURL,
		Exchange:           obtenerEnv("AMQP_EXCHANGE", "talentflow.eventos"),
		JWTSecret:          obtenerEnv("JWT_SECRET", "talentflow_super_secret_2026"),
		JWTIssuer:          obtenerEnv("JWT_ISSUER", "auth-service"),
		JWTExpirationHours: horasExp,
	}
}

func obtenerEnv(clave, valorPorDefecto string) string {
	if valor, existe := os.LookupEnv(clave); existe {
		return valor
	}
	return valorPorDefecto
}