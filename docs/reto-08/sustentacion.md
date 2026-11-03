# Reto 8 — Guion de la sustentación

Guion para explicarle el reto al profesor: **qué decir**, **qué mostrar** y **qué ejecutar**. Va en el orden de los criterios de evaluación del PDF. Todos los comandos se corren desde la raíz del repositorio. Dura unos 20 minutos, buena parte esperando a que se disparen las alertas. Por eso la caída del servicio se lanza en el minuto 10 y se explica lo demás mientras corre.

Si pregunta algo que no está aquí: [preguntas probables](README.md#3-preguntas-probables-en-la-sustentación) y [problemas comunes](README.md#4-problemas-comunes).

## 0. Antes de entrar (no se muestra)

```bash
git checkout main && git pull
docker compose up --build -d
docker compose ps                     # esperar a que los 7 servicios digan (healthy), ~40 s

TOKEN=$(curl -s -X POST localhost:8080/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@empresa.com","password":"Admin123*"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])")
for i in $(seq 1 20); do for r in empleados departamentos perfiles vacaciones; do
  curl -s -o /dev/null localhost:8080/$r -H "Authorization: Bearer $TOKEN"; done; done   # tráfico para el dashboard
```

Dejar abiertas estas pestañas:

| Pestaña | URL | Usuario |
|---|---|---|
| Prometheus | `http://localhost:9091/targets` | — |
| Grafana | `http://localhost:3000` → Dashboards → TalentFlow → *TalentFlow - Observabilidad* | `admin` / `talentflow_grafana_2026` |
| Jaeger | `http://localhost:16686` | — |
| Mailhog (correos) | `http://localhost:8025` | — |
| README en GitHub | sección *Observabilidad (Reto 8)* | — |

Comprobar antes de empezar: 7 de 7 targets en `UP` y el dashboard con datos. El `TOKEN` sirve una hora: si se vence, se repite el login.

## 1. Introducción (1 min)

**Decir:**
> "En el Reto 8 le agregamos observabilidad a los 7 microservicios. Antes, para saber si algo fallaba, había que revisar servicio por servicio. Ahora hay tres pilares: **métricas** en Prometheus, que dicen qué pasa y cuánto; **trazas** en Jaeger, que dicen dónde se va el tiempo de una petición; y **logs** en Loki, que dicen qué pasó exactamente. Los tres se ven juntos en Grafana, y si algo falla llega un correo sin que nadie esté mirando. Todo está como código: con un `docker compose up` el dashboard, las fuentes de datos y las alertas aparecen solos."

**Mostrar:** en GitHub, el README raíz → sección *Observabilidad (Reto 8)* → el **diagrama**.

**Decir, sobre el diagrama:**
> "Prometheus está en la misma red de Docker que los servicios y les pide sus métricas cada 15 segundos: es el modelo **pull**. Las trazas van al revés: cada servicio las **empuja** a Jaeger por OTLP. El `traceId` viaja en la cabecera estándar `traceparent` de W3C, por HTTP y también dentro de los mensajes de RabbitMQ. Promtail lee los logs de todos los contenedores y los envía a Loki. Grafana consulta las tres fuentes y manda las alertas por email."

## 2. Infraestructura de observabilidad (1.0)

| Mostrar | Decir |
|---|---|
| `docker-compose.yml`: servicios `prometheus`, `grafana`, `loki`, `promtail`, `jaeger` | "Los 5 están en el mismo compose, con versiones fijas y no `latest`, para que en su máquina levante exactamente lo que probamos. Todos están en la red `microservices-network`." |
| `observability/prometheus/prometheus.yml` | "Aquí están los 7 servicios como targets, por su nombre de Docker, nunca `localhost`. El Gateway es el primero. Lee sus métricas por el puerto interno 9464, porque su 8080 es público y cualquiera podría leerlas sin token." |
| Pestaña Prometheus `/targets` | "Los 7 en `UP`." |
| Carpeta `observability/` | "Toda la configuración está versionada: Prometheus, Loki, Promtail y Grafana con sus fuentes de datos, el dashboard y las alertas." |

Si pregunta por qué Prometheus está en el 9091: "El 9090 del host lo usa Jenkins desde el Reto 7. Dentro de la red, Prometheus sigue en el 9090."

## 3. Instrumentación de los microservicios (1.0)

**Ejecutar:**

```bash
docker compose exec api-gateway wget -qO- localhost:9464/metrics | grep http_requests_total | head -3
curl -s localhost:8080/health | python3 -m json.tool | grep -A7 '"checks"'
```

**Decir:**
> "Los 7 servicios publican **las mismas métricas con los mismos nombres**, aunque están en 5 lenguajes: `http_requests_total` y `http_request_duration_seconds`, con método, ruta y código de estado. Por eso una sola consulta sirve para todos. La ruta va como patrón, `/empleados/:id`, y no con cada id, para no crear una serie por cada empleado. El `/health` responde con el formato del PDF, con `checks` de base de datos y broker."

**Mostrar:** README → tabla *Librerías por microservicio*.

**Decir:**
> "Cada lenguaje usa la librería oficial de OpenTelemetry: Node, Python y Go con su SDK, y Java con Micrometer Tracing, que es lo estándar de Spring. PHP es el caso especial: el servicio no usa Composer, así que escribimos un exportador mínimo que lee el `traceparent` y envía el span a Jaeger con el mismo protocolo, OTLP. En los 7 el nombre del servicio sale de `OTEL_SERVICE_NAME`, el exportador apunta a `jaeger:4318` y la propagación es W3C. Está en el ancla `x-otel` del compose."

**Ejecutar** (crear un empleado, el onboarding):

```bash
N=$RANDOM; curl -s -X POST localhost:8080/empleados -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"id\":\"E$N\",\"nombre\":\"Ana\",\"apellido\":\"Gómez\",\"email\":\"ana.$N@empresa.com\",\"numeroEmpleado\":\"N$N\",\"cargo\":\"Analista\",\"area\":\"TI\",\"departamentoId\":\"IT\",\"fechaIngreso\":\"2026-04-01\"}"
```

**Mostrar:** Jaeger → Service `api-gateway` → Operation `POST /empleados` → *Find Traces* → abrir la traza.

**Decir:**
> "Es **una sola traza con los 7 servicios y unos 21 spans**. Entra por el Gateway, pasa a empleados, empleados consulta a departamentos y publica `empleado.creado` en RabbitMQ. De ahí la traza sigue en auth, perfiles y vacaciones, y auth publica `usuario.creado`, que consume notificaciones. No se corta en el broker porque el `traceparent` viaja en las cabeceras del mensaje. También se ven las consultas SQL con su tiempo."

## 4. Logs centralizados (1.0)

**Ejecutar:**

```bash
docker logs --tail 2 ms-auth
```

**Decir:**
> "Todos los servicios escriben JSON con el mismo formato: `timestamp`, `level`, `service`, `message` y, cuando hay una petición en curso, el `traceId` y el `spanId`."

**Mostrar:** en Jaeger, copiar el ID de la traza (arriba, junto al título). En Grafana → Explore → fuente **Loki** → escribir:

```
{project="talentflow"} |= "<traceId copiado>" | json
```

**Decir:**
> "Promtail recoge los logs de todos los contenedores por el socket de Docker y los envía a Loki. Con el `traceId` de esa traza salen los logs de la misma petición en 6 servicios. El Gateway no aparece porque no loguea las peticiones que salen bien. El `level` es una etiqueta de Loki; el `traceId` no, porque crearía un stream por cada petición, así que se busca en el texto."

**Mostrar:** abrir una línea y hacer clic en el enlace del `traceId`.

> "Desde el log se salta a la traza en Jaeger. Así se correlacionan los pilares."

Otras consultas LogQL con su resultado: README → *Logs centralizados con Loki*.

## 5. Dashboard en Grafana (0.5)

**Mostrar:** el dashboard *TalentFlow - Observabilidad*.

**Decir:**
> "Tiene las dos filas que pide el PDF y una más. **Resumen del sistema:** el estado de cada servicio con `up`, en verde o rojo, y tres números de todo el sistema: peticiones por segundo, porcentaje de errores 5xx y latencia p95. **Comportamiento del tráfico:** tasa de peticiones, latencia promedio y errores 4xx/5xx por servicio, con las consultas del PDF, más el p95. La tercera fila tiene CPU, memoria y los logs de error. Las consultas no cuentan `/health`, porque Docker lo llama cada 5 segundos y taparía el tráfico real."

**Mostrar:** `observability/grafana/dashboards/talentflow.json` y `observability/grafana/provisioning/dashboards/dashboards.yml`.

> "El dashboard es un JSON versionado en Git. Grafana lo carga solo al arrancar, sin configurar nada a mano."

## 6. Alertas y caos: servicio caído (0.5 + parte de 1.0)

**Ejecutar ya**, porque la alerta tarda unos 2 minutos:

```bash
docker compose stop departamentos-service
```

**Mostrar** mientras espera: `observability/grafana/provisioning/alerting/` → `rules.yml`, `contact-points.yml` y `policies.yml`.

**Decir:**
> "Hay 3 reglas, todas como código: **servicio caído**, si `up` está en 0 durante 1 minuto; **errores 5xx**, si pasan del 10 % durante 2 minutos; y **latencia alta**, si el promedio pasa de 2 segundos durante 2 minutos. Cada regla evalúa los 7 servicios por separado, así que el correo dice cuál falla. El canal es **email**: Grafana usa como servidor SMTP a Mailhog, que ya teníamos desde el Reto 4. Elegimos email porque no necesita tokens; un webhook de Discord o Telegram es un secreto que no podría ir en el repositorio. Para producción solo cambiaría la dirección del SMTP. La regla 4, contenedor reiniciado, no la hicimos porque necesita cAdvisor; el PDF pide mínimo 2 y tenemos 3."

**Mostrar,** en orden:
1. Prometheus `/targets`: departamentos en **`DOWN`**, *no such host*.
2. Grafana: su recuadro en **rojo**.
3. Grafana → Alerting → Alert rules: *Servicio caído* pasa de *Pending* a *Firing*.
4. Mailhog: llega **"[FIRING:1] Servicio caído departamentos-service"** (unos 2 minutos después del `stop`).

**Ejecutar:**

```bash
docker compose start departamentos-service
```

> "Al volver, llega el correo `[RESOLVED]`."

## 7. Caos: latencia inducida (parte de 1.0)

**Ejecutar:**

```bash
docker compose -f docker-compose.yml -f docker-compose.caos.yml up -d departamentos-service
for i in $(seq 1 80); do curl -s -o /dev/null localhost:8080/departamentos -H "Authorization: Bearer $TOKEN"; done
```

**Decir:**
> "El PDF sugiere meter un retardo en el código. Nosotros lo dejamos como un **interruptor apagado por defecto**: departamentos solo duerme 2,5 segundos si se levanta con `docker-compose.caos.yml`. Así la prueba se repite sin editar código y nunca queda encendida por error. Son 2,5 segundos porque superan el umbral de la alerta, 2 s, pero no llegan al timeout de 5 s con que empleados llama a departamentos: las peticiones funcionan, solo que lentas."

**Mostrar:** Grafana → panel *Latencia promedio por servicio*: departamentos y el Gateway suben a ~2,5 s. Tras ~2 minutos, en Mailhog: **"Alta latencia departamentos-service"** y **"Alta latencia api-gateway"**.

**Ejecutar:** crear un empleado con el comando de la sección 3 y abrir su traza en Jaeger.

> "La barra de departamentos ocupa casi toda la traza."

**Ejecutar** al terminar:

```bash
docker compose up -d departamentos-service    # apaga el caos
```

## 8. La pregunta del PDF: ¿qué servicio tardó más y cómo lo identificaron?

**Decir:**
> "Depende de la situación. **Sin caos**, dentro de lo que espera el cliente, el más lento es **empleados**: unos 50 de los 52 ms de crear un empleado. Abriendo su span se ve en qué se van: la llamada a departamentos, las consultas a Postgres y la confirmación del broker. El span más largo de toda la traza es **vacaciones**, unos 100 ms, pero consume el evento en segundo plano y no retrasa al cliente.
>
> **Con el caos**, el responsable es **departamentos**: de 2,5 segundos, casi todo es la espera de empleados a departamentos.
>
> Lo identificamos **cruzando los tres pilares**. La **alerta** dijo qué servicio estaba lento. El **panel de latencia** confirmó que solo departamentos, y quienes lo llaman, estaban en segundos. Y la **traza** mostró por qué: la barra de departamentos va dentro de las de empleados y el Gateway, que solo lo esperan. Las métricas dicen *qué* pasa; la traza dice *por qué*."

**Si pregunta por el p95 de 4,85 s** cuando el retardo real era 2,5 s:
> "Prometheus estima los percentiles interpolando dentro de los rangos del histograma, y 2,5 s cae en el rango de 2 a 5 s. El promedio sí es exacto. Por eso la alerta usa el promedio, como pide el PDF."

## 9. Cierre (30 s)

**Mostrar:** [`docs/reto-08/pruebas.md`](pruebas.md), con las 10 capturas, y la tabla de criterios del [manual](README.md#0-criterios-de-evaluación--dónde-se-demuestran).

**Decir:**
> "La evidencia completa, con horas y capturas, está en `pruebas.md`. Las decisiones y lo que aprendimos en cada fase, en `PLAN.md`. Y lo más importante: con todo esto instrumentado, las pruebas de los 7 servicios siguen en verde con la lógica de negocio al 100 %, y la suite E2E del Reto 6 pasa 61 de 61."

## Después

```bash
docker compose up -d departamentos-service   # por si el caos quedó encendido
docker compose ps                            # todo en (healthy)
```
