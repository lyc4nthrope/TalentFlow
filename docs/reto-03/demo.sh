#!/usr/bin/env bash
# Reto 3 — demostración reproducible de la sección 3 ("Pruebas del Sistema") de reto3.pdf.
#
# Requisito: el sistema levantado desde la raíz del repo con `docker compose up --build -d`.
# Uso (desde la raíz del repo):   bash docs/reto-03/demo.sh
#
# Diferencia deliberada con el guion literal del PDF: los POST /empleados envían el modelo
# canónico completo. El PDF usa {"id","nombre","departamentoId"}, pero desde el Reto 1 el
# servicio exige todos los campos obligatorios y respondería 400 al instante (validación
# local, antes de llamar a departamentos), sin ejercitar nunca el Circuit Breaker.
set -u

BASE_URL="${BASE_URL:-http://localhost:8080}"
RUN="$(date +%H%M%S)"   # sufijo por corrida: los datos persisten en volúmenes y email/numeroEmpleado son únicos

titulo() { printf '\n=== %s ===\n' "$1"; }

empleado() { # $1=id $2=departamentoId
  printf '{"id":"%s","nombre":"Test","apellido":"Reto3","email":"%s@reto3.test","numeroEmpleado":"N%s","cargo":"Dev","area":"IT","departamentoId":"%s","fechaIngreso":"2026-01-01"}' \
    "$1" "$1" "$1" "$2"
}

estado_sistema() {
  printf 'GET /health -> '
  curl -s "$BASE_URL/health"
  echo
}

estado_circuito() {
  printf 'Estado del circuito: '
  curl -s "$BASE_URL/empleados/circuito-departamentos"
  echo
}

esperar_departamentos_healthy() {
  for _ in $(seq 1 30); do
    [ "$(docker inspect -f '{{.State.Health.Status}}' ms-departamentos 2>/dev/null)" = "healthy" ] && return 0
    sleep 2
  done
  echo "ERROR: departamentos-service no quedó healthy" >&2
  exit 1
}

titulo "Contenedores (solo api-gateway publica puerto)"
docker compose ps --format 'table {{.Service}}\t{{.Status}}\t{{.Ports}}'

titulo "3.1 Punto de entrada único"
for url in "$BASE_URL/departamentos" "$BASE_URL/empleados" "$BASE_URL/health"; do
  printf 'GET %-40s -> HTTP %s\n' "$url" "$(curl -s -o /dev/null -w '%{http_code}' "$url")"
done
for url in "http://localhost:8081/departamentos" "http://localhost:8082/empleados"; do
  if curl -s -o /dev/null --max-time 3 "$url"; then
    printf 'GET %-40s -> RESPONDIÓ (ERROR: no debería ser accesible)\n' "$url"
  else
    printf 'GET %-40s -> conexión rechazada (curl exit %s)\n' "$url" "$?"
  fi
done
estado_sistema

titulo "3.2 Manejo de errores del Gateway"
docker compose stop departamentos-service >/dev/null 2>&1
echo "departamentos-service detenido"
curl -s -i "$BASE_URL/departamentos" | grep -iE '^HTTP|^content-type|^\{'
estado_sistema
docker compose start departamentos-service >/dev/null 2>&1
esperar_departamentos_healthy
echo "departamentos-service restaurado (healthy)"

titulo "3.3 Circuit Breaker — paso 1: crear departamento con el sistema sano"
curl -s -o /dev/null -w 'POST /departamentos {"id":"IT"} -> HTTP %{http_code} (201 = creado; 400 = ya existía de una corrida anterior)\n' \
  -X POST "$BASE_URL/departamentos" -H 'Content-Type: application/json' -d '{"id": "IT", "nombre": "Tecnología"}'
estado_circuito

titulo "3.3 Circuit Breaker — paso 2: detener departamentos-service"
docker compose stop departamentos-service >/dev/null 2>&1
echo "departamentos-service detenido"

titulo "3.3 Circuit Breaker — paso 3: registrar 8 empleados y observar el tiempo"
for i in 1 2 3 4 5 6 7 8; do
  curl -s -o /dev/null -w "Petición $i -> HTTP %{http_code} | %{time_total}s\n" \
    -X POST "$BASE_URL/empleados" -H 'Content-Type: application/json' -d "$(empleado "R${RUN}0$i" IT)"
done
estado_circuito
echo "Extra (reconciliación): un pendiente con un departamento que NO existe, registrado con el circuito abierto"
curl -s -o /dev/null -w "Petición R${RUN}BAD (departamentoId NO-EXISTE) -> HTTP %{http_code} | %{time_total}s\n" \
  -X POST "$BASE_URL/empleados" -H 'Content-Type: application/json' -d "$(empleado "R${RUN}BAD" NO-EXISTE)"
printf 'GET /empleados/R%s01 -> ' "$RUN"
curl -s "$BASE_URL/empleados/R${RUN}01" | grep -o '"validacionDepartamento":"[A-Z]*"'

titulo "3.3 Circuit Breaker — paso 4: restaurar el servicio y esperar el timeout del circuito"
docker compose start departamentos-service >/dev/null 2>&1
esperar_departamentos_healthy
echo "departamentos-service restaurado; sleep 35 (resetTimeout = 30s)"
sleep 35
estado_circuito

titulo "3.3 Circuit Breaker — paso 5: recuperación automática (departamento inexistente)"
curl -s -w '\n-> HTTP %{http_code}\n' \
  -X POST "$BASE_URL/empleados" -H 'Content-Type: application/json' -d "$(empleado "R${RUN}100" NO-EXISTE)"
estado_circuito

titulo "Reconciliación automática de los pendientes (sin reiniciar nada)"
sleep 2
for id in "R${RUN}01" "R${RUN}08" "R${RUN}BAD"; do
  printf 'GET /empleados/%-14s -> ' "$id"
  curl -s "$BASE_URL/empleados/$id" | grep -o '"departamentoId":"[A-Z-]*"\|"validacionDepartamento":"[A-Z]*"' | tr '\n' ' '
  echo
done
estado_sistema
echo "Log de empleados-service:"
docker compose logs --no-log-prefix empleados-service 2>/dev/null \
  | grep -E 'ABIERTO|HALF-OPEN|CERRADO|Reconciliados' | tail -4 | cut -c1-80
