#!/bin/sh
# Configuración de SonarQube como código (Reto 7, D15 y D19). Se ejecuta en cada
# `docker compose --profile ci up` y es idempotente: correrlo dos veces deja lo mismo.
#   1. Espera a que SonarQube responda UP.
#   2. Si admin conserva admin/admin, le pone SONAR_ADMIN_PASSWORD.
#   3. Usuario "jenkins" solo con permisos de análisis; su token va al volumen compartido
#      (Jenkins lo monta de solo lectura). Un token válido se conserva: si se regenerara,
#      el Jenkins que ya está corriendo quedaría con uno revocado.
#   4. Quality gate "TalentFlow" (cobertura total < 70 % → ERROR), por defecto.
#   5. Webhook "jenkins" para que waitForQualityGate reciba el resultado.
# Cualquier fallo termina con código distinto de 0 y Jenkins no arranca.
set -eu

SONAR_URL="${SONAR_URL:-http://sonarqube:9000}"
ADMIN_PASSWORD="${SONAR_ADMIN_PASSWORD:?falta SONAR_ADMIN_PASSWORD}"
WEBHOOK_URL="${SONAR_WEBHOOK_URL:-http://jenkins:8080/sonarqube-webhook/}"
COBERTURA_MINIMA="${SONAR_COBERTURA_MINIMA:-70}"
ARCHIVO_TOKEN=/sonar-token/token
GATE=TalentFlow

log() { echo "[sonar-init] $*"; }

# Llamada a la API como admin; falla (curl -f) ante cualquier respuesta HTTP de error.
api() {
  metodo="$1"; ruta="$2"; shift 2
  curl -fsS -u "admin:${ADMIN_PASSWORD}" -X "$metodo" "${SONAR_URL}${ruta}" "$@"
}

# 1. Esperar a SonarQube (hasta 5 minutos).
log "esperando a SonarQube en ${SONAR_URL}"
i=0
until [ "$(curl -fsS "${SONAR_URL}/api/system/status" 2>/dev/null | jq -r .status 2>/dev/null)" = "UP" ]; do
  i=$((i + 1))
  [ "$i" -le 60 ] || { log "SonarQube no llegó a UP en 5 minutos"; exit 1; }
  sleep 5
done
log "SonarQube UP"

# 2. Contraseña del administrador. Solo se cambia si sigue la de fábrica.
valida() { curl -fsS -u "admin:$1" "${SONAR_URL}/api/authentication/validate" | jq -e '.valid == true' >/dev/null; }
if [ "$ADMIN_PASSWORD" != "admin" ] && valida admin; then
  curl -fsS -u admin:admin -X POST "${SONAR_URL}/api/users/change_password" \
    --data-urlencode login=admin --data-urlencode previousPassword=admin \
    --data-urlencode "password=${ADMIN_PASSWORD}"
  log "contraseña de admin cambiada (admin/admin ya no sirve)"
fi
valida "$ADMIN_PASSWORD" || { log "admin no autentica con SONAR_ADMIN_PASSWORD"; exit 1; }

# 3. Usuario técnico "jenkins": ejecutar análisis (scan) y crear proyectos en el primer
#    análisis (provisioning). Su contraseña es aleatoria y no se usa: entra con token.
#    La existencia se consulta con user_tokens/search (lee la base de datos y responde 404
#    si el usuario no existe); users/search lee Elasticsearch y puede ir atrasado.
if ! api GET "/api/user_tokens/search?login=jenkins" >/dev/null 2>&1; then
  api POST /api/users/create --data-urlencode login=jenkins --data-urlencode name=Jenkins \
    --data-urlencode "password=$(head -c 32 /dev/urandom | base64)" >/dev/null
  log "usuario jenkins creado"
fi
for permiso in scan provisioning; do
  api POST /api/permissions/add_user --data-urlencode login=jenkins --data-urlencode "permission=${permiso}"
done

token_valido() {
  [ -s "$ARCHIVO_TOKEN" ] || return 1
  curl -fsS -u "$(cat "$ARCHIVO_TOKEN"):" "${SONAR_URL}/api/authentication/validate" | jq -e '.valid == true' >/dev/null
}
if token_valido; then
  log "token de jenkins vigente: se conserva"
else
  # Revocar si existe (por ejemplo, el volumen se borró) y generar uno nuevo.
  api POST /api/user_tokens/revoke --data-urlencode login=jenkins --data-urlencode name=jenkins
  token="$(api POST /api/user_tokens/generate --data-urlencode login=jenkins --data-urlencode name=jenkins | jq -er .token)"
  umask 077
  printf '%s' "$token" > "$ARCHIVO_TOKEN"
  token_valido || { log "el token nuevo no autentica"; exit 1; }
  log "token de jenkins generado en ${ARCHIVO_TOKEN}"
fi

# 4. Quality gate: una sola condición, cobertura total menor que el mínimo → ERROR.
if ! api GET "/api/qualitygates/show?name=${GATE}" >/dev/null 2>&1; then
  api POST /api/qualitygates/create --data-urlencode "name=${GATE}" >/dev/null
  log "quality gate ${GATE} creado"
fi
condiciones="$(api GET "/api/qualitygates/show?name=${GATE}" | jq -c '.conditions // []')"
id_cobertura="$(echo "$condiciones" | jq -r '[.[] | select(.metric == "coverage")][0].id // empty')"
if [ -n "$id_cobertura" ]; then
  api POST /api/qualitygates/update_condition --data-urlencode "id=${id_cobertura}" \
    --data-urlencode metric=coverage --data-urlencode op=LT --data-urlencode "error=${COBERTURA_MINIMA}" >/dev/null
else
  api POST /api/qualitygates/create_condition --data-urlencode "gateName=${GATE}" \
    --data-urlencode metric=coverage --data-urlencode op=LT --data-urlencode "error=${COBERTURA_MINIMA}" >/dev/null
fi
# Cualquier otra condición (agregada a mano) se quita: el gate es el del repositorio.
for id in $(echo "$condiciones" | jq -r '.[] | select(.metric != "coverage") | .id'); do
  api POST /api/qualitygates/delete_condition --data-urlencode "id=${id}"
done
api POST /api/qualitygates/set_as_default --data-urlencode "name=${GATE}"
log "quality gate ${GATE}: coverage < ${COBERTURA_MINIMA} % → ERROR, por defecto"

# 5. Webhook hacia Jenkins: exactamente uno llamado "jenkins" con la URL esperada.
claves="$(api GET /api/webhooks/list | jq -r '.webhooks[] | select(.name == "jenkins") | .key')"
primera=""
for clave in $claves; do
  if [ -z "$primera" ]; then
    primera="$clave"
    api POST /api/webhooks/update --data-urlencode "webhook=${clave}" \
      --data-urlencode name=jenkins --data-urlencode "url=${WEBHOOK_URL}"
  else
    api POST /api/webhooks/delete --data-urlencode "webhook=${clave}"
  fi
done
if [ -z "$primera" ]; then
  api POST /api/webhooks/create --data-urlencode name=jenkins --data-urlencode "url=${WEBHOOK_URL}" >/dev/null
fi
log "webhook jenkins → ${WEBHOOK_URL}"
log "listo"
