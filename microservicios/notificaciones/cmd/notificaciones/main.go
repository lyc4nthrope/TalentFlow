// Servicio de Notificaciones (Reto 4): consume eventos de RabbitMQ, simula el envío
// de notificaciones y expone su historial por REST.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/amqp"
	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/api"
	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/aplicacion"
	"github.com/lyc4nthrope/TalentFlow/microservicios/notificaciones/internal/postgres"
)

func main() {
	// La imagen final (distroless) no tiene shell ni wget: el healthcheck de Docker
	// ejecuta este mismo binario con -healthcheck.
	healthcheck := flag.Bool("healthcheck", false, "consulta /health del servicio local y termina")
	flag.Parse()
	if *healthcheck {
		os.Exit(verificarSalud(env("PORT", "8084")))
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "notificaciones-service")
	if err := ejecutar(logger); err != nil {
		logger.Error("el servicio terminó con error", "error", err)
		os.Exit(1)
	}
}

func ejecutar(logger *slog.Logger) error {
	ctx, detener := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer detener()

	pool, err := pgxpool.New(ctx, urlPostgres())
	if err != nil {
		return fmt.Errorf("configurar conexión a PostgreSQL: %w", err)
	}
	defer pool.Close()

	repositorio := postgres.NuevoRepositorio(pool)
	procesador := aplicacion.NuevoProcesador(
		repositorio,
		aplicacion.CanalConsola{Salida: os.Stdout},
		logger,
		time.Now,
	)
	consumidor := amqp.NuevoConsumidor(amqp.Config{
		URL:             urlRabbitMQ(),
		Cola:            env("RABBITMQ_COLA", "notificaciones.eventos"),
		Etiqueta:        "notificaciones-service",
		Prefetch:        10,
		EsperaReintento: 5 * time.Second,
	}, procesador, logger)

	go consumidor.Ejecutar(ctx)

	servidor := &http.Server{
		Addr:    ":" + env("PORT", "8084"),
		Handler: api.NuevoManejador(repositorio, consumidor, logger),
		// Evita que clientes lentos mantengan conexiones abiertas indefinidamente.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	errServidor := make(chan error, 1)
	go func() {
		logger.Info("escuchando HTTP", "addr", servidor.Addr)
		errServidor <- servidor.ListenAndServe()
	}()

	select {
	case err := <-errServidor:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("servidor HTTP: %w", err)
		}
	case <-ctx.Done():
		logger.Info("señal de terminación recibida; cerrando")
	}

	// Cierre ordenado: termina las peticiones en curso (máx. 10 s). El consumidor se
	// detiene al cancelarse ctx; los mensajes sin confirmar vuelven a la cola.
	ctxCierre, cancelar := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelar()
	return servidor.Shutdown(ctxCierre)
}

// URLs armadas con net/url: usuario y contraseña quedan codificados aunque tengan
// caracteres especiales (@, :, /).
func urlPostgres() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(env("DB_USER", "postgres"), env("DB_PASS", "")),
		Host:   net.JoinHostPort(env("DB_HOST", "localhost"), env("DB_PORT", "5432")),
		Path:   env("DB_NAME", "db_notificaciones"),
	}
	return u.String()
}

func urlRabbitMQ() string {
	u := url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(env("RABBITMQ_USER", "guest"), env("RABBITMQ_PASS", "guest")),
		Host:   net.JoinHostPort(env("RABBITMQ_HOST", "localhost"), env("RABBITMQ_PORT", "5672")),
		Path:   "/",
	}
	return u.String()
}

func verificarSalud(puerto string) int {
	cliente := http.Client{Timeout: 2 * time.Second}
	respuesta, err := cliente.Get("http://127.0.0.1:" + puerto + "/health")
	if err != nil {
		return 1
	}
	defer respuesta.Body.Close()
	if respuesta.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func env(clave, porDefecto string) string {
	if valor, ok := os.LookupEnv(clave); ok && valor != "" {
		return valor
	}
	return porDefecto
}
