#!/bin/sh
# Importa la topología del ecosistema (exchange, colas y bindings de definitions.json)
# a través de la API de administración, DESPUÉS de que el broker arrancó.
#
# Por qué no se usa load_definitions al arrancar: cuando RabbitMQ carga definiciones
# en el boot, no crea el usuario por defecto (RABBITMQ_DEFAULT_USER/PASS) y la única
# alternativa sería versionar credenciales dentro de definitions.json.
#
# Es idempotente: importar las mismas definiciones de nuevo no cambia nada.
set -eu

URL="http://message-broker:15672/api/definitions"
AUTH="$(printf '%s:%s' "$RABBITMQ_USER" "$RABBITMQ_PASS" | base64 | tr -d '\n')"

# El healthcheck del broker verifica el puerto AMQP; la API de administración
# puede tardar unos segundos más en aceptar peticiones.
for intento in 1 2 3 4 5 6 7 8 9 10; do
  if wget -q -O /dev/null \
      --header "Authorization: Basic $AUTH" \
      --header "Content-Type: application/json" \
      --post-file /definitions.json "$URL"; then
    echo "Topología de eventos importada en el broker"
    exit 0
  fi
  echo "API de administración no disponible todavía (intento $intento/10)"
  sleep 3
done

echo "ERROR: no se pudo importar la topología de eventos" >&2
exit 1
