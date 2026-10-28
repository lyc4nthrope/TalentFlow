#!/bin/sh
# D20: Elasticsearch (el buscador interno de SonarQube) usa por defecto límites de disco en
# porcentaje: por encima del 95 % de uso bloquea sus índices y SonarQube deja de funcionar,
# y solo los desbloquea por debajo del 90 %. En un disco grande eso pasa con varios GB
# libres, algo común en las máquinas del equipo. Este arranque inicia SonarQube como siempre
# y, cuando Elasticsearch responde, fija límites en espacio libre absoluto (como ajustes
# persistentes, que quedan en el volumen de datos) y quita un bloqueo previo si lo hubiera.
# Si el disco llega de verdad a menos de 1 GB libre, Elasticsearch sigue protegiéndose.

/opt/sonarqube/docker/entrypoint.sh "$@" &
sonar=$!
# Al detener el contenedor, se pasa la señal a SonarQube y se espera a que cierre de forma
# ordenada (su base y su índice); si el script terminara antes, Docker lo cortaría de golpe.
trap 'kill -TERM "$sonar" 2>/dev/null; wait "$sonar"' TERM INT

(
  for _ in $(seq 1 120); do
    if wget -q -O /dev/null --method=PUT --header='Content-Type: application/json' \
        --body-data='{"persistent":{"cluster.routing.allocation.disk.watermark.low":"2gb","cluster.routing.allocation.disk.watermark.high":"1500mb","cluster.routing.allocation.disk.watermark.flood_stage":"1gb"}}' \
        http://127.0.0.1:9001/_cluster/settings; then
      wget -q -O /dev/null --method=PUT --header='Content-Type: application/json' \
        --body-data='{"index.blocks.read_only_allow_delete":null}' \
        http://127.0.0.1:9001/_all/_settings || true
      echo "arranque.sh: límites de disco de Elasticsearch fijados (2gb / 1500mb / 1gb libres)"
      exit 0
    fi
    sleep 5
  done
  echo "arranque.sh: Elasticsearch no respondió en 10 minutos; límites de disco sin cambiar" >&2
) &

wait "$sonar"
