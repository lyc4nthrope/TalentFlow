#!/usr/bin/env bash
# Reto 5 — demostración reproducible del "Flujo de prueba sugerido" de reto5.pdf (18 pasos).
#
# Requisito: el sistema levantado desde la raíz del repo (este script NO lo levanta):
#   docker compose up --build -d
# Uso (desde cualquier carpeta):   bash docs/reto-05/demo.sh
# Duración: unos 5 minutos con el scheduler por defecto (VACACIONES_CRON = cada minuto).
# Termina con código 0 solo si todas las verificaciones pasan.
#
# Variables opcionales:
#   BASE_URL (http://localhost:8080)    ADMIN_EMAIL / ADMIN_PASSWORD (admin@empresa.com / Admin123*)
#   MAILHOG_UI (http://localhost:8025)  ID_E001 / ID_E002 (ids de los dos empleados)
#   DURACION_E001 (60) / DURACION_E002 (120): segundos que duran las vacaciones de los pasos 12 y 15
#
# Decisiones y desviaciones respecto al PDF (cada una explicada donde ocurre):
#   - Re-ejecutable sobre una BD con datos: los emails y los números de empleado llevan un sufijo
#     único por corrida. Los ids los elige el cliente (POST /empleados exige "id"): se usan E001 y
#     E002 si están libres; si ya existen (corrida anterior), se usan E001-<sufijo> y E002-<sufijo>.
#     El PDF se refiere a ellos como E001 y E002; la salida muestra el id real.
#   - Paso 2: el endpoint ya está protegido, así que el administrador semilla (ADMIN_EMAIL /
#     ADMIN_PASSWORD, D15) inicia sesión y crea el departamento y los empleados.
#   - Paso 5: también se activa E002 (paso 15: solo una cuenta ACTIVA se suspende por vacaciones).
#   - Pasos 12 a 15: el PDF pide fechaInicio = fechaFin = hoy. Con fecha sola, el fin es las
#     23:59:59 (D6) y la cuenta quedaría suspendida el resto del día. Se usan fecha y hora reales
#     de hoy (America/Bogota): inicio en el próximo minuto exacto (cuando corre el scheduler) y fin
#     DURACION_E001 segundos después. Así el ciclo completo se ve en unos 2 o 3 minutos.
#   - Las comprobaciones "en los logs" leen `docker compose logs` desde el comienzo de la corrida y
#     enlazan cada evento por su id (envelope) entre el productor y el consumidor cuando el log lo
#     permite; el orden del paso 13 se compara con las marcas de tiempo de Docker.
set -uo pipefail

cd "$(dirname "$0")/../.." || exit 1

G="${BASE_URL:-http://localhost:8080}"
MAIL="${MAILHOG_UI:-http://localhost:8025}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@empresa.com}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-Admin123*}"
DURACION_E001="${DURACION_E001:-60}"
DURACION_E002="${DURACION_E002:-120}"
ZONA="America/Bogota"
SUFIJO="$(date +%s)"
INICIO_DEMO="$(date -u +%FT%TZ)"
H='Content-Type: application/json'

# Contraseñas que cumplen la política (D14: 8 a 72 caracteres, mayúscula, minúscula y dígito).
PASS_INICIAL='Inicial123'
PASS_RECUPERADA='Recuperada123'
PASS_NUEVA='Cambiada123'

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# ---------------------------------------------------------------- utilidades
TOTAL_OK=0
TOTAL_FALLAS=0
RESUMEN=()
PASO_ACTUAL=""
PASO_FALLO=0

cerrar_paso() {
  [ -z "$PASO_ACTUAL" ] && return
  if [ "$PASO_FALLO" -eq 0 ]; then RESUMEN+=("PASS  $PASO_ACTUAL"); else RESUMEN+=("FAIL  $PASO_ACTUAL"); fi
}
paso() { cerrar_paso; PASO_ACTUAL="$1"; PASO_FALLO=0; printf '\n=== Paso %s ===\n' "$1"; }
info() { printf '   %s\n' "$*"; }
ok() { TOTAL_OK=$((TOTAL_OK + 1)); printf '   [PASS] %s\n' "$*"; }
falla() { TOTAL_FALLAS=$((TOTAL_FALLAS + 1)); PASO_FALLO=1; printf '   [FAIL] %s\n' "$*"; }
comprobar() { # comprobar "descripción" condición...
  local d="$1"; shift
  if "$@"; then ok "$d"; else falla "$d"; fi
}

resumen_final() {
  cerrar_paso; PASO_ACTUAL=""
  printf '\n=== Resumen ===\n'
  printf '   %s\n' "${RESUMEN[@]}"
  printf '   Verificaciones: %d correctas, %d fallidas | duración: %dm%02ds\n' \
    "$TOTAL_OK" "$TOTAL_FALLAS" $((SECONDS / 60)) $((SECONDS % 60))
  if [ "$TOTAL_FALLAS" -gt 0 ]; then echo "   RESULTADO: FALLÓ"; exit 1; fi
  echo "   RESULTADO: OK"
  exit 0
}
abortar() { falla "$*"; info "No se puede continuar sin este resultado."; resumen_final; }

# pedir MÉTODO RUTA [TOKEN] [CUERPO]: deja el código en $CODIGO y el cuerpo en $TMP/cuerpo.
pedir() {
  local args=(-s -o "$TMP/cuerpo" -w '%{http_code}' -X "$1" "$G$2")
  [ -n "${3:-}" ] && args+=(-H "Authorization: Bearer $3")
  [ -n "${4:-}" ] && args+=(-H "$H" -d "$4")
  CODIGO="$(curl "${args[@]}")" || CODIGO="000"
}
# esperar_codigo "descripción" CÓDIGOS_ESPERADOS (separados por |): verifica $CODIGO.
esperar_codigo() {
  if [[ "$CODIGO" =~ ^($2)$ ]]; then ok "$1 -> HTTP $CODIGO"; else falla "$1 -> HTTP $CODIGO (se esperaba $2) $(head -c 300 "$TMP/cuerpo")"; fi
}
campo() { # campo CLAVE: imprime un campo del último cuerpo JSON
  python3 -c "import json,sys;d=json.load(open(sys.argv[1]));print(d.get(sys.argv[2]) if isinstance(d,dict) else '')" "$TMP/cuerpo" "$1" 2>/dev/null
}
login() { # login EMAIL PASSWORD: deja $CODIGO y, si es 200, el JWT en $TOKEN
  TOKEN=""
  pedir POST /auth/login "" "$(printf '{"email":"%s","password":"%s"}' "$1" "$2")"
  [ "$CODIGO" = 200 ] && TOKEN="$(campo token)"
}
codigo_login() { login "$1" "$2"; echo "$CODIGO"; }
claims() { # imprime sub, role y email de un JWT (sin verificar la firma; solo para mostrarlos)
  python3 -c "
import base64,json,sys
p=sys.argv[1].split('.')[1]; p+='='*(-len(p)%4)
d=json.loads(base64.urlsafe_b64decode(p)); print({k:d.get(k) for k in ('sub','role','email')})" "$1"
}
# esperar SEGUNDOS comando...: reintenta cada 2 s hasta que el comando tenga éxito.
esperar() {
  local limite=$((SECONDS + $1)); shift
  until "$@"; do
    [ "$SECONDS" -ge "$limite" ] && return 1
    sleep 2
  done
}

# Logs de un servicio desde el comienzo de la corrida, con la marca de tiempo de Docker.
logs() { docker compose logs --no-log-prefix --timestamps --since "$INICIO_DEMO" "$1" 2>/dev/null; }
# linea_log SERVICIO PATRÓN_ERE: primera línea que coincide (con su marca de tiempo).
linea_log() { logs "$1" | grep -E -m1 -- "$2"; }
# linea_log_desde SERVICIO DESDE PATRÓN_ERE: igual, pero solo líneas posteriores a DESDE (UTC).
linea_log_desde() { logs "$1" | awk -v d="$2" '$1 >= d' | grep -E -m1 -- "$3"; }
hay_log() { [ -n "$(linea_log "$1" "$2")" ]; }
NOTIF='^[^ ]+ \[NOTIFICACI.{1,2}N\]' # línea [NOTIFICACIÓN] (la Ó ocupa 2 bytes en UTF-8)
mostrar_log() { printf '      %s | %s\n' "$1" "$(printf '%s' "$2" | cut -d' ' -f2- | cut -c1-220)"; }
marca() { printf '%s' "$1" | cut -d' ' -f1; }
antes_o_igual() { # antes_o_igual MARCA1 MARCA2 [TOLERANCIA_S] (RFC 3339 con nanosegundos)
  python3 -c "
import sys,re
from datetime import datetime
def t(s):
    m=re.match(r'(.*T[0-9:]+)(\.\d+)?Z',s); return datetime.fromisoformat(m.group(1)).timestamp()+float(m.group(2) or 0)
sys.exit(0 if t(sys.argv[1])<=t(sys.argv[2])+float(sys.argv[3]) else 1)" "$1" "$2" "${3:-0}"
}
# en_orden PRODUCTOR AUTH NOTIFICACIONES. vacaciones-service escribe "Evento publicado" después
# de que el broker confirma, y auth-service puede consumir el mensaje antes de esa confirmación:
# las dos líneas pueden salir en el mismo milisegundo en cualquier orden. Por eso el primer par
# admite 1 s de tolerancia (la causalidad la prueba el vacacionesId que nombra auth-service, que
# solo viaja en vacaciones.iniciadas). El segundo par es estricto: auth publica cuenta.desactivada
# después de registrar la suspensión.
en_orden() { antes_o_igual "$1" "$2" 1 && antes_o_igual "$2" "$3"; }
# notificacion_registrada TIPO_EVENTO [EMPLEADO_ID] [DESDE]: imprime "marca eventoId" de la
# línea "notificación registrada" de notificaciones-service (log JSON).
notificacion_registrada() {
  logs notificaciones-service | python3 -c "
import json,sys
tipo,emp,desde=sys.argv[1],sys.argv[2],sys.argv[3]
for linea in sys.stdin:
    marca,_,resto=linea.partition(' ')
    if not resto.startswith('{'): continue
    try: d=json.loads(resto)
    except ValueError: continue
    if d.get('msg')=='notificación registrada' and d.get('tipo')==tipo and d.get('empleadoId','')==emp and marca>=desde:
        print(marca,d['eventoId']); break" "$1" "${2:-}" "${3:-}"
}
# Paso 5 del PDF: el token se lee de la línea [NOTIFICACIÓN] del log de notificaciones-service
# (es el único lugar, junto con el correo, donde viaja completo; en la API queda enmascarado).
token_de_log() { # token_de_log EMAIL [TOKEN_A_EXCLUIR]: el último token enviado a ese email
  logs notificaciones-service | grep -F "Para: $1 |" | grep -oE 'use el token [0-9a-f-]{36}' \
    | awk '{print $4}' | grep -vxF "${2:-ninguno}" | tail -1
}
hay_token() { [ -n "$(token_de_log "$@")" ]; }
hay_notificacion() { [ -n "$(notificacion_registrada "$@")" ]; }
login_da() { [ "$(codigo_login "$1" "$2")" = "$3" ]; } # login_da EMAIL PASSWORD CÓDIGO
existe() { [ "$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $ADMIN_TOKEN" "$G$1")" = 200 ]; }
token_en_mailhog() { # token_en_mailhog EMAIL TOKEN: ¿Mailhog recibió un correo con ese token?
  curl -sf "$MAIL/api/v2/search?kind=to&query=$1" 2>/dev/null | python3 -c "
import json,sys,quopri
d=json.load(sys.stdin)
sys.exit(0 if any(sys.argv[1] in quopri.decodestring(m['Content']['Body']).decode('utf-8','replace') for m in d['items']) else 1)" "$2"
}
# Hora local de negocio (America/Bogota) del próximo minuto exacto, a 20 s o más de ahora.
proximo_minuto() { echo $(( ( $(date +%s) + 20 + 59 ) / 60 * 60 )); }
hora_local() { TZ="$ZONA" date -d "@$1" +%FT%T; }
estado_vacaciones() { pedir GET "/vacaciones/$1" "$ADMIN_TOKEN"; campo estado; }
vacaciones_en() { [ "$(estado_vacaciones "$1")" = "$2" ]; }

# ---------------------------------------------------------------- 1
paso "1 — Iniciar todos los servicios"
info "Este script no levanta el sistema. Antes de ejecutarlo: docker compose up --build -d"
pedir GET /health
SISTEMA="$(campo sistema)"
if [ "$CODIGO" = 200 ] && [ "$SISTEMA" = OK ]; then
  ok "GET /health -> HTTP $CODIGO, sistema: $SISTEMA"
else
  abortar "GET /health -> HTTP $CODIGO, sistema: ${SISTEMA:-?} (problemas: $(campo problemas))"
fi
info "Contenedores: $(docker compose ps --format '{{.Status}}' | grep -c '(healthy)') con estado (healthy)"

# ---------------------------------------------------------------- 2
paso "2 — Onboarding de empleados (POST /empleados con el JWT del administrador semilla)"
login "$ADMIN_EMAIL" "$ADMIN_PASSWORD"
esperar_codigo "POST /auth/login del administrador semilla ($ADMIN_EMAIL)" 200
ADMIN_TOKEN="$TOKEN"
[ -n "$ADMIN_TOKEN" ] || abortar "Sin el JWT del administrador no se puede crear a los empleados"
info "claims: $(claims "$ADMIN_TOKEN")"

pedir GET /departamentos/IT "$ADMIN_TOKEN"
if [ "$CODIGO" = 200 ]; then
  ok "Departamento IT ya existe -> HTTP 200 (no se crea de nuevo)"
else
  pedir POST /departamentos "$ADMIN_TOKEN" '{"id":"IT","nombre":"Tecnología","descripcion":"Departamento de TI"}'
  esperar_codigo "POST /departamentos (IT)" 201
fi

elegir_id() { # elegir_id E001: E001 si está libre; si no, E001-<sufijo>
  pedir GET "/empleados/$1" "$ADMIN_TOKEN"
  if [ "$CODIGO" = 404 ]; then echo "$1"; else echo "$1-$SUFIJO"; fi
}
E1="${ID_E001:-$(elegir_id E001)}"
E2="${ID_E002:-$(elegir_id E002)}"
EMAIL1="juan.perez.$SUFIJO@empresa.com"
EMAIL2="ana.munoz.$SUFIJO@empresa.com"
[ "$E1" = E001 ] || info "E001 ya existe (corrida anterior): esta corrida usa los ids $E1 y $E2"

crear_empleado() { # crear_empleado ID NOMBRE APELLIDO EMAIL N
  pedir POST /empleados "$ADMIN_TOKEN" "$(printf '{"id":"%s","nombre":"%s","apellido":"%s","email":"%s",
    "numeroEmpleado":"EMP-%s-%s","cargo":"Desarrollador","area":"Tecnología",
    "departamentoId":"IT","fechaIngreso":"2026-03-01"}' "$1" "$2" "$3" "$4" "$SUFIJO" "$5")"
  esperar_codigo "POST /empleados $1 ($4)" 201
}
crear_empleado "$E1" Juan Pérez "$EMAIL1" 1
crear_empleado "$E2" Ana Muñoz "$EMAIL2" 2
[ "$PASO_FALLO" -eq 0 ] || abortar "No se crearon los empleados"

# ---------------------------------------------------------------- 3
paso "3 — Verificar eventos en los logs (auth-service y notificaciones-service)"
for par in "$E1:$EMAIL1" "$E2:$EMAIL2"; do
  emp="${par%%:*}"; email="${par#*:}"
  if esperar 30 hay_log auth-service "empleado\.creado recibido .*cuenta del empleado $emp creada"; then
    l="$(linea_log auth-service "empleado\.creado recibido .*cuenta del empleado $emp creada")"
    ok "auth-service detectó empleado.creado y generó el usuario de $emp"; mostrar_log auth-service "$l"
    id_evento="$(printf '%s' "$l" | grep -oE '\(id [0-9a-f-]+\)' | tr -d '()' | awk '{print $2}')"
    l="$(linea_log empleados-service "Evento publicado: empleado\.creado \(id $id_evento\)")"
    comprobar "es el mismo empleado.creado que publicó empleados-service (id $id_evento)" test -n "$l"
  else
    falla "auth-service no registró el alta de la cuenta de $emp"
  fi
  if esperar 30 hay_notificacion usuario.creado "$emp"; then
    r="$(notificacion_registrada usuario.creado "$emp")"; id_evento="${r#* }"
    ok "notificaciones-service detectó usuario.creado de $emp (eventoId $id_evento)"
    l="$(linea_log auth-service "evento publicado usuario\.creado \(id $id_evento\)")"
    comprobar "auth-service publicó ese usuario.creado" test -n "$l"; mostrar_log auth-service "$l"
    l="$(linea_log notificaciones-service "$NOTIF Tipo: SEGURIDAD \| Para: $email \|")"
    comprobar "se simuló el envío de las credenciales (línea [NOTIFICACIÓN] para $email)" test -n "$l"
    mostrar_log notificaciones-service "$(printf '%s' "$l" | sed -E 's/token ([0-9a-f]{8})[0-9a-f-]{28}/token \1-…/')"
  else
    falla "notificaciones-service no registró usuario.creado de $emp"
  fi
done

# ---------------------------------------------------------------- 4
paso "4 — Petición denegada: GET /empleados sin cabeceras"
pedir GET /empleados
esperar_codigo "GET /empleados sin token" 401
info "respuesta: $(head -c 200 "$TMP/cuerpo")"

# ---------------------------------------------------------------- 5
paso "5 — Establecer contraseña con el token del log de la notificación (POST /auth/reset-password)"
activar() { # activar EMAIL: lee el token de activación del log y fija PASS_INICIAL
  local t; t="$(token_de_log "$1")"
  if [ -z "$t" ]; then falla "no se encontró el token de activación de $1 en el log"; return 1; fi
  info "token de $1 leído del log de notificaciones-service: ${t:0:8}-…"
  if token_en_mailhog "$1" "$t"; then info "(el mismo token llegó por correo a Mailhog)"; fi
  pedir POST /auth/reset-password "" "$(printf '{"token":"%s","newPassword":"%s"}' "$t" "$PASS_INICIAL")"
  esperar_codigo "POST /auth/reset-password de $1" 200
  TOKEN_ACTIVACION="$t"
}
activar "$EMAIL1"; TOKEN_ACTIVACION_E1="${TOKEN_ACTIVACION:-}"
activar "$EMAIL2"
pedir POST /auth/reset-password "" "$(printf '{"token":"%s","newPassword":"%s"}' "$TOKEN_ACTIVACION_E1" "$PASS_INICIAL")"
esperar_codigo "el mismo token por segunda vez (un solo uso)" 401

# ---------------------------------------------------------------- 6
paso "6 — Login USER con la contraseña recién establecida"
login "$EMAIL1" "$PASS_INICIAL"
esperar_codigo "POST /auth/login de $E1" 200
USER_TOKEN="$TOKEN"
[ -n "$USER_TOKEN" ] || abortar "Sin el JWT de $E1 no se puede continuar"
info "claims: $(claims "$USER_TOKEN")"

# ---------------------------------------------------------------- 7
paso "7 — Lectura exitosa: GET /empleados con Authorization: Bearer <token USER>"
pedir GET /empleados "$USER_TOKEN"
esperar_codigo "GET /empleados con el JWT de $E1" 200
info "empleados en la respuesta: $(python3 -c 'import json,sys;print(len(json.load(open(sys.argv[1]))))' "$TMP/cuerpo")"

# ---------------------------------------------------------------- 8
paso "8 — Recuperación: POST /auth/recover-password, nuevo token, repetir los pasos 5 y 6"
DESDE_PASO="$(date -u +%FT%T)"
pedir POST /auth/recover-password "" "$(printf '{"email":"%s"}' "$EMAIL1")"
esperar_codigo "POST /auth/recover-password ($EMAIL1)" 202
if esperar 30 hay_token "$EMAIL1" "$TOKEN_ACTIVACION_E1"; then
  TOKEN_RECUPERACION="$(token_de_log "$EMAIL1" "$TOKEN_ACTIVACION_E1")"
  ok "nuevo token en el log de notificaciones-service: ${TOKEN_RECUPERACION:0:8}-… (distinto del de activación)"
  r="$(notificacion_registrada usuario.recuperacion "" "$DESDE_PASO")"
  comprobar "notificaciones-service registró el evento usuario.recuperacion (eventoId ${r#* })" test -n "$r"
  l="$(linea_log auth-service "evento publicado usuario\.recuperacion \(id ${r#* }\)")"
  comprobar "auth-service publicó ese usuario.recuperacion" test -n "$l"
  pedir POST /auth/reset-password "" "$(printf '{"token":"%s","newPassword":"%s"}' "$TOKEN_RECUPERACION" "$PASS_RECUPERADA")"
  esperar_codigo "repetición del paso 5: reset-password con el token nuevo" 200
  login "$EMAIL1" "$PASS_RECUPERADA"
  esperar_codigo "repetición del paso 6: login con la contraseña recuperada" 200
  [ -n "$TOKEN" ] && USER_TOKEN="$TOKEN"
  comprobar "la contraseña anterior ya no sirve -> HTTP $(codigo_login "$EMAIL1" "$PASS_INICIAL")" \
    test "$(codigo_login "$EMAIL1" "$PASS_INICIAL")" = 401
else
  falla "no llegó al log un token de recuperación nuevo para $EMAIL1"
fi

# ---------------------------------------------------------------- 9
paso "9 — Escritura denegada: DELETE /empleados/$E1 con el JWT del usuario regular"
pedir DELETE "/empleados/$E1" "$USER_TOKEN"
esperar_codigo "DELETE /empleados/$E1 con rol USER" 403
pedir GET "/empleados/$E1" "$ADMIN_TOKEN"
comprobar "el empleado sigue ACTIVO (estado: $(campo estado))" test "$(campo estado)" = ACTIVO

# ---------------------------------------------------------------- 10
paso "10 — Propiedad del recurso: PUT /perfiles con el JWT de $E1"
# El perfil lo crea perfiles-service al consumir empleado.creado (asincrónico).
esperar 30 existe "/perfiles/$E1" && esperar 30 existe "/perfiles/$E2" || true
pedir PUT "/perfiles/$E1" "$USER_TOKEN" '{"telefono":"3001234567","ciudad":"Armenia","biografia":"Ingeniero de sistemas"}'
esperar_codigo "PUT /perfiles/$E1 (su propio perfil)" 200
info "perfil: ciudad=$(campo ciudad), telefono=$(campo telefono)"
pedir PUT "/perfiles/$E2" "$USER_TOKEN" '{"ciudad":"Bogotá"}'
esperar_codigo "PUT /perfiles/$E2 (perfil ajeno)" 403

# ---------------------------------------------------------------- 11
paso "11 — Cambio de contraseña: POST /auth/change-password con el JWT de $E1"
pedir POST /auth/change-password "$USER_TOKEN" "$(printf '{"currentPassword":"%s","newPassword":"%s"}' "$PASS_RECUPERADA" "$PASS_NUEVA")"
esperar_codigo "POST /auth/change-password" 200
login "$EMAIL1" "$PASS_RECUPERADA"; esperar_codigo "login con la contraseña antigua" 401
login "$EMAIL1" "$PASS_NUEVA"; esperar_codigo "login con la contraseña nueva" 200

# ---------------------------------------------------------------- 12
paso "12 — Programar vacaciones (ADMIN): POST /vacaciones para $E1"
DESDE_PASO="$(date -u +%FT%T)"
INI1="$(proximo_minuto)"; FIN1=$((INI1 + DURACION_E001))
info "Desviación: en lugar de fechaInicio = fechaFin = hoy (fin a las 23:59:59, D6) se usan horas reales de hoy:"
info "fechaInicio=$(hora_local "$INI1") fechaFin=$(hora_local "$FIN1") ($ZONA)"
pedir POST /vacaciones "$ADMIN_TOKEN" "$(printf '{"empleadoId":"%s","fechaInicio":"%s","fechaFin":"%s"}' "$E1" "$(hora_local "$INI1")" "$(hora_local "$FIN1")")"
esperar_codigo "POST /vacaciones ($E1)" 201
VAC1="$(campo id)"
[ -n "$VAC1" ] || abortar "No se programaron las vacaciones de $E1"
info "vacacionesId=$VAC1 estado=$(campo estado)"
if esperar 30 hay_notificacion vacaciones.programadas "$E1" "$DESDE_PASO"; then
  r="$(notificacion_registrada vacaciones.programadas "$E1" "$DESDE_PASO")"
  l="$(linea_log vacaciones-service "Evento publicado: vacaciones\.programadas \(id ${r#* }\)")"
  comprobar "vacaciones-service publicó vacaciones.programadas (id ${r#* })" test -n "$l"; mostrar_log vacaciones-service "$l"
  pedir GET "/notificaciones/$E1" "$ADMIN_TOKEN"
  m="$(python3 -c "import json,sys;print(next((n['mensaje'] for n in json.load(open(sys.argv[1])) if n['tipo']=='VACACIONES' and 'programadas' in n['mensaje']),''))" "$TMP/cuerpo")"
  comprobar "llegó la notificación de confirmación (VACACIONES): \"$m\"" test -n "$m"
else
  falla "notificaciones-service no registró vacaciones.programadas de $E1"
fi

# ---------------------------------------------------------------- 13
paso "13 — Desactivación automática al iniciar las vacaciones"
info "El scheduler corre cada minuto (VACACIONES_CRON); el inicio se detecta a las $(hora_local "$INI1") (espera de ~$((INI1 - $(date +%s))) s)"
if esperar $((INI1 - $(date +%s) + 90)) vacaciones_en "$VAC1" EN_CURSO; then
  ok "GET /vacaciones/$VAC1 -> estado EN_CURSO"
  esperar 30 hay_notificacion cuenta.desactivada "$E1" || true
  r="$(notificacion_registrada vacaciones.iniciadas "$E1")"
  l_vac="$(linea_log vacaciones-service "Evento publicado: vacaciones\.iniciadas \(id ${r#* }\)")"
  l_auth="$(linea_log auth-service "cuenta del empleado $E1 suspendida por las vacaciones $VAC1")"
  l_not="$(linea_log notificaciones-service "$NOTIF Tipo: CUENTA \| Para: $EMAIL1 \| .*desactivada temporalmente por vacaciones")"
  r_not="$(notificacion_registrada cuenta.desactivada "$E1")"
  comprobar "1) vacaciones-service publicó vacaciones.iniciadas" test -n "$l_vac"; mostrar_log vacaciones-service "$l_vac"
  comprobar "2) auth-service consumió el evento y suspendió la cuenta" test -n "$l_auth"; mostrar_log auth-service "$l_auth"
  comprobar "3) notificaciones-service recibió cuenta.desactivada (motivo VACACIONES, temporal)" test -n "$l_not" -a -n "$r_not"
  mostrar_log notificaciones-service "$l_not"
  if [ -n "$l_vac" ] && [ -n "$l_auth" ] && [ -n "$l_not" ]; then
    comprobar "en ese orden: $(marca "$l_vac") <= $(marca "$l_auth") <= $(marca "$r_not") (marcas de Docker)" \
      en_orden "$(marca "$l_vac")" "$(marca "$l_auth")" "$(marca "$r_not")"
  fi
  esperar 20 login_da "$EMAIL1" "$PASS_NUEVA" 401 || true
  login "$EMAIL1" "$PASS_NUEVA"; esperar_codigo "POST /auth/login de $E1 durante las vacaciones" 401
else
  falla "las vacaciones $VAC1 no pasaron a EN_CURSO (estado: $(estado_vacaciones "$VAC1"))"
fi

# ---------------------------------------------------------------- 14
paso "14 — Reactivación automática al llegar la fechaFin"
info "El fin se detecta a las $(hora_local "$FIN1") (espera de ~$((FIN1 - $(date +%s))) s)"
DESDE_PASO="$(date -u -d "@$FIN1" +%FT%T)" # el scheduler publica el fin en la primera pasada >= fechaFin
if esperar $((FIN1 - $(date +%s) + 90)) vacaciones_en "$VAC1" FINALIZADA; then
  ok "GET /vacaciones/$VAC1 -> estado FINALIZADA"
  esperar 30 hay_notificacion cuenta.activada "$E1" "$DESDE_PASO" || true
  l="$(linea_log_desde vacaciones-service "$DESDE_PASO" "Evento publicado: vacaciones\.finalizadas")"
  comprobar "vacaciones-service publicó vacaciones.finalizadas" test -n "$l"; mostrar_log vacaciones-service "$l"
  l="$(linea_log auth-service "cuenta del empleado $E1 reactivada al terminar las vacaciones $VAC1")"
  comprobar "auth-service reactivó la cuenta" test -n "$l"; mostrar_log auth-service "$l"
  l="$(linea_log notificaciones-service "$NOTIF Tipo: CUENTA \| Para: $EMAIL1 \| .*Bienvenido de regreso")"
  comprobar "cuenta.activada con motivo FIN_VACACIONES (\"Bienvenido de regreso\")" test -n "$l"; mostrar_log notificaciones-service "$l"
  esperar 20 login_da "$EMAIL1" "$PASS_NUEVA" 200 || true
  login "$EMAIL1" "$PASS_NUEVA"; esperar_codigo "POST /auth/login de $E1 vuelve a funcionar" 200
else
  falla "las vacaciones $VAC1 no pasaron a FINALIZADA (estado: $(estado_vacaciones "$VAC1"))"
fi

# ---------------------------------------------------------------- 15
paso "15 — Caso borde: retiro de $E2 durante sus vacaciones"
login "$EMAIL2" "$PASS_INICIAL"; esperar_codigo "antes: login de $E2" 200
INI2="$(proximo_minuto)"; FIN2=$((INI2 + DURACION_E002))
pedir POST /vacaciones "$ADMIN_TOKEN" "$(printf '{"empleadoId":"%s","fechaInicio":"%s","fechaFin":"%s"}' "$E2" "$(hora_local "$INI2")" "$(hora_local "$FIN2")")"
esperar_codigo "POST /vacaciones $E2 ($(hora_local "$INI2") a $(hora_local "$FIN2"))" 201
VAC2="$(campo id)"
info "vacacionesId=$VAC2; inicio en ~$((INI2 - $(date +%s))) s, fin en ~$((FIN2 - $(date +%s))) s"
if [ -n "$VAC2" ] && esperar $((INI2 - $(date +%s) + 90)) vacaciones_en "$VAC2" EN_CURSO \
   && esperar 30 hay_log auth-service "cuenta del empleado $E2 suspendida por las vacaciones $VAC2"; then
  ok "las vacaciones iniciaron y la cuenta de $E2 quedó suspendida"
  mostrar_log auth-service "$(linea_log auth-service "cuenta del empleado $E2 suspendida por las vacaciones $VAC2")"
  login "$EMAIL2" "$PASS_INICIAL"; esperar_codigo "login de $E2 durante las vacaciones" 401
  pedir DELETE "/empleados/$E2" "$ADMIN_TOKEN"
  esperar_codigo "DELETE /empleados/$E2 con el JWT de ADMIN (mientras está de vacaciones)" 200
  if esperar 30 hay_log auth-service "cuenta del empleado $E2 desactivada de forma permanente"; then
    ok "auth-service: cuenta de $E2 DESACTIVADA_PERMANENTE"
    mostrar_log auth-service "$(linea_log auth-service "cuenta del empleado $E2 desactivada de forma permanente")"
  else
    falla "auth-service no registró el retiro de $E2"
  fi
  info "Esperando la fechaFin ($(hora_local "$FIN2"), ~$((FIN2 - $(date +%s))) s)..."
  if esperar $((FIN2 - $(date +%s) + 90)) vacaciones_en "$VAC2" FINALIZADA \
     && esperar 30 hay_log auth-service "no se reactiva \(empleado $E2, vacaciones $VAC2\)"; then
    l="$(linea_log_desde vacaciones-service "$(date -u -d "@$FIN2" +%FT%T)" "Evento publicado: vacaciones\.finalizadas")"
    comprobar "vacaciones-service publicó vacaciones.finalizadas de $VAC2 (estado FINALIZADA)" test -n "$l"
    mostrar_log vacaciones-service "$l"
    l="$(linea_log auth-service "no se reactiva \(empleado $E2, vacaciones $VAC2\)")"
    ok "auth-service ignoró vacaciones.finalizadas sobre la cuenta retirada (D5)"; mostrar_log auth-service "$l"
  else
    falla "no se observó vacaciones.finalizadas ignorado para $E2 (estado: $(estado_vacaciones "$VAC2"))"
  fi
  login "$EMAIL2" "$PASS_INICIAL"; esperar_codigo "después de la fechaFin: login de $E2 sigue fallando" 401
  pedir GET "/notificaciones/$E2" "$ADMIN_TOKEN"
  regreso="$(python3 -c "import json,sys;print(sum('Bienvenido de regreso' in n['mensaje'] for n in json.load(open(sys.argv[1]))))" "$TMP/cuerpo")"
  comprobar "$E2 no recibió la \"bienvenida de regreso\" (notificaciones: $regreso)" test "$regreso" = 0
else
  falla "las vacaciones de $E2 no iniciaron o la cuenta no se suspendió (estado: $(estado_vacaciones "${VAC2:-?}"))"
fi

# ---------------------------------------------------------------- 16
paso "16 — Offboarding (ADMIN): DELETE /empleados/$E1"
DESDE_PASO="$(date -u +%FT%T)"
pedir DELETE "/empleados/$E1" "$ADMIN_TOKEN"
esperar_codigo "DELETE /empleados/$E1 con el JWT del administrador semilla" '200|204'
comprobar "estado RETIRADO (estado: $(campo estado), fechaRetiro: $(campo fechaRetiro))" test "$(campo estado)" = RETIRADO
if esperar 30 hay_log auth-service "cuenta del empleado $E1 desactivada de forma permanente"; then
  l="$(linea_log_desde empleados-service "$DESDE_PASO" 'Evento publicado: empleado\.retirado')"
  comprobar "empleados-service publicó empleado.retirado" test -n "$l"; mostrar_log empleados-service "$l"
  mostrar_log auth-service "$(linea_log auth-service "cuenta del empleado $E1 desactivada de forma permanente")"
else
  falla "auth-service no consumió empleado.retirado de $E1"
fi

# ---------------------------------------------------------------- 17
paso "17 — Verificar inhabilitación: login del empleado retirado"
login "$EMAIL1" "$PASS_NUEVA"
esperar_codigo "POST /auth/login de $E1 (retirado)" '401|403'

# ---------------------------------------------------------------- 18
paso "18 — Verificar auditoría: GET /empleados?estado=RETIRADO"
pedir GET "/empleados?estado=RETIRADO" "$ADMIN_TOKEN"
esperar_codigo "GET /empleados?estado=RETIRADO" 200
f="$(python3 -c "
import json,sys
print(next((e.get('fechaRetiro') or '' for e in json.load(open(sys.argv[1])) if e['id']==sys.argv[2]),''))" "$TMP/cuerpo" "$E1")"
comprobar "lista a $E1 con su fechaRetiro ($f)" test -n "$f"

resumen_final
