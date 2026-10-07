package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/config"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/api"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/aplicacion"
	infra "github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/infra"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/infra/amqp"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/infra/jwt"
	repositorio "github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/infra/repositorio"
)

func main() {
	cfg := config.CargarConfiguracion()

	// 1. Inicializar Base de Datos Postgres (GORM)
	db, err := infra.ConectarBD(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Error al conectar a la base de datos: %v", err)
	}

	// 2. Ejecutar AutoMigrate y el Seed del usuario Admin
	if err := infra.InitDatabase(db); err != nil {
		log.Fatalf("Error al inicializar esquemas/seed de la BD: %v", err)
	}

	// 3. Inicializar Infraestructura y Repositorio Real
	usuarioRepo := repositorio.NuevoUsuarioRepository(db)
	productorAMQP := amqp.NuevoProductor(cfg.AMQPURL, cfg.Exchange)
	tokenProvider := jwt.NuevoTokenProvider(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTExpirationHours)

	// 4. Inicializar Servicio de Aplicación (Inyectando el repositorio)
	authService := aplicacion.NuevoAuthService(productorAMQP, tokenProvider, usuarioRepo)

	// 5. Inicializar Handlers y Rutas
	authHandler := api.NuevoAuthHandler(authService)
	router := api.ConfigurarRutas(authHandler)

	direccion := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Servicio auth-service corriendo en el puerto %s...", cfg.Port)
	if err := http.ListenAndServe(direccion, router); err != nil {
		log.Fatalf("Error al iniciar el servidor: %v", err)
	}
}