#!/usr/bin/env bash
# Reto 4 — demostración reproducible de la sección 6 ("Pruebas del Sistema") de reto4.pdf.
#
# Requisito: el sistema recién levantado desde la raíz del repo, con volúmenes vacíos:
#   docker compose down -v && docker compose up --build -d
# Uso (desde la raíz del repo):   bash docs/reto-04/demo.sh
#
# Desviaciones deliberadas respecto al PDF (cada una explicada donde ocurre):
#   - Paso 7/8: las fechas del PDF (junio de 2026) ya pasaron y serían rechazadas por la
#     validación "fechas en el pasado". Se usan las mismas fechas un año después
#     (junio de 2027); además se muestra el cuerpo literal del PDF recibiendo ese 400.
#   - Paso 9: la publicación "desde la UI del broker" se hace con la API de administración
#     de RabbitMQ (POST /api/exchanges/.../publish), que es exactamente lo que ejecuta el
#     botón "Publish message" de la UI. La UI queda en http://localhost:15672.
set -u

G="${BASE_URL:-http://localhost:8080}"
UI="${RABBITMQ_UI:-http://localhost:15672}"
MQ_AUTH="${RABBITMQ_USER:-admin}:${RABBITMQ_PASS:-admin}"
H='Content-Type: application/json'

titulo() { printf '\n=== %s ===\n' "$1"; }
json() { python3 -c "import json,sys;print(json.dumps(json.load(sys.stdin),ensure_ascii=False,indent=2))"; }
campos() { # imprime solo algunos campos de un objeto JSON: campos k1 k2 ...
  python3 -c "import json,sys;d=json.load(sys.stdin);print('  ',{k:d.get(k) for k in sys.argv[1:]})" "$@"
}

if curl -s -o /dev/null -w '%{http_code}' "$G/empleados/E001" | grep -q 200; then
  echo "AVISO: E001 ya existe. Esta demo espera volúmenes vacíos: docker compose down -v && docker compose up --build -d"
  exit 1
fi

titulo "Paso 1 — Servicios levantados"
docker compose ps --format 'table {{.Service}}\t{{.Status}}\t{{.Ports}}'

titulo "Paso 2 — Broker activo y accesible (UI de administración)"
printf 'GET %s (UI) -> HTTP %s\n' "$UI" "$(curl -s -o /dev/null -w '%{http_code}' "$UI")"
curl -s -u "$MQ_AUTH" "$UI/api/overview" | python3 -c "import json,sys;d=json.load(sys.stdin);print('RabbitMQ',d['rabbitmq_version'],'| nodo',d['node'])"
echo "Colas y a qué eventos están suscritas:"
curl -s -u "$MQ_AUTH" "$UI/api/bindings" | python3 -c "
import json,sys,collections
m=collections.defaultdict(list)
for b in json.load(sys.stdin):
    if b['source']=='talentflow.eventos': m[b['destination']].append(b['routing_key'])
for q,ks in sorted(m.items()): print('  ',q,'<-',', '.join(sorted(ks)))"

titulo "Paso 3 — Crear un departamento"
curl -s -o /dev/null -w 'POST /departamentos -> HTTP %{http_code}\n' -X POST "$G/departamentos" -H "$H" \
  -d '{"id": "IT", "nombre": "Tecnología", "descripcion": "Departamento de TI"}'

titulo "Paso 4 — Crear un empleado (publica empleado.creado)"
curl -s -o /dev/null -w 'POST /empleados -> HTTP %{http_code}\n' -X POST "$G/empleados" -H "$H" \
  -d '{"id": "E001", "nombre": "Juan", "apellido": "Pérez",
       "email": "juan.perez@empresa.com", "numeroEmpleado": "EMP-2026-001",
       "cargo": "Desarrollador Senior", "area": "Tecnología",
       "departamentoId": "IT", "fechaIngreso": "2026-03-01"}'
sleep 2 # la reacción es asincrónica

titulo "Paso 5 — El evento fue procesado por los consumidores (fan-out)"
echo "Perfil creado automáticamente (GET /perfiles/E001):"
curl -s "$G/perfiles/E001" | campos empleadoId nombre email telefono ciudad biografia archivado
echo "Notificación registrada (GET /notificaciones/E001):"
curl -s "$G/notificaciones/E001" | python3 -c "import json,sys;[print('  ',n['tipo'],'->',n['destinatario']) for n in json.load(sys.stdin)]"
echo "Notificación simulada en el log de notificaciones-service:"
docker compose logs --no-log-prefix notificaciones-service 2>/dev/null | grep '^\[NOTIFICACI' | sed 's/^/   /'

titulo "Paso 6 — Actualizar el perfil del empleado"
curl -s -X PUT "$G/perfiles/E001" -H "$H" \
  -d '{"telefono": "3001234567", "ciudad": "Armenia", "biografia": "Ingeniero de sistemas"}' \
  | campos empleadoId nombre telefono ciudad biografia

titulo "Paso 7 — Programar vacaciones y verificar la confirmación asincrónica"
echo "Cuerpo LITERAL del PDF (junio de 2026 ya pasó -> validación 'fechas en el pasado'):"
curl -s -X POST "$G/vacaciones" -H "$H" -d '{"empleadoId": "E001", "fechaInicio": "2026-06-15", "fechaFin": "2026-06-30"}' \
  | campos status message
echo "Mismas fechas, un año después:"
curl -s -i -X POST "$G/vacaciones" -H "$H" -d '{"empleadoId": "E001", "fechaInicio": "2027-06-15", "fechaFin": "2027-06-30"}' \
  | tr -d '\r' | grep -iE '^HTTP|^location|^\{' | sed 's/^/   /'
sleep 2
echo "Notificación de tipo VACACIONES (GET /notificaciones/E001):"
curl -s "$G/notificaciones/E001" | python3 -c "import json,sys;[print('  ',n['tipo'],'|',n['mensaje']) for n in json.load(sys.stdin)]"

titulo "Paso 8 — Validaciones de vacaciones (todas 400)"
validar() {
  curl -s -X POST "$G/vacaciones" -H "$H" -d "$2" | python3 -c "
import json,sys
d=json.load(sys.stdin); c=d.get('periodoEnConflicto')
print(f\"   {sys.argv[1]:<22} -> {d['status']} {d['message']}\")
if c: print(f\"      periodoEnConflicto: {c['id']} ({c['fechaInicio']} a {c['fechaFin']}, {c['estado']})\")" "$1"
}
validar "Fechas incoherentes"  '{"empleadoId": "E001", "fechaInicio": "2027-06-30", "fechaFin": "2027-06-15"}'
validar "Fechas en el pasado"  '{"empleadoId": "E001", "fechaInicio": "2026-06-15", "fechaFin": "2026-06-30"}'
validar "Solapamiento"         '{"empleadoId": "E001", "fechaInicio": "2027-06-20", "fechaFin": "2027-07-05"}'
validar "Empleado inexistente" '{"empleadoId": "NO-EXISTE", "fechaInicio": "2027-08-01", "fechaFin": "2027-08-15"}'

titulo "Paso 9 — Deduplicación: el mismo empleado.creado publicado DOS veces (mismo id)"
MENSAJE='{"id":"3f9b2c10-8e4a-4d6b-9f21-7c0a5d8e1234","type":"empleado.creado","version":1,"occurredAt":"2026-03-01T14:32:05Z","producer":"empleados-service","data":{"empleadoId":"E900","nombre":"Laura","apellido":"Gil","email":"laura.gil@empresa.com","numeroEmpleado":"EMP-2026-900","cargo":"QA","area":"Tecnología","departamentoId":"IT","fechaIngreso":"2026-03-01","estado":"ACTIVO"}}'
for intento in 1 2; do
  python3 -c "import json,sys;print(json.dumps({'properties':{'delivery_mode':2,'content_type':'application/json'},'routing_key':'empleado.creado','payload':sys.argv[1],'payload_encoding':'string'}))" "$MENSAJE" \
    | curl -s -u "$MQ_AUTH" "$UI/api/exchanges/%2F/talentflow.eventos/publish" -H "$H" -d @- \
    | python3 -c "import json,sys;print('   publicación $intento ->', json.load(sys.stdin))"
done
sleep 3
echo "   notificaciones de E900: $(curl -s "$G/notificaciones/E900" | python3 -c 'import json,sys;print(len(json.load(sys.stdin)))')"
echo "   perfiles de E900:       $(curl -s "$G/perfiles" | python3 -c 'import json,sys;print(sum(p["empleadoId"]=="E900" for p in json.load(sys.stdin)))')"
echo "   logs (cada consumidor lo procesó una vez y descartó el duplicado):"
for servicio in notificaciones-service perfiles-service vacaciones-service; do
  docker compose logs --no-log-prefix "$servicio" 2>/dev/null | grep '3f9b2c10' | grep -oE 'duplicado descartado|evento duplicado descartado' | head -1 \
    | sed "s/^/      $servicio: /"
done

titulo "Paso 10 — Retirar un empleado (baja lógica)"
curl -s -X DELETE "$G/empleados/E001" | campos id estado fechaRetiro motivoRetiro
sleep 2
echo "El empleado NO desaparece (GET /empleados/E001):"
curl -s "$G/empleados/E001" | campos id estado fechaRetiro
echo "Auditoría (GET /empleados?estado=RETIRADO):"
curl -s "$G/empleados?estado=RETIRADO" | python3 -c "import json,sys;[print('  ',e['id'],e['estado'],e['fechaRetiro']) for e in json.load(sys.stdin)]"
HOY=$(TZ=America/Bogota date +%F)
echo "Auditoría por rango (desde=$HOY&hasta=$HOY):"
curl -s "$G/empleados?estado=RETIRADO&desde=$HOY&hasta=$HOY" | python3 -c "import json,sys;print('  ',[e['id'] for e in json.load(sys.stdin)])"
echo "Notificación de desvinculación:"
curl -s "$G/notificaciones/E001" | python3 -c "import json,sys;[print('  ',n['tipo']) for n in json.load(sys.stdin)]"
echo "Perfil archivado:"
curl -s "$G/perfiles/E001" | campos empleadoId archivado fechaArchivado

titulo "Paso 11 — Reiniciar los contenedores y verificar que los datos persisten"
resumen() {
  printf '   empleado E001: %s | perfil archivado: %s | notificaciones E001: %s | vacaciones E001: %s\n' \
    "$(curl -s "$G/empleados/E001" | python3 -c 'import json,sys;print(json.load(sys.stdin).get("estado"))')" \
    "$(curl -s "$G/perfiles/E001" | python3 -c 'import json,sys;print(json.load(sys.stdin).get("archivado"))')" \
    "$(curl -s "$G/notificaciones/E001" | python3 -c 'import json,sys;print(len(json.load(sys.stdin)))')" \
    "$(curl -s "$G/vacaciones?empleadoId=E001" | python3 -c 'import json,sys;print(len(json.load(sys.stdin)))')"
}
echo "Antes:"; resumen
docker compose down >/dev/null 2>&1 # SIN -v: los volúmenes (y los datos) se conservan
docker compose up -d >/dev/null 2>&1
for _ in $(seq 1 90); do
  [ "$(docker compose ps --format '{{.Status}}' | grep -c '(healthy)')" -ge 12 ] && break
  sleep 3
done
echo "Después de docker compose down + up:"; resumen

titulo "Estado del sistema (GET /health)"
curl -s "$G/health" | python3 -c "import json,sys;d=json.load(sys.stdin);print('   sistema:',d['sistema'],'| problemas:',d['problemas'])"
