# Reto 8 — Manual paso a paso

Qué demuestra este reto: los 7 microservicios se observan desde un solo lugar, con métricas en Prometheus, trazas en Jaeger y logs en Loki, todo reunido en Grafana. Además, avisan por correo cuando algo falla. URL base: `http://localhost:8080` · usuario de prueba: `admin@empresa.com / Admin123*`.

| Archivo | Para qué |
|---|---|
| **Este README** | Manual: criterios → dónde se demuestran, cómo ejecutar, preguntas, problemas comunes |
| [README raíz, sección Observabilidad](../../README.md#observabilidad-reto-8) | Lo que el PDF pide "en el README": conceptos, diagrama, librerías, LogQL, canal de alertas, capturas y la respuesta a la pregunta |
| [`PLAN.md`](PLAN.md) | Diagnóstico, decisiones D1-D16 y aprendizajes de cada fase |
| [`pruebas.md`](pruebas.md) | Evidencia, con las 10 capturas |
| [`sustentacion.md`](sustentacion.md) | Paso a paso para la sustentación |

## 0. Criterios de evaluación → dónde se demuestran

| # | Criterio (puntos) | Dónde | Comando |
|---|---|---|---|
| 1 | Infraestructura de observabilidad (1.0): stack en `docker-compose.yml`, redes, `prometheus.yml` con todos los servicios | `docker-compose.yml` (servicios `prometheus`, `grafana`, `loki`, `promtail`, `jaeger`); `observability/prometheus/prometheus.yml` (7 `scrape_job`, el Gateway primero); captura 01 | `docker compose up -d` → `http://localhost:9091/targets` |
| 2 | Instrumentación (1.0): `/metrics` y `/health` en todos, OpenTelemetry con nombre, exportador y W3C, traza completa | [Librerías por microservicio](../../README.md#librerías-por-microservicio); `x-otel` en `docker-compose.yml`; captura 03 (7 servicios, 21 spans) | `curl -s -X POST localhost:8080/empleados ...` → Jaeger |
| 3 | Logs centralizados (1.0): JSON en todos, Loki y Promtail, Loki como fuente, consulta LogQL | `observability/promtail/promtail-config.yml`, `observability/grafana/provisioning/datasources/datasources.yml`; [Logs centralizados](../../README.md#logs-centralizados-con-loki); captura 10 | Grafana → Explore → Loki |
| 4 | Dashboard (0.5): 4 paneles, aprovisionado y versionado | `observability/grafana/dashboards/talentflow.json` (3 filas, 11 paneles); captura 02 | Grafana → Dashboards → TalentFlow |
| 5 | Alertas (0.5): ≥ 2 reglas, canal externo, captura de una alerta recibida | `observability/grafana/provisioning/alerting/` (3 reglas, email); capturas 06 y 08 | `docker compose stop departamentos-service` → `http://localhost:8025` |
| 6 | Caos y documentación (1.0): capturas de dashboard, traza y alerta; diagrama; respuesta a la pregunta | [`pruebas.md`](pruebas.md); [diagrama](../../README.md#diagrama-de-arquitectura-de-observabilidad); [respuesta](../../README.md#qué-servicio-del-ecosistema-tardó-más-en-responder-y-cómo-lo-identificaron) | `docker-compose.caos.yml` |

Desviaciones del PDF, con su razón:

| El PDF dice | Se hizo | Por qué |
|---|---|---|
| Prometheus en el puerto 9090 | Puerto 9091 en el host; 9090 dentro de la red | El 9090 del host es Jenkins desde el Reto 7 (D2) |
| `GET http://api-gateway:8080/metrics` | Puerto interno 9464 | El 8080 es público: cualquiera leería las métricas sin JWT (D13) |
| Imágenes `:latest` | Versiones fijas | Un `up` del profesor debe levantar exactamente lo probado (D11) |
| `winston`, `logstash-logback-encoder`... | `console.*` en JSON en Node; logging estructurado nativo de Spring Boot 3.5 | El mismo formato JSON sin agregar librerías ni tocar cada llamada |
| Retardo temporal en el código para el caos | Interruptor apagado por defecto (`docker-compose.caos.yml`) | Se repite sin editar ni reconstruir; nunca queda encendido por error |
| Regla 4: contenedor reiniciado | No se hizo; hay 3 de las 4 | Necesita cAdvisor (métricas de Docker); el PDF pide al menos 2 |

## 1. Prerrequisitos

| Herramienta | Versión probada |
|---|---|
| Docker Engine + Compose | 26.1 + v2 (Linux). En Docker Desktop (Mac), al menos 6–8 GB de memoria en *Settings → Resources* si también se levanta el perfil `ci` |
| Navegador | Cualquiera, para Grafana, Jaeger, Prometheus y Mailhog |
| Puertos libres en el host | 3000, 3100, 8025, 8080, 9091, 16686 |

## 2. Inicio rápido

```bash
docker compose up --build -d                   # 21 contenedores, ~40 s hasta healthy
docker compose ps                              # los 7 servicios en (healthy)

# Token y tráfico de ejemplo
TOKEN=$(curl -s -X POST localhost:8080/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@empresa.com","password":"Admin123*"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])")
curl -s -X POST localhost:8080/empleados -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"id":"E010","nombre":"Ana","apellido":"Gómez","email":"ana@empresa.com","numeroEmpleado":"N010","cargo":"Analista","area":"TI","departamentoId":"IT","fechaIngreso":"2026-04-01"}'
for i in $(seq 1 10); do curl -s -o /dev/null localhost:8080/empleados -H "Authorization: Bearer $TOKEN"; done
```

| Qué | Dónde | Credenciales |
|---|---|---|
| Targets | `http://localhost:9091/targets` | — |
| Dashboard | `http://localhost:3000` → Dashboards → TalentFlow → *TalentFlow - Observabilidad* | `admin` / `talentflow_grafana_2026` |
| Logs | Grafana → Explore → Loki → `{project="talentflow"} \|= "<traceId>"` | — |
| Trazas | `http://localhost:16686` → Service `api-gateway` → Operation `POST /empleados` | — |
| Correos de las alertas | `http://localhost:8025` | — |

Caos:

```bash
docker compose stop departamentos-service      # caída: ~2 min → target DOWN, panel rojo, correo "Servicio caído"
docker compose start departamentos-service     # vuelve: correo "[RESOLVED]"

docker compose -f docker-compose.yml -f docker-compose.caos.yml up -d departamentos-service   # 2,5 s de latencia
for i in $(seq 1 80); do curl -s -o /dev/null localhost:8080/departamentos -H "Authorization: Bearer $TOKEN"; done
docker compose up -d departamentos-service     # quitar el caos
```

## 3. Preguntas probables en la sustentación

- **¿Por qué Prometheus hace pull y las trazas push?** Con pull, Prometheus sabe si un servicio murió (`up` = 0) y controla su propio ritmo. Un span es un evento único: si nadie lo recoge en el momento, se pierde, así que el servicio lo empuja.
- **¿Cómo pasa el `traceId` de Node a Go o a PHP?** En la cabecera estándar `traceparent` (W3C). Cada SDK la entiende, y en PHP la leemos a mano. Por RabbitMQ va en las cabeceras del mensaje.
- **¿Por qué `route` es `/empleados/:id` y no la URL?** Cada id distinto crearía una serie nueva en Prometheus (explosión de cardinalidad). Por la misma razón, el `traceId` no es etiqueta en Loki.
- **¿Por qué la alerta de latencia usa el promedio y el dashboard muestra p95?** El PDF pide el promedio, que es exacto. El p95 se estima interpolando dentro de los rangos del histograma: con 2,5 s reales marcó 4,85 s. Sirve para ver picos, no como umbral exacto.
- **¿Por qué no se cuentan `/health` ni `/metrics`?** Docker llama `/health` cada 5 s y Prometheus llama `/metrics` cada 15 s. Ese tráfico taparía el real y llenaría Jaeger de trazas vacías.
- **¿Qué pasa si Jaeger está caído?** Los servicios siguen respondiendo y las trazas de ese lapso se pierden. Ningún exportador bloquea la petición; en PHP el envío tiene un plazo de 0,5 s.
- **¿Por qué la alerta de latencia llegó del Gateway y no de empleados?** Durante la prueba solo se llamó `GET /departamentos`, que va del Gateway directo a departamentos. Empleados casi no recibió tráfico y su promedio de 2 minutos no superó el umbral.
- **¿Por qué en departamentos los contadores van a un archivo?** El servidor de PHP ejecuta el script desde cero en cada petición: sin almacenamiento compartido, cada contador volvería a 0 (D15).

## 4. Problemas comunes

| Síntoma | Causa y solución |
|---|---|
| Un target en `DOWN` con `no such host` justo después de arrancar o reiniciar la máquina | El contenedor está apagado. `docker compose up -d` |
| `Bind for 0.0.0.0:3000 failed: port is already allocated` | Otro programa usa el puerto (en la máquina de desarrollo era otro proyecto en Docker). Detenerlo o definir `GRAFANA_PORT` en `.env` |
| Los paneles de latencia tienen huecos | Sin tráfico no hay promedio (`0/0`). Generar tráfico (sección 2) |
| El dashboard no aparece | Grafana carga los archivos de `observability/grafana/` al arrancar y cada 30 s. `docker compose restart grafana` y revisar `docker logs grafana` |
| Jaeger no muestra un servicio | Aún no recibió tráfico, o tiene `OTEL_SDK_DISABLED=true`. Revisar `docker compose config <servicio>` |
| No llega el correo de una alerta | La regla necesita sostenerse (1 min la caída, 2 min la latencia y los 5xx). Ver el estado en Grafana → Alerting → Alert rules |
| Una alerta de latencia sigue disparada después de la prueba | Quedó el caos encendido. `docker compose up -d departamentos-service` |
