package main

import (
	"os"
	"strings"
	"testing"
)

// main conecta el consumidor con amqp.ConfigConsumidorAuth (prefetch 1, para
// conservar el orden de los eventos ante reintentos) y no con una configuración
// armada a mano.
func TestMainUsaLaConfiguracionOrdenadaDelConsumidor(t *testing.T) {
	fuente, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	codigo := string(fuente)
	if !strings.Contains(codigo, "amqp.NuevoConsumidor(amqp.ConfigConsumidorAuth(cfg.AMQPURL, cfg.Exchange), servicio)") {
		t.Fatal("main debe crear el consumidor con amqp.ConfigConsumidorAuth")
	}
	if strings.Contains(codigo, "Prefetch:") {
		t.Fatal("main no debe fijar el prefetch a mano")
	}
}
