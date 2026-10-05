package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port              string
	AMQPURL           string
	Exchange          string
	JWTSecret         string
	JWTIssuer         string
	JWTExpirationHours int
}

func CargarConfiguracion() *Config {
	horasExp, err := strconv.Atoi(obtenerEnv("JWT_EXPIRATION_HOURS", "24"))
	if err != nil {
		horasExp = 24
	}

	return &Config{
		Port:               obtenerEnv("PORT", "8081"),
		AMQPURL:            obtenerEnv("AMQP_URL", "amqp://guest:guest@localhost:5672/"),
		Exchange:           obtenerEnv("AMQP_EXCHANGE", "talentflow.eventos"),
		JWTSecret:          obtenerEnv("JWT_SECRET", "super-secreto-llave-desarrollo"),
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