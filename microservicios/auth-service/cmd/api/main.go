package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/config"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/api"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/aplicacion"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/infra/amqp"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/infra/jwt"
)

func main() {
	cfg := config.CargarConfiguracion()

	// Inicializar Infraestructura
	productorAMQP := amqp.NuevoProductor(cfg.AMQPURL, cfg.Exchange)
	tokenProvider := jwt.NuevoTokenProvider(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTExpirationHours)

	// Inicializar Servicio de Aplicación
	authService := aplicacion.NuevoAuthService(productorAMQP, tokenProvider)

	// Inicializar Handlers y Rutas
	authHandler := api.NuevoAuthHandler(authService)
	router := api.ConfigurarRutas(authHandler)

	direccion := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Servicio auth-service corriendo en el puerto %s...", cfg.Port)
	if err := http.ListenAndServe(direccion, router); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}