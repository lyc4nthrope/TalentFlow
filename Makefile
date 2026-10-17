# Comandos únicos de las pruebas de TalentFlow (Reto 6, Punto 6).
#
# Cada objetivo termina con el código de salida de la herramienta que ejecuta: si una
# prueba falla, `make` termina con un código distinto de cero (lo que hará fallar el
# pipeline del Reto 7). Los reportes quedan en las rutas de la tabla del README (D9).
#
#   make pruebas        pruebas unitarias e integración de cada microservicio, en orden;
#                       se detiene en la primera que falla
#   make e2e            levanta el sistema con el override de pruebas, espera a que esté
#                       OK y ejecuta la suite BDD en un contenedor (no requiere Node)
#   make e2e-local      lo mismo, pero la suite corre con el Node de la máquina
#   make e2e-restaurar  vuelve vacaciones-service al cron por defecto (cada minuto)
#
# Ningún objetivo borra volúmenes ni datos: `up` solo recrea los contenedores cuya
# configuración o imagen cambió.

.DEFAULT_GOAL := ayuda
.PHONY: ayuda pruebas pruebas-empleados pruebas-gateway pruebas-auth \
        pruebas-notificaciones pruebas-perfiles pruebas-vacaciones pruebas-soporte-e2e \
        sistema-e2e e2e e2e-local e2e-restaurar

# El override docker-compose.e2e.yml pone el scheduler de vacaciones cada 5 s. Sin él,
# el ciclo de vacaciones de la suite falla con E2E_SCHEDULER_SEGUNDOS=5 (D6). bdd-tests está
# en el perfil "e2e": `up` nunca lo levanta y `run bdd-tests` lo activa al nombrarlo.
COMPOSE     := docker compose
COMPOSE_E2E := docker compose -f docker-compose.yml -f docker-compose.e2e.yml

# El contenedor de la suite corre con el uid/gid de quien ejecuta make, así los
# reportes montados en e2e-tests/reports quedan de ese usuario y no de root.
export E2E_UID := $(shell id -u)
export E2E_GID := $(shell id -g)

ayuda:
	@echo "Objetivos:"
	@echo "  make pruebas        unitarias + integración de cada microservicio (para en la primera falla)"
	@echo "  make e2e            sistema con docker-compose.e2e.yml + suite BDD en un contenedor"
	@echo "  make e2e-local      sistema con docker-compose.e2e.yml + suite BDD con el Node local"
	@echo "  make e2e-restaurar  vacaciones-service de vuelta al cron por defecto"

# --- Pruebas de cada microservicio (Punto 1) -------------------------------------------

pruebas: pruebas-empleados pruebas-gateway pruebas-auth pruebas-notificaciones \
         pruebas-perfiles pruebas-vacaciones pruebas-soporte-e2e
	@echo "Todas las pruebas de los microservicios pasaron."

# Node: las dependencias vienen de los workspaces de la raíz (npm ci en la raíz).
pruebas-empleados:
	cd microservicios/gestion-empleados && npm test

pruebas-gateway:
	cd microservicios/api-gateway && npm test

# Go y Python: cada Makefile vacía reports/ y exige el 100 % de la lógica de negocio.
pruebas-auth:
	$(MAKE) -C microservicios/auth-service test

pruebas-notificaciones:
	$(MAKE) -C microservicios/notificaciones test

pruebas-perfiles:
	$(MAKE) -C microservicios/gestion-perfiles test

# Java: `verify` corre las unitarias (Surefire) y las de integración contra PostgreSQL
# con Testcontainers (Failsafe), así que necesita Docker.
pruebas-vacaciones:
	cd microservicios/gestion-vacaciones && mvn -q verify

# Pruebas unitarias del soporte de la suite BDD (polling, fechas, tokens): no
# necesitan el sistema levantado.
pruebas-soporte-e2e:
	cd e2e-tests && { test -d node_modules || npm ci; } && npm run test:unidad

# --- Suite BDD (Puntos 2 a 5) ------------------------------------------------------------

# Levanta (o actualiza) todo el sistema con el override de pruebas. Si vacaciones-service
# corría con el cron por defecto, se recrea solo ese contenedor.
sistema-e2e:
	$(COMPOSE_E2E) up -d --build

# La espera es polling con tope (scripts/esperar-sistema.js), no un sleep fijo. La carpeta
# de reportes se crea antes: si la creara Docker al montar el volumen, sería de root.
e2e: sistema-e2e
	mkdir -p e2e-tests/reports
	$(COMPOSE_E2E) run --rm --build bdd-tests npm run --silent esperar-sistema
	$(COMPOSE_E2E) run --rm bdd-tests

e2e-local: sistema-e2e
	cd e2e-tests && { test -d node_modules || npm ci; } && npm run --silent esperar-sistema && \
		E2E_SCHEDULER_SEGUNDOS=5 npm test

e2e-restaurar:
	$(COMPOSE) up -d vacaciones-service
