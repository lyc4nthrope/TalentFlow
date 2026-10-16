package main

import (
	"net/http"
	"os"
	"time"
)

// timeoutChequeo acota el healthcheck: menor que el timeout de Docker (3 s).
const timeoutChequeo = 2 * time.Second

// urlSaludLocal apunta al /health del propio proceso (mismo PORT que el servidor).
func urlSaludLocal() string {
	puerto := os.Getenv("PORT")
	if puerto == "" {
		puerto = "8086"
	}
	return "http://127.0.0.1:" + puerto + "/health"
}

// comprobarSalud es verdadero solo si GET url responde 200 a tiempo.
func comprobarSalud(url string, cliente *http.Client) bool {
	respuesta, err := cliente.Get(url)
	if err != nil {
		return false
	}
	defer respuesta.Body.Close()
	return respuesta.StatusCode == http.StatusOK
}

func codigoSalida(sano bool) int {
	if sano {
		return 0
	}
	return 1
}
