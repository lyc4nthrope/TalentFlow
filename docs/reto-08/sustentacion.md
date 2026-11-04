# Reto 8 — Guion de la sustentación

Cada sección lleva **el número del PDF** (`reto08.pdf`). Si el profesor pregunta "muéstreme el 4.3", se va directo a la sección 4.3. Cada una tiene el mismo orden:

- **Qué pide el PDF**: en una línea.
- **Decir**: la explicación corta.
- **Ejecutar**: los comandos, copiados tal cual. Todos se corren **desde la raíz del repo**.
- **Mostrar**: los clics exactos en cada interfaz.
- **Se espera**: lo que debe salir. Si no sale eso, algo está mal.

Duración: unos 25 minutos. Las alertas tardan ~2 minutos: por eso la caída del servicio (sección 7) se lanza y se explica mientras corre.

## Mapa: criterio de evaluación → sección

| # | Criterio (puntos) | Secciones de este guion |
|---|---|---|
| 1 | Infraestructura (1.0) | 2 y 3 |
| 2 | Instrumentación (1.0) | 4.1 métricas, 4.2 trazas, **4.3 `/health`** |
| 3 | Logs centralizados (1.0) | 4.4 |
| 4 | Dashboard (0.5) | 5 |
| 5 | Alertas (0.5) | 6 y 7 (7 C: base de datos caída) |
| 6 | Caos y documentación (1.0) | 1 (diagrama), 7 y 8 |

## 0. Antes de entrar (no se muestra)

```bash
git checkout main && git pull
docker compose up --build -d
docker compose ps        # esperar ~40 s: los 7 ms-* y el gateway en (healthy)
```

Login y tráfico para que el dashboard tenga datos:

```bash
TOKEN=$(curl -s -X POST localhost:8080/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@empresa.com","password":"Admin123*"}' | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])")
for i in $(seq 1 20); do for r in empleados departamentos perfiles vacaciones; do
  curl -s -o /dev/null localhost:8080/$r -H "Authorization: Bearer $TOKEN"; done; done
```

Comprobar que el token quedó guardado:

```bash
echo ${TOKEN:0:20}     # debe mostrar texto; si sale vacío, el login falló
```

> Todos los comandos van en **la misma terminal**: `$TOKEN` solo existe en la terminal donde se hizo el login. Si un comando responde `Token requerido` o `401`, el token está vacío o venció (dura **1 hora**): repetir el login.

Pestañas abiertas:

| Pestaña | URL | Usuario |
|---|---|---|
| Prometheus | `http://localhost:9091/targets` | — |
| Grafana | `http://localhost:3000` | `admin` / `talentflow_grafana_2026` |
| Jaeger | `http://localhost:16686` | — |
| Mailhog (correos) | `http://localhost:8025` | — |
| README en GitHub | sección *Observabilidad (Reto 8)* | — |

Se espera: 8 targets en `UP` en Prometheus (7 servicios + el propio Prometheus).

## 1. Diagrama y conceptos

**Qué pide el PDF:** el diagrama de arquitectura y la investigación de los conceptos en el README.

**Mostrar:** README → *Observabilidad (Reto 8)* → *Diagrama de arquitectura de observabilidad*.

**Decir:**
> "El sistema tenía 7 microservicios que eran una caja negra. Le agregamos los tres pilares: **métricas** en Prometheus (qué pasa y cuánto), **trazas** en Jaeger (dónde se va el tiempo) y **logs** en Loki (qué pasó exactamente). Grafana junta los tres y manda alertas por correo."
>
> "En el diagrama se ven los dos modelos. Prometheus usa **pull**: él le pide `/metrics` a cada servicio cada 15 s. Las trazas usan **push**: cada servicio envía sus spans a Jaeger por OTLP. Los logs también van por push: Promtail los lee de Docker y los envía a Loki."

Si pregunta por los conceptos, las respuestas cortas están en la [sección 9](#9-preguntas-probables).

## 2. Infraestructura en Docker Compose

**Qué pide el PDF:** Prometheus, Grafana, Loki, Promtail y Zipkin/Jaeger en el compose, en la misma red; Grafana con fuentes de datos aprovisionadas y volumen persistente.

**Mostrar:** `docker-compose.yml`, buscar (`Ctrl+F`) `prometheus:`, `grafana:`, `loki:`, `promtail:`, `jaeger:`.

**Decir:**
> "Los 5 están en el mismo compose, todos en la red `microservices-network`, con versiones fijas en vez de `latest` para que sea reproducible. Grafana guarda sus datos en el volumen `grafana-data`. Elegimos **Jaeger y no Zipkin** porque recibe OTLP, el protocolo propio de OpenTelemetry: los 5 lenguajes exportan igual, sin un exportador de Zipkin distinto en cada uno."

**Mostrar:** `observability/grafana/provisioning/datasources/datasources.yml`.

> "Las 3 fuentes de datos (Prometheus, Loki y Jaeger) se cargan solas al arrancar. Usan los nombres de Docker, como `http://prometheus:9090`, nunca `localhost`."

**Ejecutar** (para probar que todos están en la misma red):

```bash
docker network inspect talentflow_microservices-network --format '{{range .Containers}}{{.Name}} {{end}}' | tr ' ' '\n' | sort
```

**Se espera:** en la lista salen `prometheus`, `grafana`, `loki`, `jaeger`, `api-gateway` y los `ms-*`.

Si pregunta por qué Prometheus está en el **9091**: "El 9090 del host lo usa Jenkins desde el Reto 7. Dentro de la red sigue en el 9090."

## 3. Configuración de Prometheus

**Qué pide el PDF:** un `scrape_job` por microservicio, con nombres de Docker, y el Gateway primero.

**Mostrar:** `observability/prometheus/prometheus.yml`.

**Decir:**
> "Hay 7 jobs, uno por servicio, por su nombre de Docker. El Gateway va primero porque es la vista del cliente. Su `/metrics` está en el puerto interno **9464** y no en el 8080, porque el 8080 es público y cualquiera podría leer las métricas sin token. Vacaciones, que es Spring Boot, usa `/actuator/prometheus`; los demás, `/metrics`."

**Mostrar:** pestaña Prometheus → `http://localhost:9091/targets`.

**Se espera:** 8 filas en `UP`. La columna *Endpoint* muestra las URL con nombres de Docker:

| Job | Endpoint |
|---|---|
| api-gateway | `http://api-gateway:9464/metrics` |
| empleados-service | `http://empleados-service:8081/metrics` |
| departamentos-service | `http://departamentos-service:8082/metrics` |
| perfiles-service | `http://perfiles-service:8083/metrics` |
| notificaciones-service | `http://notificaciones-service:8084/metrics` |
| vacaciones-service | `http://vacaciones-service:8085/actuator/prometheus` |
| auth-service | `http://auth-service:8086/metrics` |
| prometheus | `http://localhost:9090/metrics` (se vigila a sí mismo) |

## 4.1 Métricas en formato Prometheus

**Qué pide el PDF:** cada servicio expone CPU, memoria, peticiones por código y latencia.

**Ejecutar:**

```bash
docker compose exec prometheus wget -qO- http://empleados-service:8081/metrics \
  | grep -E '^(process_cpu_seconds_total|process_resident_memory_bytes|http_requests_total|http_request_duration_seconds_count)' | grep -v health | head -6
```

> Se ejecuta **desde el contenedor de Prometheus** porque los servicios no publican su puerto en el host: solo el Gateway (8080) es público. Es lo mismo que hace Prometheus al raspar.

**Se espera:** líneas como `process_cpu_seconds_total 55.6`, `process_resident_memory_bytes 73183232` y `http_requests_total{method="GET",route="/empleados",status="200"} 61`.

**Decir:**
> "Los 7 servicios publican **las mismas métricas con los mismos nombres**, aunque son 5 lenguajes: `http_requests_total` y el histograma `http_request_duration_seconds`, con método, ruta y código. Así una sola consulta sirve para todos. La ruta va como patrón, `/empleados/:id`, no con cada id, para no crear una serie por empleado."

**Mostrar:** README → *Librerías por microservicio* (la tabla con la librería de cada lenguaje).

## 4.2 Trazas con OpenTelemetry

**Qué pide el PDF:** OTel en los 7 con `service.name`, exportador y propagación W3C. En Jaeger, una traza que muestre: (1) qué servicio recibió la petición, (2) a quién llamó y cuánto tardó, (3) si alguno dio error. Y el **caso difícil**: el scheduler de vacaciones.

**Mostrar:** `docker-compose.yml`, buscar `x-otel`.

**Decir:**
> "La configuración de OpenTelemetry es la misma en los 7, en el ancla `x-otel`: `OTEL_SERVICE_NAME` con el nombre del servicio, el exportador a `http://jaeger:4318` (nombre de Docker, no `localhost`) y `OTEL_PROPAGATORS=tracecontext,baggage`, que es W3C. Cada lenguaje usa la librería oficial; PHP no usa Composer, así que hicimos un exportador mínimo que habla el mismo protocolo."

### A. La traza del onboarding (crear un empleado)

**Ejecutar:**

```bash
N=$RANDOM; curl -s -X POST localhost:8080/empleados -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"id\":\"E$N\",\"nombre\":\"Ana\",\"apellido\":\"Gomez\",\"email\":\"ana.$N@empresa.com\",\"numeroEmpleado\":\"N$N\",\"cargo\":\"Analista\",\"area\":\"TI\",\"departamentoId\":\"IT\",\"fechaIngreso\":\"2026-04-01\"}"
echo; echo "Empleado: E$N  correo: ana.$N@empresa.com"
```

**Se espera:** un JSON con `"estado":"ACTIVO"` y `"validacionDepartamento":"ACEPTADO"`. Anotar el id y el correo: se usan en la parte B.

Si sale `"Token requerido para acceder a este recurso."`, `$TOKEN` está vacío: hacer el login del paso 0 **en esta terminal** y repetir. Sin el empleado creado, la parte B falla.

**Mostrar en Jaeger** (`http://localhost:16686`), paso a paso:

1. Arriba, la pestaña **Search** (la que abre por defecto).
2. Panel izquierdo → **Service**: elegir `api-gateway`.
3. **Operation**: elegir `POST /empleados`.
4. **Lookback**: `Last 15 Minutes`. Clic en el botón azul **Find Traces**.
5. A la derecha sale una lista, la más reciente arriba. Abrir la fila que dice `api-gateway: POST /empleados` con **21 Spans** y los nombres de los 7 servicios (clic en el título). Una fila de **1 Span** es una petición que el Gateway rechazó (por ejemplo, un 401 sin token): no pasó a ningún otro servicio.
6. Se abre la traza. Arriba, en la cabecera: **Services: 7**, **Total Spans: 21** y la duración total.

**Decir, señalando la cascada** (a la izquierda el servicio y la operación; a la derecha la barra de tiempo):
> "(1) **Quién la recibió:** la primera barra, `api-gateway POST /empleados`. (2) **A quién llamó y cuánto tardó:** el Gateway llama a empleados; empleados consulta Postgres, llama a `departamentos-service GET /departamentos/{id}` y publica `empleado.creado` en RabbitMQ. De ahí la traza **sigue** en auth, perfiles y vacaciones; auth publica `usuario.creado`, que consume notificaciones. Cada barra muestra su duración en milisegundos."
>
> "No se corta en el broker porque el `traceparent` viaja en las **cabeceras del mensaje** de RabbitMQ. Eso lo hicimos a mano o con la instrumentación de cada lenguaje, porque el SDK solo lo hace solo por HTTP."

**Mostrar:** clic en la barra de `departamentos-service`. Se despliega el detalle: en **Tags** se ven `http.response.status_code` = `200` y la ruta. Clic otra vez para cerrar.

**Opcional:** arriba a la derecha, el menú **Trace Timeline** → **Trace Graph** dibuja la traza como grafo de servicios.

**Copiar el traceId** para la sección 4.4: está en la **barra de direcciones**, después de `/trace/` (32 caracteres).

El punto (3), **un servicio con error**, se muestra en la sección 7, con departamentos apagado.

### B. El caso difícil: el scheduler de vacaciones

**Decir:**
> "Este flujo no empieza con una petición: el scheduler de vacaciones corre solo, cada minuto. Sin una traza padre, lo que sigue (auth suspende la cuenta, notificaciones manda el correo) saldría como trazas sueltas. El `@Scheduled` de Spring abre el **span raíz** (`task vacaciones-scheduler.ejecutar`), la publicación cuelga de él y el contexto viaja en las cabeceras del mensaje."

**Ejecutar:** se necesita una cuenta `ACTIVA`, porque auth no suspende cuentas pendientes. Se activa la del empleado de la parte A con el token de su correo de bienvenida y se programan vacaciones de 1 minuto:

```bash
EMP=E$N; EMAIL=ana.$N@empresa.com     # el empleado de la parte A
ACT=$(curl -s "localhost:8025/api/v2/search?kind=to&query=$EMAIL" | python3 -c "
import sys,json,re,quopri
for m in json.load(sys.stdin)['items']:
    b=quopri.decodestring(m['Content']['Body']).decode('utf-8','ignore')
    t=re.search(r'token[=:\s\"]+([0-9a-f-]{36})',b)
    if t: print(t.group(1)); break")
curl -s -X POST localhost:8080/auth/reset-password -H 'Content-Type: application/json' \
  -d "{\"token\":\"$ACT\",\"newPassword\":\"Clave123*\"}"; echo
INI=$(TZ=America/Bogota date -d '+1 min' +%FT%H:%M:00); FIN=$(TZ=America/Bogota date -d '+2 min' +%FT%H:%M:00)
curl -s -X POST localhost:8080/vacaciones -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"empleadoId\":\"$EMP\",\"fechaInicio\":\"$INI\",\"fechaFin\":\"$FIN\"}"; echo
```

> En Mac, `date -d` no existe: usar `date -v+1M +%FT%H:%M:00` y `date -v+2M +%FT%H:%M:00`.

**Se espera:** `Contraseña establecida correctamente` y unas vacaciones en `PROGRAMADA`. Si sale `token inválido o expirado`, el empleado de la parte A no se creó (no hubo correo de bienvenida): revisar `$TOKEN` y repetir la parte A. Al segundo 0 del minuto siguiente el scheduler las inicia, y un minuto después las termina. Mientras tanto se puede seguir con la sección 4.3.

**Mostrar en Jaeger:**

1. **Service**: `vacaciones-service`. **Operation**: `task vacaciones-scheduler.ejecutar`. **Lookback**: `Last 5 Minutes`. **Find Traces**.
2. Salen muchas filas de **1 Span**: son las pasadas del scheduler sin nada que hacer. Buscar la fila con **6 Spans** y abrirla.

**Se espera** una sola cascada:

```
vacaciones-service      task vacaciones-scheduler.ejecutar      ← span raíz, sin petición HTTP
vacaciones-service      talentflow.eventos/vacaciones.iniciadas send
notificaciones-service  process vacaciones.iniciadas
auth-service            process vacaciones.iniciadas            ← suspende la cuenta
auth-service            publish cuenta.desactivada
notificaciones-service  process cuenta.desactivada
```

Un minuto después sale otra de **5 Spans** con `vacaciones.finalizadas` → auth → `cuenta.activada` → notificaciones.

> "Es lo que pide el PDF: una sola cascada del scheduler a auth y de ahí a notificaciones. Si el contexto no se propagara, veríamos tres trazas separadas."

## 4.3 Endpoint de salud (`/health`)

**Qué pide el PDF:** cada servicio responde `GET /health` con `status`, `service` y `checks` de base de datos y broker; **200** si está `UP` y **503** si está `DOWN`.

**Decir:**
> "Los 7 servicios tienen `/health` desde los Retos 3 a 5. En este reto le agregamos el formato exacto del PDF: `status`, `service` y `checks` con `database` y `messageBroker`. Lo usan tres cosas: el healthcheck de Docker (por eso `docker compose ps` dice *healthy*), el Gateway, que junta la salud de todos, y la sustentación."

**Ejecutar** (los 7, por nombre de Docker, desde dentro de la red):

```bash
for s in api-gateway:8080 empleados-service:8081 departamentos-service:8082 perfiles-service:8083 \
         notificaciones-service:8084 vacaciones-service:8085 auth-service:8086; do
  echo "== $s"; docker compose exec grafana curl -s -w '  -> HTTP %{http_code}\n' http://$s/health
done
```

> Se usa el contenedor de **Grafana** porque está en la misma red y trae `curl`, que muestra el código HTTP aunque sea 503. Desde el host no se puede llamar `localhost:8081`: solo el Gateway publica su puerto.

**Se espera:** 7 respuestas con `"status":"UP"`, `-> HTTP 200` y `checks`. Por ejemplo:

```json
{"status":"UP","service":"empleados-service","checks":{"database":"UP","messageBroker":"UP"}, ...}
```

Dos casos especiales que conviene decir antes de que pregunte:

| Servicio | Qué tiene `checks` | Por qué |
|---|---|---|
| departamentos-service | Solo `database` | No usa RabbitMQ: no hay broker que revisar |
| api-gateway | Un `UP`/`DOWN` por cada servicio de atrás | No tiene base de datos: su salud es la de los servicios a los que enruta |
| vacaciones-service (Spring) | Igual que los demás, en `/health` | El PDF dice que Spring trae `/actuator/health`. Lo dejamos apagado y usamos un `/health` propio para que los 7 respondan **el mismo formato** |

**Ejecutar** el caso degradado (503), apagando la base de datos de perfiles:

```bash
docker compose stop database-perfiles
docker compose exec grafana curl -s -w '  -> HTTP %{http_code}\n' http://perfiles-service:8083/health
docker compose start database-perfiles      # restaurar
```

**Se espera:**

```json
{"status":"DOWN","service":"perfiles-service","checks":{"database":"DOWN","messageBroker":"UP"}, ...}  -> HTTP 503
```

### Desde fuera, por el Gateway (lo que pidió el profesor)

Los servicios no publican su puerto: desde fuera solo se llega al Gateway (`localhost:8080`). Por eso el Gateway tiene tres URL de salud, **públicas, sin token**:

| URL | Qué devuelve | Código | Para qué sirve |
|---|---|---|---|
| `GET /health` | La salud del **propio Gateway** y un resumen de los 6 | **Siempre 200** mientras el Gateway viva | Para el healthcheck de Docker. Si diera 503 cuando cae otro servicio, Docker creería que el enfermo es el Gateway |
| `GET /health/servicios` | El `/health` de **los 6 servicios a la vez**, cada uno tal cual | **200 si todos están UP; 503 si alguno no** | Para vigilar el sistema completo con una sola URL, como haría un monitor externo |
| `GET /health/{servicio}` | El `/health` de **un solo servicio**, tal cual | **El mismo 200 o 503 del servicio** | Para ver el 4.3 servicio por servicio, con su `checks` de base de datos y broker |

**Ejecutar** (todos a la vez):

```bash
curl -s localhost:8080/health/servicios | python3 -m json.tool | head -12
curl -s -o /dev/null -w 'HTTP %{http_code}\n' localhost:8080/health/servicios
```

**Se espera:** `"status": "UP"`, `checks` con los 6 en `UP`, el `/health` completo de cada uno en `servicios` y `HTTP 200`.

**Ejecutar** (uno por uno):

```bash
for s in auth-service empleados-service departamentos-service perfiles-service notificaciones-service vacaciones-service; do
  echo "== $s"; curl -s -w '  -> HTTP %{http_code}\n' localhost:8080/health/$s
done
curl -s -w '  -> HTTP %{http_code}\n' localhost:8080/health/no-existe     # nombre inválido
```

**Se espera:** cada servicio con su `checks` y `-> HTTP 200`. El nombre inválido da `404` con la lista de nombres válidos.

**Mostrar en Jaeger:** Service `api-gateway` → Operation `GET /health/servicios` → **Find Traces** → abrir la primera. Se ven **8 spans**: la petición, su handler y las **6 llamadas** del Gateway al `/health` de cada servicio, con su código.

> "El healthcheck de Docker y el scrape de Prometheus también llaman `/health`, cada 5 y 15 s, pero no dejan trazas, para no tapar las de negocio. Solo se traza cuando alguien de fuera pide `/health/servicios` o `/health/{servicio}`."

El caso degradado por el Gateway se muestra en la sección 7 C, con la base de datos de departamentos apagada.

## 4.4 Logs estructurados (Loki + Promtail)

**Qué pide el PDF:** logs JSON con `timestamp`, `level`, `service`, `traceId` y `message`; Promtail recoge los de todos los contenedores; Loki como fuente en Grafana; una consulta LogQL.

**Ejecutar:**

```bash
docker logs --tail 2 ms-auth
```

**Se espera:** líneas JSON: `{"timestamp":"...","level":"INFO","message":"...","service":"auth-service","traceId":"...","spanId":"..."}`.

**Mostrar:** `observability/promtail/promtail-config.yml`.

> "Promtail descubre los contenedores por el socket de Docker, como pide el PDF, y los envía a `http://loki:3100`. Al `level` lo convierte en etiqueta. El `traceId` **no** es etiqueta: crearía un stream nuevo por cada petición, así que se busca en el texto."

**Mostrar en Grafana**, paso a paso:

1. Menú de la izquierda (☰) → **Explore**.
2. Arriba a la izquierda, el selector de fuente de datos: elegir **Loki**.
3. A la derecha del editor, cambiar de **Builder** a **Code**.
4. Pegar la consulta, con el traceId copiado de Jaeger en la sección 4.2:

   ```
   {project="talentflow"} |= "<traceId>" | json
   ```

5. Clic en **Run query** (arriba a la derecha).

**Se espera:** logs de **6 servicios** de la misma petición. El Gateway no sale porque no loguea las peticiones que salen bien.

**Mostrar:** clic en una línea para abrirla → abajo, en **Links**, el enlace de `traceId` abre la traza de Jaeger en un panel al lado. "Así se salta del log a la traza: se correlacionan los pilares."

Otra consulta para mostrar, la de errores por servicio:

```
sum by (service) (count_over_time({project="talentflow", level="ERROR"}[5m]))
```

Más consultas con su resultado: README → *Logs centralizados con Loki*.

## 5. Dashboard en Grafana

**Qué pide el PDF:** 4 paneles (estado, tasa, latencia, errores) en dos filas, aprovisionado desde un JSON en Git.

**Mostrar:** Grafana → menú ☰ → **Dashboards** → carpeta **TalentFlow** → *TalentFlow - Observabilidad*.

**Decir:**
> "**Fila 1, Resumen del sistema:** el estado de cada servicio con `up`, en verde o rojo, y tres números del sistema: peticiones por segundo, % de errores 5xx y latencia p95. **Fila 2, Comportamiento del tráfico:** tasa de peticiones, latencia promedio y errores 4xx/5xx por servicio, con las consultas del PDF, más el p95. Agregamos una **fila 3** con CPU, memoria y logs de error. Las consultas no cuentan `/health`: Docker lo llama cada 5 s y taparía el tráfico real."

**Mostrar:** en un panel, menú ⋮ → **Edit** → se ve la consulta PromQL. Salir con **Discard** (o `Esc`).

**Mostrar:** `observability/grafana/dashboards/talentflow.json` y `observability/grafana/provisioning/dashboards/dashboards.yml`.

> "El dashboard es un JSON versionado en Git. Grafana lo carga solo al arrancar."

## 6. Alertas proactivas

**Qué pide el PDF:** al menos 2 reglas y un canal externo funcional.

**Mostrar:** `observability/grafana/provisioning/alerting/` → `rules.yml`, `contact-points.yml`, `policies.yml`.

**Decir:**
> "Cuatro reglas, como código: **servicio caído** (`up` en 0 durante 1 min), **errores 5xx** (más del 10 % durante 2 min), **latencia alta** (promedio sobre 2 s durante 2 min) y **dependencia caída** (la base de datos o el broker de un servicio en DOWN durante 1 min; se explica en la sección 7 C). Cada una evalúa los 7 servicios por separado, así que el correo dice cuál falla. El canal es **email** por **Mailhog**, que ya estaba desde el Reto 4: no necesita tokens; un webhook de Discord o Telegram sería un secreto que no puede ir en el repo. La regla 4, contenedor reiniciado, no la hicimos porque necesita cAdvisor; el PDF pide mínimo 2."

**Mostrar:** Grafana → ☰ → **Alerting** → **Alert rules**: las 4 reglas en la carpeta TalentFlow.

## 7. Simulación del caos

### A. Apagar un servicio (empezar ya: tarda ~2 min)

**Ejecutar:**

```bash
docker compose stop departamentos-service
```

Mientras llega la alerta, mostrar **el servicio con error en Jaeger** (el punto 3 del 4.2):

```bash
N=$RANDOM; curl -s -X POST localhost:8080/empleados -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"id\":\"E$N\",\"nombre\":\"Luis\",\"apellido\":\"Paz\",\"email\":\"luis.$N@empresa.com\",\"numeroEmpleado\":\"N$N\",\"cargo\":\"Analista\",\"area\":\"TI\",\"departamentoId\":\"IT\",\"fechaIngreso\":\"2026-04-01\"}"; echo
```

**Se espera:** `201` con `"validacionDepartamento":"PENDIENTE"`: el circuit breaker del Reto 3 guarda al empleado aunque departamentos no responda.

**Mostrar en Jaeger:** Service `api-gateway`, Operation `POST /empleados`, **Find Traces**, abrir la primera. Los spans `empleados-service GET` tienen un **ícono rojo de error**. Clic en uno → Tags: `error = true` y `getaddrinfo ENOTFOUND departamentos-service`.

> "Se ven **4 intentos** con error: el primero y los 3 reintentos. Después, el fallback guarda al empleado como `PENDIENTE`. La traza muestra el error **y** cómo se recuperó el sistema."

**Mostrar,** en orden:

1. Prometheus `/targets`: `departamentos-service` en **DOWN**, error *no such host*.
2. Grafana, dashboard: su recuadro en **rojo**.
3. Grafana → Alerting → Alert rules: *Servicio caído* pasa de *Pending* a *Firing*.
4. Mailhog: llega **"[FIRING:1] Servicio caído departamentos-service"** (~2 min después del `stop`).

**Ejecutar:**

```bash
docker compose start departamentos-service
```

**Se espera:** a los ~2 min, el correo **"[RESOLVED]"**.

### B. Latencia inducida

**Ejecutar:**

```bash
docker compose -f docker-compose.yml -f docker-compose.caos.yml up -d departamentos-service
for i in $(seq 1 80); do curl -s -o /dev/null localhost:8080/departamentos -H "Authorization: Bearer $TOKEN"; done
```

**Decir:**
> "El PDF sugiere meter un retardo en el código. Lo dejamos como un **interruptor apagado por defecto**: departamentos solo duerme 2,5 s si se levanta con `docker-compose.caos.yml`. Así no se edita código y nunca queda encendido por error. Son 2,5 s porque pasan el umbral de la alerta (2 s) pero no el timeout de 5 s con que empleados llama a departamentos."

**Mostrar:**

1. Dashboard → panel *Latencia promedio por servicio*: departamentos y el Gateway suben a ~2,5 s.
2. Mailhog, ~2 min después: **"Alta latencia departamentos-service"** y **"Alta latencia api-gateway"**.
3. Crear un empleado con el comando de la sección 4.2 A y abrir su traza en Jaeger: la barra de `departamentos-service` ocupa casi toda la traza.

**Ejecutar** al terminar (apaga el caos):

```bash
docker compose up -d departamentos-service
```

### C. Apagar una base de datos (lo que preguntó el profesor)

**Decir, antes de ejecutar:**
> "Una base de datos caída es distinta de un servicio caído. El proceso sigue vivo, Prometheus sigue leyendo su `/metrics` y `up` sigue en 1, así que *Servicio caído* no se entera. Prometheus no lee `/health`, solo métricas. Por eso el Gateway, que ya consulta el `/health` de todos, lo publica como una métrica, `talentflow_health_check`: 1 si la dependencia está UP y 0 si no, por servicio y por dependencia. Sobre esa métrica hay una alerta, **Dependencia caída**, y un panel en el dashboard."

**Ejecutar:**

```bash
docker compose stop database-departamentos
sleep 20
curl -s -w '  -> HTTP %{http_code}\n' localhost:8080/health/departamentos-service
curl -s -o /dev/null -w '/health/servicios -> HTTP %{http_code}\n' localhost:8080/health/servicios
```

**Se espera:**

```
{"status":"DOWN","service":"departamentos-service","checks":{"database":"DOWN"}, ...}  -> HTTP 503
/health/servicios -> HTTP 503
```

**Mostrar,** en orden (unos 90 s en total):

1. **Prometheus** `/targets`: departamentos sigue en **UP**. "Por eso hacía falta otra métrica."
2. **Prometheus** → arriba, **Query** → escribir `talentflow_health_check{service="departamentos-service"}` → **Execute**: `check="database"` vale **0** y `check="status"` vale **0**.
3. **Grafana**, dashboard: el panel **Dependencias de cada servicio (BD y broker)** muestra `departamentos-service · database` en **rojo**. El recuadro de arriba, *Estado de cada servicio*, sigue verde: el proceso está vivo.
4. **Grafana** → Alerting → Alert rules: *Dependencia caída* pasa de *Pending* a *Firing*.
5. **Mailhog**: llega **"[FIRING:1] Dependencia caída database … departamentos-service"** (~1 min 30 s después del `stop`). El correo dice qué dependencia y de qué servicio, y a qué URL mirar.
6. **Jaeger**: la traza de `GET /health/servicios` tiene la llamada a `departamentos-service` en **rojo** con 503.

**Ejecutar** para restaurar:

```bash
docker compose start database-departamentos
```

**Se espera:** en ~1 min, el correo **"[RESOLVED] Dependencia caída …"** y el panel otra vez verde.

Si pregunta por qué no se usó la alerta de errores 5xx:
> "Antes de este cambio llegaba un correo *Alta tasa de errores 5xx*, pero por casualidad: el healthcheck de Docker llama `/health` cada 5 s y cada 503 contaba como error. El motivo del correo era equivocado. Ahora la regla de 5xx no cuenta `/health`, y la base de datos caída tiene su propia alerta con el nombre correcto."

## 8. La pregunta: ¿qué servicio tardó más y cómo lo identificaron?

**Decir:**
> "**Sin caos**, el más lento para el cliente es **empleados**: unos 50 de los 52 ms de crear un empleado, entre la llamada a departamentos, Postgres y el broker. El span más largo es **vacaciones** (~100 ms), pero consume el evento en segundo plano y no retrasa al cliente.
>
> **Con el caos**, es **departamentos**: de 2,5 s, casi todo es la espera de empleados a departamentos.
>
> Lo identificamos **cruzando los tres pilares**: la **alerta** dijo qué servicio estaba lento; el **panel de latencia** confirmó que solo departamentos y quienes lo llaman; la **traza** mostró por qué: la barra de departamentos va dentro de las de empleados y el Gateway, que solo lo esperan."

**Cierre:** mostrar [`pruebas.md`](pruebas.md) con las 10 capturas.

> "La evidencia, con horas y capturas, está en `pruebas.md`; las decisiones, en `PLAN.md`. Con todo esto instrumentado, las pruebas de los 7 servicios siguen en verde y la suite E2E del Reto 6 pasa 61 de 61."

## 9. Preguntas probables

| Pregunta | Respuesta corta |
|---|---|
| ¿Cuál es el rol de cada pilar? | Métricas: **qué** pasa y cuánto (dashboards, alertas). Trazas: **dónde** se va el tiempo de una petición. Logs: **qué pasó exactamente**. La métrica avisa, la traza ubica, el log explica. |
| Pull vs. push | **Pull**: Prometheus pide `/metrics` cada 15 s; si no responde, `up = 0` y así sabe que está caído. **Push**: el servicio envía cada span a Jaeger cuando termina; un span es un evento único y si no se envía, se pierde. |
| ¿Qué es la CNCF? ¿Por qué OpenTelemetry? | La *Cloud Native Computing Foundation* (Linux Foundation) gobierna Kubernetes, Prometheus, Jaeger y OpenTelemetry, para que no dependan de un proveedor. OTel es **agnóstico**: 5 lenguajes, un mismo formato y protocolo (OTLP). Cambiar Jaeger por otro backend es cambiar una URL. |
| ¿Cómo viaja el traceId? (W3C) | En la cabecera `traceparent: 00-<traceId 32 hex>-<spanId 16 hex>-01`. Quien llama la inyecta; quien recibe la extrae y crea su span como hijo. Al ser estándar, Node, Go, PHP, Python y Java se entienden. En RabbitMQ va en las cabeceras del mensaje. |
| ¿Por qué Jaeger y no Zipkin? | Jaeger recibe OTLP nativo; Zipkin pediría su exportador en cada lenguaje. Es graduado de la CNCF y su UI muestra la cascada y el grafo. Versión 2.20.0 fija: la 2.21 quitó un endpoint que usa Grafana. |
| ¿Por qué no `latest`? | Reproducibilidad: en otra máquina levanta exactamente lo que probamos. |
| ¿Qué es un span? | Una operación con inicio y duración (una petición HTTP, una consulta SQL, un mensaje). Una traza es el árbol de spans de una petición. |
| ¿Por qué no se cuenta `/health` ni `/metrics`? | Docker llama `/health` cada 5 s y Prometheus `/metrics` cada 15 s. Taparían el tráfico real y llenarían Jaeger de trazas vacías. |
| ¿Por qué `route` es un patrón? | `/empleados/:id` en vez de `/empleados/E001`: con el id habría una serie por empleado (cardinalidad) y Prometheus se llenaría. |
| ¿Por qué el p95 dio 4,85 s si el retardo era 2,5 s? | Prometheus estima el percentil interpolando dentro del rango del histograma (2 a 5 s). El promedio sí es exacto: por eso la alerta usa el promedio, como pide el PDF. |
| ¿Muestreo? | 100 %, para desarrollo. En producción se muestrearía por cabecera. |
| ¿Cómo lo instrumentaron sin romper el 100 % de cobertura? | La instrumentación vive en el arranque y en los adaptadores HTTP y de mensajería, nunca en el dominio (D7). |

Más preguntas y problemas comunes: [manual del reto](README.md#3-preguntas-probables-en-la-sustentación).

## Después

```bash
docker compose up -d departamentos-service database-perfiles database-departamentos   # por si algo quedó apagado o con caos
docker compose ps                                              # todo en (healthy)
```
