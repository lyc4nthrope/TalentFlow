#!/usr/bin/env bash
# Demo del flujo asincrónico del Reto 4 (perfiles, notificaciones, vacaciones).
#
# Mientras empleados-service no publique eventos, este script hace de "empleados-service":
# publica empleado.creado / empleado.retirado en el broker usando la API HTTP de RabbitMQ
# (la misma que usa la UI de administración). Si empleados-service ya publica, cambie
# SIMULAR_EMPLEADOS=0 y el script creará el empleado por REST (POST /empleados).
#
# Requisitos: sistema levantado (docker compose up --build -d), curl y python3.
# Uso: bash docs/reto-04/demo.sh
set -u

BASE="${BASE:-http://localhost:8080}"
MQ="${MQ:-http://localhost:15672}"
MQ_USER="${RABBITMQ_USER:-talentflow}"
MQ_PASS="${RABBITMQ_PASS:-talentflow_dev}"
EXCHANGE="talentflow.events"
SIMULAR_EMPLEADOS="${SIMULAR_EMPLEADOS:-1}"
ANIO=$(( $(date +%Y) + 1 ))     # fechas FUTURAS: el servicio rechaza fechas pasadas
EMP="E001"; EMAIL="juan.perez@empresa.com"

paso() { printf '\n\033[1m== %s\033[0m\n' "$*"; }
get()  { curl -s -w '\n[HTTP %{http_code}]\n' "$@"; }

# publicar <routing_key> <message_id> <json-data>
publicar() {
  python3 - "$1" "$2" "$3" "$MQ" "$EXCHANGE" "$MQ_USER" "$MQ_PASS" <<'PY'
import sys, json, base64, urllib.request, datetime
clave, mid, data, mq, ex, u, p = sys.argv[1:8]
env = {"id": mid, "type": clave, "version": 1,
       "occurredAt": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
       "producer": "empleados-service", "data": json.loads(data)}
body = json.dumps({"properties": {"message_id": mid, "content_type": "application/json", "delivery_mode": 2},
                   "routing_key": clave, "payload": json.dumps(env), "payload_encoding": "string"}).encode()
req = urllib.request.Request(f"{mq}/api/exchanges/%2F/{ex}/publish", body,
      {"Content-Type": "application/json",
       "Authorization": "Basic " + base64.b64encode(f"{u}:{p}".encode()).decode()})
r = json.load(urllib.request.urlopen(req))
print(f"  publicado {clave} id={mid} -> enrutado={r.get('routed')}")
PY
}

paso "0. Salud del sistema"
get "$BASE/health"

paso "1. Crear el empleado $EMP (genera empleado.creado)"
if [ "$SIMULAR_EMPLEADOS" = "1" ]; then
  publicar empleado.creado "msg-creado-$EMP" \
    "{\"empleadoId\":\"$EMP\",\"nombre\":\"Juan\",\"apellido\":\"Pérez\",\"email\":\"$EMAIL\",\"numeroEmpleado\":\"EMP-2026-001\",\"cargo\":\"Desarrollador Senior\",\"area\":\"Tecnología\",\"departamentoId\":\"IT\",\"fechaIngreso\":\"2026-03-01\",\"estado\":\"ACTIVO\"}"
else
  curl -s -X POST "$BASE/departamentos" -H 'Content-Type: application/json' -d '{"id":"IT","nombre":"Tecnología","descripcion":"Departamento de TI"}' >/dev/null
  get -X POST "$BASE/empleados" -H 'Content-Type: application/json' -d "{\"id\":\"$EMP\",\"nombre\":\"Juan\",\"apellido\":\"Pérez\",\"email\":\"$EMAIL\",\"numeroEmpleado\":\"EMP-2026-001\",\"cargo\":\"Desarrollador Senior\",\"area\":\"Tecnología\",\"departamentoId\":\"IT\",\"fechaIngreso\":\"2026-03-01\"}"
fi
sleep 2

paso "2. Perfil creado automáticamente + notificación de bienvenida"
get "$BASE/perfiles/$EMP"
get "$BASE/notificaciones/$EMP"

paso "3. Actualizar el perfil (PUT)"
get -X PUT "$BASE/perfiles/$EMP" -H 'Content-Type: application/json' \
  -d '{"telefono":"3001234567","ciudad":"Armenia","biografia":"Ingeniero de sistemas"}'

paso "4. Programar vacaciones (publica vacaciones.programadas)"
get -X POST "$BASE/vacaciones" -H 'Content-Type: application/json' \
  -d "{\"empleadoId\":\"$EMP\",\"fechaInicio\":\"$ANIO-06-15\",\"fechaFin\":\"$ANIO-06-30\"}"
sleep 2
echo "-- notificaciones (debe aparecer una de tipo VACACIONES):"
get "$BASE/notificaciones/$EMP"

paso "5. Validaciones de vacaciones (todas deben responder 400)"
echo "-- fechas incoherentes";   get -X POST "$BASE/vacaciones" -H 'Content-Type: application/json' -d "{\"empleadoId\":\"$EMP\",\"fechaInicio\":\"$ANIO-06-30\",\"fechaFin\":\"$ANIO-06-15\"}"
echo "-- fechas en el pasado";   get -X POST "$BASE/vacaciones" -H 'Content-Type: application/json' -d "{\"empleadoId\":\"$EMP\",\"fechaInicio\":\"2020-01-10\",\"fechaFin\":\"2020-01-20\"}"
echo "-- solapamiento";          get -X POST "$BASE/vacaciones" -H 'Content-Type: application/json' -d "{\"empleadoId\":\"$EMP\",\"fechaInicio\":\"$ANIO-06-20\",\"fechaFin\":\"$ANIO-07-05\"}"
echo "-- empleado inexistente";  get -X POST "$BASE/vacaciones" -H 'Content-Type: application/json' -d "{\"empleadoId\":\"NO-EXISTE\",\"fechaInicio\":\"$ANIO-08-01\",\"fechaFin\":\"$ANIO-08-15\"}"

paso "6. Deduplicación: el MISMO mensaje (mismo id) dos veces -> un solo efecto"
publicar empleado.creado "msg-dup-1" '{"empleadoId":"E777","nombre":"Dup","apellido":"Prueba","email":"dup@empresa.com","cargo":"QA","area":"TI","departamentoId":"IT"}'
publicar empleado.creado "msg-dup-1" '{"empleadoId":"E777","nombre":"Dup","apellido":"Prueba","email":"dup@empresa.com","cargo":"QA","area":"TI","departamentoId":"IT"}'
sleep 2
echo "-- notificaciones de E777 (esperado: UNA):";  get "$BASE/notificaciones/E777"
echo "-- perfil de E777 (esperado: uno):";          get "$BASE/perfiles/E777"

paso "7. Retirar al empleado $EMP (empleado.retirado)"
if [ "$SIMULAR_EMPLEADOS" = "1" ]; then
  publicar empleado.retirado "msg-retirado-$EMP" \
    "{\"empleadoId\":\"$EMP\",\"email\":\"$EMAIL\",\"fechaRetiro\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\",\"motivo\":\"RENUNCIA\"}"
else
  get -X DELETE "$BASE/empleados/$EMP"
fi
sleep 2
echo "-- perfil archivado (no borrado):";   get "$BASE/perfiles/$EMP"
echo "-- notificación de desvinculación:";  get "$BASE/notificaciones/$EMP"
echo "-- vacaciones de un retirado (debe dar 400):"
get -X POST "$BASE/vacaciones" -H 'Content-Type: application/json' -d "{\"empleadoId\":\"$EMP\",\"fechaInicio\":\"$ANIO-09-01\",\"fechaFin\":\"$ANIO-09-10\"}"

paso "Listo. Swagger UI: $BASE/perfiles/docs  $BASE/notificaciones/docs  $BASE/vacaciones/docs"
echo "Log simulado de notificaciones:  docker compose logs notificaciones-service | grep NOTIFICACI"
