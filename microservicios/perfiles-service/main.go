// Servicio de Gestión de Perfiles (Go).
//
// Combina los dos estilos: consume empleado.creado / actualizado / retirado para crear,
// sincronizar y archivar perfiles, y expone REST para consultarlos y editarlos.
package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"talentflow/perfiles-service/internal/api"
	"talentflow/perfiles-service/internal/broker"
	"talentflow/perfiles-service/internal/config"
	"talentflow/perfiles-service/internal/consumidor"
	"talentflow/perfiles-service/internal/store"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := config.Cargar()

	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		log.Error("no se pudo abrir la base de datos", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	almacen := store.New(db)
	handler := &consumidor.Consumidor{Store: almacen, Log: log}

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	cons := &broker.Consumidor{
		URL: cfg.BrokerURL, Exchange: cfg.Exchange, Cola: cfg.Queue,
		Eventos: consumidor.EventosConsumidos, Manejador: handler.Manejar, Log: log,
	}
	// En segundo plano: la API responde aunque el broker aún no esté listo.
	go cons.Run(ctx)

	servidor := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           (&api.API{Store: almacen, BrokerConectado: cons.Conectado, Log: log}).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		cierre, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelar()
		_ = servidor.Shutdown(cierre)
	}()

	log.Info("perfiles-service escuchando", "puerto", cfg.Port)
	if err := servidor.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("servidor detenido", "error", err)
		os.Exit(1)
	}
}
