package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/config"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/api"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/aplicacion"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/infra/amqp"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/infra/jwt"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/infra/postgres"
	"github.com/lyc4nthrope/TalentFlow/microservicios/auth-service/internal/metricas"
)

// tokenAleatorio genera los tokens de activación y recuperación: UUID v4 a partir
// de crypto/rand (Reto 5, D1).
func tokenAleatorio() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

func main() {
	// Reto 5, D22: la imagen distroless no tiene shell ni wget, así que el healthcheck de
	// Docker ejecuta el propio binario con -salud.
	salud := flag.Bool("salud", false, "consulta GET /health del servicio local y termina (0 = sano)")
	flag.Parse()
	if *salud {
		os.Exit(codigoSalida(comprobarSalud(urlSaludLocal(), &http.Client{Timeout: timeoutChequeo})))
	}

	cfg, err := config.Cargar()
	if err != nil {
		log.Printf("auth-service: configuración inválida: %v", err)
		os.Exit(1)
	}

	ctx, cancelar := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelar()

	pool, err := postgres.Conectar(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Printf("auth-service: %v", err)
		os.Exit(1)
	}
	defer pool.Close()

	emisor, err := jwt.NuevoTokenProvider(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTVigencia, time.Now)
	if err != nil {
		log.Printf("auth-service: %v", err)
		os.Exit(1)
	}
	servicio, err := aplicacion.NuevoAuthService(
		postgres.NuevoRepositorio(pool),
		amqp.NuevoProductor(cfg.AMQPURL, cfg.Exchange),
		emisor,
		tokenAleatorio,
		time.Now,
		aplicacion.Opciones{
			VigenciaActivacion:   cfg.VigenciaActivacion,
			VigenciaRecuperacion: cfg.VigenciaRecuperacion,
		},
	)
	if err != nil {
		log.Printf("auth-service: %v", err)
		os.Exit(1)
	}

	if err := servicio.SembrarAdmin(ctx, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Printf("auth-service: %v", err)
		os.Exit(1)
	}

	// Ciclo de vida de la cuenta: empleado.creado/retirado y vacaciones.iniciadas/finalizadas.
	// Prefetch 1 para conservar el orden ante reintentos (ver amqp.PrefetchOrdenado).
	consumidor := amqp.NuevoConsumidor(amqp.ConfigConsumidorAuth(cfg.AMQPURL, cfg.Exchange), servicio)
	go consumidor.Ejecutar(ctx)

	servidor := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           metricas.Nuevas().Instrumentar(api.ConfigurarRutas(api.NuevoAuthHandler(servicio), api.NuevoSaludHandler(pool, consumidor))),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		apagado, cancelarApagado := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelarApagado()
		_ = servidor.Shutdown(apagado)
	}()

	log.Printf("auth-service escuchando en el puerto %s", cfg.Port)
	if err := servidor.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("auth-service: %v", err)
		os.Exit(1)
	}
}
