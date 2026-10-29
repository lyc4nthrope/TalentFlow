# Pruebas con reportes (Reto 6, D9, D10 y D14). Uso: make test
# Resultados en reports/junit.xml; perfil completo en reports/coverage.out; perfil de
# la lógica de negocio en reports/coverage-negocio.out y en Cobertura reports/coverage.xml.
# Falla si una prueba falla o si la lógica de negocio no está al 100 % de sentencias.

SHELL := /bin/bash
.SHELLFLAGS := -euo pipefail -c

GOTESTSUM :=gotest.tools/gotestsum@v1.13.0
COBERTURA := github.com/boumenot/gocover-cobertura@v1.4.0
FILTRO    := ../../scripts/cobertura-go.awk

# Lógica de negocio medida (D11 y D14). Fuera: cmd/ y los adaptadores (internal/api,
# internal/amqp, internal/smtp, internal/postgres); ver docs/reto-06/PLAN.md, F4.
NEGOCIO := internal/dominio internal/aplicacion internal/eventos

.PHONY: test
test:
	rm -rf reports && mkdir -p reports
	go run $(GOTESTSUM) --junitfile reports/junit.xml -- \
		-coverprofile=reports/coverage.out -covermode=atomic ./...
	awk -v modulo="$$(go list -m)" -v paquetes="$(NEGOCIO)" -f $(FILTRO) \
		reports/coverage.out > reports/coverage-negocio.out
	go run $(COBERTURA) < reports/coverage-negocio.out > reports/coverage.xml
	go tool cover -func=reports/coverage-negocio.out | grep -v '100.0%' || true
	awk -v modo=exigir -f $(FILTRO) reports/coverage-negocio.out
