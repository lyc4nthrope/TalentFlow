# Reto 8 — Paso a paso para la sustentación

Orden: el de los criterios de evaluación del PDF. Cada paso dice **qué ejecutar**, **qué mostrar** y **qué se espera**. Todo desde la raíz del repo. Tiempo total: unos 20 minutos, la mayoría esperando que se disparen las alertas.

## 0. Antes de entrar (preparación, no se muestra)

```bash
docker compose up --build -d
docker compose ps        # los 7 servicios en (healthy)
TOKEN=$(curl -s -X POST localhost:8080/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@empresa.com","password":"Admin123*"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])")
for i in $(seq 1 20); do for r in empleados departamentos perfiles vacaciones; do curl -s -o /dev/null localhost:8080/$r -H "Authorization: Bearer $TOKEN"; done; done
```

Pestañas abiertas: Prometheus `:9091/targets`, Grafana `:3000` (admin / talentflow_grafana_2026) con el dashboard, Jaeger `:16686` y Mailhog `:8025`.

Comprobar: los 7 targets en `UP` y el dashboard con datos.

## 1. Infraestructura de observabilidad (1.0)

| # | Ejecutar / mostrar | Se espera |
|---|---|---|
| 1.1 | `docker-compose.yml`: servicios `prometheus`, `grafana`, `loki`, `promtail`, `jaeger` | Versiones fijas, todos en `microservices-network` |
| 1.2 | `observability/prometheus/prometheus.yml` | 7 `scrape_job` por nombre de Docker; el Gateway primero (`api-gateway:9464`) |
| 1.3 | `http://localhost:9091/targets` | 7 de 7 `UP` (captura 01) |
| 1.4 | Explicar el diagrama del README | Pull de métricas, push de trazas y logs, Grafana arriba |

## 2. Instrumentación (1.0)

| # | Ejecutar / mostrar | Se espera |
|---|---|---|
| 2.1 | `docker compose exec api-gateway wget -qO- localhost:9464/metrics \| grep http_requests_total \| head -3` | Series con `method`, `route` y `status` |
| 2.2 | `curl -s localhost:8080/health \| python3 -m json.tool \| head -20` | El Gateway con un `checks` por servicio |
| 2.3 | Ancla `x-otel` en `docker-compose.yml` y la tabla *Librerías por microservicio* | Nombre de servicio, exportador `jaeger:4318`, W3C en los 7 |
| 2.4 | Crear un empleado (comando abajo) y abrir su traza en Jaeger | 1 traza, 7 servicios, ~21 spans: Gateway → empleados → departamentos → RabbitMQ → auth → notificaciones, perfiles y vacaciones (captura 03) |

```bash
N=$RANDOM; curl -s -X POST localhost:8080/empleados -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"id\":\"E$N\",\"nombre\":\"Ana\",\"apellido\":\"Gómez\",\"email\":\"ana.$N@empresa.com\",\"numeroEmpleado\":\"N$N\",\"cargo\":\"Analista\",\"area\":\"TI\",\"departamentoId\":\"IT\",\"fechaIngreso\":\"2026-04-01\"}"
```

## 3. Logs centralizados (1.0)

| # | Ejecutar / mostrar | Se espera |
|---|---|---|
| 3.1 | `docker logs --tail 3 ms-auth` | Líneas JSON con `timestamp`, `level`, `service`, `message`, `traceId` |
| 3.2 | Copiar el `traceId` de la traza del paso 2.4. Grafana → Explore → Loki → `{project="talentflow"} \|= "<traceId>" \| json` | Los logs de esa petición en 6 servicios (captura 10) |
| 3.3 | En una línea, clic en el enlace del `traceId` | Abre la misma traza en Jaeger |

## 4. Dashboard (0.5)

| # | Ejecutar / mostrar | Se espera |
|---|---|---|
| 4.1 | Dashboard *TalentFlow - Observabilidad* | Fila 1: estado de los 7 (Stat verde/rojo). Fila 2: tasa, latencia y errores. Fila 3: CPU, memoria y logs |
| 4.2 | `observability/grafana/dashboards/talentflow.json` y `provisioning/dashboards/dashboards.yml` | Está en Git; Grafana lo carga solo al arrancar |

## 5 y 6. Alertas y caos (0.5 + 1.0)

| # | Ejecutar / mostrar | Se espera |
|---|---|---|
| 5.1 | `observability/grafana/provisioning/alerting/` | 3 reglas, punto de contacto por email, política |
| 6.1 | `docker compose stop departamentos-service` | En ~15 s: target `DOWN` y panel en rojo |
| 6.2 | Esperar ~2 min y abrir Mailhog | Correo `[FIRING:1] Servicio caído departamentos-service` (captura 06) |
| 6.3 | `docker compose start departamentos-service` | Correo `[RESOLVED]` |
| 6.4 | Caos de latencia (comandos abajo) | Panel de latencia de departamentos en ~2,5 s; a los ~2 min, correo `Alta latencia` de departamentos y del Gateway (captura 08) |
| 6.5 | Crear un empleado con el caos encendido y abrir su traza | La barra de departamentos ocupa casi toda la traza (captura 09) |
| 6.6 | Responder la pregunta del PDF | Sección [¿Qué servicio tardó más?](../../README.md#qué-servicio-del-ecosistema-tardó-más-en-responder-y-cómo-lo-identificaron) |

```bash
docker compose -f docker-compose.yml -f docker-compose.caos.yml up -d departamentos-service
for i in $(seq 1 80); do curl -s -o /dev/null localhost:8080/departamentos -H "Authorization: Bearer $TOKEN"; done
```

Respuesta corta: con el sistema normal, empleados es el que más tarda dentro de la respuesta (49,5 de 52,5 ms), y vacaciones tiene el span más largo, pero en segundo plano. Con el caos, departamentos se lleva 2510 de 2532 ms. Se identificó cruzando los tres pilares: la alerta dice *qué* servicio, el panel de latencia lo confirma y la traza muestra *por qué*, porque los demás solo lo esperan.

## Después

```bash
docker compose up -d departamentos-service   # quitar el caos si quedó encendido
docker compose ps                            # todo en (healthy)
```
