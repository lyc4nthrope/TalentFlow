# TalentFlow

Sistema de onboarding y offboarding de empleados basado en una arquitectura orientada a microservicios.

## Arquitectura (Reto 3 — API Gateway + Circuit Breaker)

Desde el Reto 3 el sistema tiene un **único punto de entrada**. Ningún microservicio ni base de datos publica puerto al host: todos usan `expose` (solo visibles dentro de la red de Docker). El único servicio con `ports:` es `api-gateway`.

```
                         Cliente HTTP (curl, Postman, Bruno)
                                       │
                                única URL base
                                       ▼
                         ┌─────────────────────────┐
                         │   api-gateway  :8080     │  ← único puerto publicado al host
                         │  (Node.js + Express +    │
                         │   http-proxy-middleware) │
                         └────────────┬─────────────┘
                       /empleados/*   │   /departamentos/*
                    ┌──────────────────┴──────────────────┐
                    ▼                                     ▼
         empleados-service :8081                departamentos-service :8082
         (Node.js + Express)                     (PHP nativo)
                    │   HTTP REST + Circuit Breaker (opossum)
                    └────────────────────────────────────► departamentos-service
                    │                                     │
                    ▼                                     ▼
         database-empleados :5432                database-departamentos :3306
         (PostgreSQL, expose interno)             (MySQL, expose interno)
                    │                                     │
                    ▼                                     ▼
              vol-empleados                         vol-departamentos
```

| Servicio | Lenguaje | Puerto interno | Puerto al host |
|---|---|---|---|
| **api-gateway** | Node.js + Express + `http-proxy-middleware` | 8080 | **8080 (único)** |
| empleados-service | Node.js + Express | 8081 | — (`expose` interno) |
| departamentos-service | PHP 8.2 (servidor nativo) | 8082 | — (`expose` interno) |
| database-empleados | PostgreSQL 16 | 5432 | — (`expose` interno) |
| database-departamentos | MySQL 8.0 | 3306 | — (`expose` interno) |

`empleados-service` no accede a la base de datos de `departamentos-service` ni viceversa: cada servicio solo habla con su propia base de datos y con el otro servicio por HTTP, siempre a través de su nombre de servicio en la red de Docker (nunca `localhost`, nunca IPs fijas).

### El Gateway: por qué un Gateway de aplicación (no Nginx/Traefik)

Se evaluaron dos familias: un **enrutador declarativo** (Nginx, Traefik) y un **Gateway de aplicación** (código propio). Se eligió el segundo, en Node.js + Express + `http-proxy-middleware`, por la trayectoria del curso, no por preferencia entre sí:

- **Reto 5**: el Gateway debe validar el JWT y aplicar reglas de autorización — lógica que un enrutador declarativo sin plugins no puede ejecutar.
- **Reto 10**: debe validar tokens por JWKS y propagar la identidad como cabeceras a los servicios internos.
- **Proyecto final**: contempla componer respuestas combinando empleado + perfil, algo que requiere código, no solo enrutamiento.

Ninguna de las tres es natural en un enrutador declarativo. Traefik aparecerá recién en el Reto 11, como balanceador de carga delante de este mismo Gateway.

### Tabla de rutas del Gateway

Enrutamiento **exactamente** el exigido por el Reto 3 — ninguna ruta adicional (ni `/docs`, ni `/openapi.json`: la documentación Swagger de cada servicio dejó de ser alcanzable desde fuera; ver los README de cada microservicio):

| Ruta externa | Servicio interno | Notas |
|---|---|---|
| `GET /health` | (el propio Gateway) | Health check propio del Gateway (no hace proxy) + estado de cada servicio, su base de datos y el Circuit Breaker — ver [manual del Reto 3](docs/reto-03/README.md#3-dónde-veo-el-estado-de-cada-cosa) |
| `/empleados/*` | `http://empleados-service:8081` | Cuerpo, cabeceras y código de estado se reenvían sin alterar |
| `/departamentos/*` | `http://departamentos-service:8082` | Ídem |
| cualquier otra ruta | — | `404` en JSON (`{"error", "message", "path"}`) del propio Gateway, no la página por defecto de Express |

**URL base del sistema (desde el Reto 3 en adelante): `http://localhost:8080`**

Si el servicio destino no responde (caído, timeout de conexión), el Gateway responde `503` con un cuerpo JSON descriptivo (`{"error", "message", "path"}`), nunca la página de error por defecto del framework proxy. Verificado: con `http-proxy-middleware@3`, el hook de error es `on: { error }`, no el `onError` plano de la v2 — con la sintaxis vieja el error nunca se intercepta y el cliente recibe un `504` en texto plano de la librería en vez del `503` JSON exigido.

## Reto 4 — Eventos asincrónicos (perfiles, notificaciones, vacaciones)

```
 empleados-service ──publica──▶  ┌──────────────────────────────┐
 (empleado.*)                    │ message-broker (RabbitMQ)    │
 vacaciones-service ─publica──▶  │ exchange topic talentflow.events │
 (vacaciones.programadas)        └───┬───────────┬──────────┬───┘
                      empleado.*     │           │          │ empleado.creado/retirado
                                     ▼           ▼          ▼ vacaciones.programadas
                              perfiles-service  notificaciones-service  vacaciones-service
                              (Go)              (Java + Spring Boot)    (Python + FastAPI)
                              db-perfiles       db-notificaciones       db-vacaciones
```

Los tres servicios entran **detrás del Gateway** (`expose`, nunca `ports`): `/perfiles`, `/notificaciones`, `/vacaciones`.

### Servicio ↔ lenguaje

| Servicio | Lenguaje | Puerto | Base de datos | Rol |
|---|---|---|---|---|
| api-gateway | Node.js | 8080 | — | entrada única |
| empleados-service | Node.js | 8081 | PostgreSQL | REST |
| departamentos-service | PHP | 8082 | MySQL | REST |
| **perfiles-service** | **Go** | 8083 | PostgreSQL | consume eventos + REST |
| **notificaciones-service** | **Java (Spring Boot)** | 8084 | PostgreSQL | solo consume eventos |
| **vacaciones-service** | **Python (FastAPI)** | 8085 | PostgreSQL | REST + produce eventos |

5 lenguajes distintos (el reto pide ≥ 4). Elección: Python para vacaciones (aritmética de fechas, y el scheduler del Reto 5);
Go para perfiles (servicio ligero de eventos + REST); Java/Spring para notificaciones (`@RabbitListener` y JDBC maduros).

### Broker: por qué RabbitMQ

| | RabbitMQ | Kafka | Redis Streams | NATS |
|---|---|---|---|---|
| Modelo | exchanges + colas, routing por tema | log particionado | stream + grupos | subjects (core sin persistencia) |
| Encaja con el catálogo (`entidad.accion`) | **Sí: routing key = nombre del evento** | hay que mapear a topics | clave de stream | subject |
| Confirmaciones / reentrega | ack/nack nativos | offsets | ack + reclamar pendientes | solo en JetStream |
| UI para publicar a mano (prueba de deduplicación) | **Sí (management)** | no incluida | no | no |
| Costo operativo en Compose | bajo | alto (KRaft, memoria) | bajo | bajo |

Se eligió RabbitMQ porque (1) un exchange `topic` mapea 1:1 el catálogo, (2) la UI de administración permite publicar a mano el mismo
mensaje dos veces (verificación exigida), (3) el ack/nack y la reentrega son justo lo que exige la deduplicación, y (4) el volumen no justifica Kafka.
UI: <http://localhost:15672> (solo en loopback; usuario/clave en `.env`, por defecto `talentflow` / `talentflow_dev`). El puerto 5672 no se publica.

### Eventos

Ver [`docs/eventos.md`](docs/eventos.md) (topología, tabla implementada, política de ack, deduplicación) y el Catálogo de Eventos.

| Evento | Productor | Reacción |
|---|---|---|
| `empleado.creado` | empleados | perfiles: perfil por defecto · notificaciones: BIENVENIDA · vacaciones: alta en réplica |
| `empleado.actualizado` | empleados | perfiles: sincroniza campos replicados |
| `empleado.retirado` | empleados | perfiles: archiva · notificaciones: DESVINCULACION · vacaciones: marca RETIRADO |
| `vacaciones.programadas` | vacaciones | notificaciones: VACACIONES |

### Vacaciones: cómo se valida que el empleado existe → opción (b), réplica por eventos

`vacaciones-service` consume `empleado.creado` / `empleado.retirado` y mantiene `empleados_replica`. Se prefirió **disponibilidad y autonomía**
a consistencia inmediata, igual que la decisión del Circuit Breaker del Reto 3: RRHH puede programar vacaciones aunque `empleados-service`
esté caído, y no hay acoplamiento síncrono. **Costo aceptado:** consistencia eventual (un empleado recién creado puede tardar milisegundos en ser
válido), y los empleados creados *antes* de levantar este servicio no están en la réplica (habría que re-publicar sus eventos).
Además la réplica guarda el email de `empleado.creado`: si luego cambia (`empleado.actualizado`), vacaciones no se entera porque el catálogo no lo
lista como consumidor — ver `docs/eventos.md`.

Reglas: 4 validaciones → `400` (fechas incoherentes, pasadas, solapamiento —incluye `periodoEnConflicto`—, empleado inexistente o RETIRADO).
`DELETE` cancela (estado `CANCELADA`, no borra) solo períodos PROGRAMADA cuyo inicio no ha llegado; si no, `409`.
`diasHabiles` = lunes a viernes inclusive, sin festivos. Ojo: el catálogo ejemplifica 12 días para 15–30 mar 2026, que con L–V da 11 (el ejemplo es ilustrativo).

### Swagger UI (a través del Gateway, sin agregar rutas)

- <http://localhost:8080/perfiles/docs> · <http://localhost:8080/notificaciones/docs> · <http://localhost:8080/vacaciones/docs>

### Probar el flujo

```bash
docker compose up --build -d
bash docs/reto-04/demo.sh          # ver el encabezado del script
docker compose logs notificaciones-service | grep NOTIFICACI
```

Hasta que `empleados-service` publique eventos, el script publica `empleado.*` directo al broker. Para **la prueba de deduplicación a mano**: en la UI de RabbitMQ →
Exchanges → `talentflow.events` → *Publish message*, routing key `empleado.creado`, propiedad `message_id` y `id` del JSON iguales, publicar **dos veces** y comprobar
`GET /notificaciones/{empleadoId}` (una) y `GET /perfiles/{empleadoId}`. Pegar aquí la evidencia:

```
(pendiente: salida real de la prueba)
```

Pruebas automáticas: `vacaciones-service` (`pip install -r requirements-dev.txt && pytest`, usa PostgreSQL embebido), `perfiles-service`
(`TEST_DATABASE_URL=... go test -p 1 ./...`, requiere un PostgreSQL), `notificaciones-service` (`mvn test`, pruebas unitarias de parser y mensajes).

## Estructura

```
TalentFlow/
├── microservicios/
│   ├── api-gateway/                   # Reto 3 (Node.js + Express) — único punto de entrada
│   ├── gestion-empleados/             # Reto 1 + 2 + 3 (Node.js + Postgres, Circuit Breaker) — autónomo
│   ├── gestion-departamentos/         # Reto 2 (PHP + MySQL) — autónomo
│   ├── perfiles-service/              # Reto 4 (Go + Postgres)
│   ├── notificaciones-service/        # Reto 4 (Java Spring Boot + Postgres)
│   └── vacaciones-service/            # Reto 4 (Python FastAPI + Postgres)
├── docs/                              # Evidencia y decisiones por reto
├── docker-compose.yml                 # Orquesta todos los microservicios
└── package.json                       # npm workspaces (solo tooling de desarrollo, ver nota abajo)
```

> **Nota — no existe código compartido entre microservicios.** Hasta el Reto 1 existía un paquete `shared/` con el modelo de empleado y la clase de error, importado por `gestion-empleados`. Es un patrón de monolito (un "shared kernel" entre servicios) y se corrigió en el Reto 2: ese código ahora vive únicamente dentro de `microservicios/gestion-empleados/src/` (`errores.js` y `dominio/empleado.js`). Ningún microservicio importa código de negocio de otro ni de un paquete común — si dos servicios necesitan la misma validación, cada uno la implementa por su cuenta; la única comunicación permitida entre ellos es HTTP. `package.json` en la raíz sigue usando `npm workspaces`, pero solo como conveniencia de desarrollo (instalar una vez, correr `npm test` en todos) — ninguna dependencia real cruza de un servicio a otro, y el `Dockerfile` de cada servicio no depende del código de ningún otro.

## Flujo de trabajo con ramas (Git)

> Esto es únicamente la **estructura de ramas en Git** del equipo, no una implementación de DevOps completa — todavía no hay integración continua (CI), despliegue automático ni entornos separados por rama. Esa parte se evaluará e implementará más adelante en el proyecto final, si el curso lo requiere.

| Rama | Rol |
|---|---|
| `main` | Producción. Siempre debe reflejar el sistema funcionando y evaluado. Nadie trabaja directo aquí: solo recibe merges desde `dev`. |
| `dev` | Integración. Rama base donde se junta el trabajo de todo el equipo antes de llegar a producción. |
| `davidcr08` | Rama personal de David. |
| `DillanSnayderBuitrago` | Rama personal de Dillan. |
| `Vale292005` | Rama personal de Vale. |
| `lyc4nthrope` | Rama personal de Cristhian. |

Flujo:

1. Cada integrante trabaja en su propia rama (la que lleva su usuario de GitHub) — nunca directo en `dev` ni en `main`.
2. Al terminar una funcionalidad, se abre un Pull Request de la rama personal hacia `dev`.
3. `dev` es donde se integra y se prueba el trabajo de todos junto.
4. Cuando `dev` está estable (`docker compose up --build` y `npm test` pasando), se hace un Pull Request de `dev` hacia `main`.

Antes de empezar a trabajar cada día, actualizar la rama personal desde `dev`:

```bash
git checkout dev
git pull origin dev
git checkout <tu-rama>
git merge dev
```

## Cómo levantar todo desde cero

Un solo comando, sin pasos manuales (el esquema de cada base de datos se crea automáticamente desde `init.sql` la primera vez que el volumen está vacío):

```bash
docker compose up --build
```

Detener conservando los datos:

```bash
docker compose down
```

Detener y borrar también los volúmenes (reinicio total, útil si cambiaste el esquema en `init.sql`):

```bash
docker compose down -v
```

### Variables de entorno

`docker-compose.yml` define valores por defecto para desarrollo (usuarios, contraseñas y nombres de base de datos). Para un entorno propio, cópialos a un `.env` en la raíz y ajusta lo necesario — ver `.env.example`. Las credenciales nunca están escritas en el código, solo en variables de entorno.

## Arranque ordenado — evidencia

`depends_on` por sí solo solo espera a que el *contenedor* arranque, no a que el servicio esté listo. Por eso cada base de datos tiene un `healthcheck` (`pg_isready` / `mysqladmin ping`), `departamentos-service` tiene el suyo propio (consulta su endpoint interno `GET /health`, que también verifica la conexión a MySQL), `empleados-service` también (su `GET /health` interno verifica Postgres) y usa `depends_on: condition: service_healthy` contra **ambos**: su base de datos y `departamentos-service` — esto se agregó porque el profesor señaló, revisando el Reto 2, que faltaba esa dependencia explícita. El Gateway tiene su propio `healthcheck` (su `/health`) y sigue con `depends_on` simple (sin `condition`) hacia los dos microservicios: si arranca antes de que alguno esté listo, sus peticiones fallan con el `503` descrito arriba hasta que el servicio responde — no se cae, se degrada. Verificado en este repo: al ejecutar `docker compose up --build` desde cero, `departamentos-service` queda `(healthy)` ANTES de que `empleados-service` arranque, y `docker compose ps` muestra:

```
NAME               SERVICE                  STATUS                 PORTS
api-gateway        api-gateway              Up (healthy)           0.0.0.0:8080->8080/tcp
db-departamentos   database-departamentos   Up (healthy)           (sin publicar)
db-empleados       database-empleados       Up (healthy)           (sin publicar)
ms-departamentos   departamentos-service    Up (healthy)           (sin publicar)
ms-empleados       empleados-service        Up (healthy)           (sin publicar)
```

## Persistencia de datos — evidencia

```bash
# 1. Destruir contenedores, conservando volúmenes
docker compose down
docker compose up -d
curl http://localhost:8080/empleados/E001
# → 200, el empleado sigue existiendo: los datos viven en el volumen, no en el contenedor

# 2. Destruir también los volúmenes
docker compose down -v
docker compose up -d --build
curl http://localhost:8080/empleados/E001
# → 404, el volumen se borró y con él los datos
```

Ambos casos se probaron manualmente sobre esta rama y se comportan como arriba.

## Documentación OpenAPI (Swagger)

Desde el Reto 3, ninguno de los dos servicios publica puerto al host y el Gateway no enruta `/docs` ni `/openapi.json` (la tabla de rutas del reto exige exactamente `/empleados/*` y `/departamentos/*`, nada más). La documentación Swagger sigue existiendo en el código de cada servicio pero no es alcanzable desde fuera de la red de Docker; para verla hay que exponer el puerto del contenedor temporalmente durante desarrollo.

## Circuit Breaker (Reto 3) — `empleados-service → departamentos-service`

### Por qué

`empleados-service` ya tenía timeout y reintentos (Reto 2). Eso protege contra un fallo momentáneo, pero si `departamentos-service` lleva minutos caído, cada petición de registro sigue gastando tiempo completo en reintentar contra un servicio que ya se sabe caído — hilos, conexiones y memoria que se necesitan para atender lo que sí puede responder. El Circuit Breaker corta ese desperdicio: cuando ya sabe que la dependencia está caída, deja de intentarlo.

### Librería

**`opossum`** (Node.js) — no se implementó el patrón a mano, según lo exige el reto, porque además de la lógica de estados expone métricas del circuito que se van a necesitar en el Reto 11.

### Parámetros elegidos

| Parámetro | Valor | Justificación |
|---|---|---|
| Timeout por intento (`timeoutMs`) | 5000 ms | Valor sugerido por el reto; alineado con lo que ya usaba el Reto 2. |
| Reintentos internos (`maxReintentos`) | 3 | Heredado del Reto 2, espera fija de 200ms entre cada uno (no exponencial — ver corrección más abajo). |
| Umbral de error (`errorThresholdPercentage`) | 50% | Una de las dos alternativas sugeridas por el reto ("3-5 fallos consecutivos o 50% de una ventana de 10"). |
| Volumen mínimo (`volumeThreshold`) | 4 | **Decisión propia, no viene por defecto en `opossum` (por defecto es 0).** Sin este mínimo, el circuito abre con la primera petición fallida (1 de 1 = 100% ≥ 50%): técnicamente cumple el "50% de una ventana", pero no refleja los "3-5 fallos consecutivos" que también sugiere el reto, y como evidencia visual es mucho menos convincente (un solo salto de 0.6s a 0.005s en vez de varias peticiones lentas seguidas). Verificado en Docker real: con `volumeThreshold: 4`, las primeras 4 peticiones tardan ~0.63s cada una (reintentando de verdad) y desde la 5ª el circuito abre y responde en ~0.005s. |
| Ventana estadística (`rollingCountTimeout` / `rollingCountBuckets`) | 60000 ms / 10 cubos | **Corrección sobre el valor por defecto de `opossum` (10000 ms).** Según el entorno de Docker, con `departamentos-service` detenido cada llamada completa (timeout de conexión + 3 reintentos) puede tardar ~10s, lo mismo que la ventana por defecto: las estadísticas de una llamada caducaban antes de que terminara la siguiente, el circuito nunca juntaba las 4 llamadas del `volumeThreshold` en la misma ventana y se quedaba `CLOSED` para siempre. Observado por el equipo en Docker Desktop: con la ventana a 60s, las peticiones 1-4 tardan ~10s y desde la 5ª el circuito abre (~35 ms). En entornos donde la conexión falla al instante (~0.63s por petición, ver prueba abajo) el cambio no altera el comportamiento. |
| Timeout de reseteo (`resetTimeout`) | 30000 ms | Dentro del rango sugerido (30-60s). Coincide con el `sleep 35` que usa el propio guion de pruebas del reto (sección 3.3), confirmando que a los 35s el circuito ya tuvo oportunidad de pasar a `HALF_OPEN` y cerrar solo. |
| Timeout total de la función (`timeout` interno de opossum) | `(timeoutMs × (maxReintentos+1)) + 5000` = 25000 ms | Margen de seguridad por encima del peor caso real de los reintentos internos (~20.6s), para que el timeout de `opossum` nunca dispare antes que los reintentos terminen por sí solos. |

**Corrección de documentación:** una versión anterior de este README decía que los reintentos usaban "espera creciente (1s → 2s → 4s)". Se verificó el código (`departamentos.client.js`) y eso no es así: la espera entre reintentos es **fija, 200ms**. Se corrige aquí para que la documentación no contradiga el código.

### Estrategia de fallback: registrar como `PENDIENTE` y reconciliar al recuperarse

> **Decisión revisada.** La primera versión de este README elegía "rechazar con 503". El profesor, revisando el sistema, pidió explícitamente la otra opción que plantea el reto: que el registro no se bloquee, que quede pendiente, y que al restablecerse el servicio se revise automáticamente y se lleve a aceptado o rechazado. Se implementó así; esta sección documenta la decisión final, no la original.

Cuando `departamentos-service` no responde (circuito `OPEN`, o la llamada falla tras agotar los reintentos), `empleados-service` **registra igual** al empleado — no lo bloquea — y lo marca en un campo independiente, `validacionDepartamento = "PENDIENTE"`. La respuesta sigue siendo `201 Created`, con ese campo en el cuerpo:

```json
{ "id": "E900", "nombre": "Ana", "...": "...", "validacionDepartamento": "PENDIENTE" }
```

**Por qué un campo aparte y no el `estado` del empleado.** El pseudocódigo del reto (sección 2.4) sugiere `estado: "PENDIENTE_VALIDACION"`, pero `estado` en este sistema ya es el ciclo de vida laboral (`ACTIVO` / `EN_VACACIONES` / `RETIRADO`, desde el Reto 1). Reutilizar ese mismo campo mezclaría dos cosas distintas: si un empleado está de vacaciones y a la vez pendiente de validar su departamento, un solo campo no puede representar ambas. Se agregó `validacion_departamento` como columna independiente (`PENDIENTE` / `ACEPTADO` / `RECHAZADO`, ver `init.sql`), con el mismo significado que pide el reto pero sin chocar con un campo que ya tenía otro dueño.

**Cómo se reconcilia (lo que el reto exige explicar si se elige esta opción):**

1. `departamentos.client.js` expone `onRecuperado(callback)`, suscrito al evento `close` del Circuit Breaker de `opossum` — se dispara exactamente cuando el circuito pasa de abierto a cerrado tras una llamada de prueba exitosa.
2. `server.js` conecta ese evento a `servicio.reconciliarPendientes()`: sin intervención manual, apenas el circuito cierra.
3. `reconciliarPendientes()` (en `empleados.service.js`) trae todos los empleados con `validacionDepartamento = 'PENDIENTE'` (`repositorio.listarPendientes()`) y, por cada uno, vuelve a preguntarle a `departamentos-service` si su `departamentoId` existe:
   - Existe → `ACEPTADO`.
   - No existe → `RECHAZADO`.
   - Si el circuito se reabre a mitad de la revisión, ese empleado se deja como estaba: se reintenta en el próximo cierre.
4. Queda registrado en el log del contenedor: `🔁 Reconciliados N empleado(s) pendiente(s)`.

**Justificación de negocio** (la pregunta que el reto anticipa: ¿disponibilidad o consistencia?): aquí se prioriza **disponibilidad** — RR. HH. sigue registrando empleados aunque departamentos esté caído — aceptando la consistencia eventual de que un empleado puede quedar unos segundos u horas con un departamento sin verificar. Es una decisión de negocio válida siempre que la reconciliación sea automática y visible (por eso el log, y por eso el campo es consultable en `GET /empleados/:id`).

### Estado del circuito — diagnóstico

`GET /empleados/circuito-departamentos` (alcanzable a través del Gateway, porque cae bajo el prefijo `/empleados/*` sin agregar una ruta nueva al Gateway) devuelve el estado observable del Circuit Breaker:

```bash
curl http://localhost:8080/empleados/circuito-departamentos
# {"dependencia":"departamentos-service","estado":"CLOSED"}
```

`estado` es uno de `CLOSED` / `OPEN` / `HALF_OPEN`. Útil para demostrarle al profesor el estado sin depender de leer logs o de medir tiempos de respuesta.

### Cómo reproducir la prueba (verificado en este repo)

```bash
docker compose up --build -d

# 1. Crear un departamento con el sistema sano
curl -X POST http://localhost:8080/departamentos \
  -H "Content-Type: application/json" \
  -d '{"id":"IT","nombre":"Tecnologia"}'

# 2. Apagar departamentos-service
docker compose stop departamentos-service

# 3. Registrar empleados: YA NO se rechazan, quedan PENDIENTE con 201
for i in 1 2 3 4 5 6 7 8; do
  curl -s -X POST http://localhost:8080/empleados -H "Content-Type: application/json" \
    -d "{\"id\":\"E90$i\",\"nombre\":\"Test$i\",\"apellido\":\"A\",\"email\":\"t$i@test.com\",\"numeroEmpleado\":\"90$i\",\"cargo\":\"Dev\",\"area\":\"IT\",\"departamentoId\":\"IT\",\"fechaIngreso\":\"2026-01-01\"}" \
    -w " -> HTTP %{http_code} | %{time_total}s\n"
done
curl http://localhost:8080/empleados/circuito-departamentos
# Observado en este repo: todas responden 201 con "validacionDepartamento":"PENDIENTE".
# Las primeras ~4 tardan ~0.63s (circuito CLOSED, reintentando de verdad); desde
# la 5ª, ~0.008s (circuito OPEN). El campo PENDIENTE aparece en las 8, sin importar
# si el circuito ya estaba abierto o todavía cerrado cuando se registró cada una.

# 4. Restaurar el servicio y esperar el reseteo del circuito
docker compose start departamentos-service
sleep 32

# 5. El circuito no se autoprueba solo: necesita UNA petición real después del
#    resetTimeout para pasar de HALF_OPEN a CLOSED. Esa misma petición dispara
#    la reconciliación automática de todos los pendientes.
curl -X POST http://localhost:8080/empleados -H "Content-Type: application/json" \
  -d '{"id":"E999","nombre":"Trigger","apellido":"T","email":"trigger@test.com","numeroEmpleado":"999","cargo":"Dev","area":"IT","departamentoId":"IT","fechaIngreso":"2026-01-01"}'
curl http://localhost:8080/empleados/circuito-departamentos   # -> CLOSED
curl http://localhost:8080/empleados/E901                     # -> "validacionDepartamento":"ACEPTADO"
# Observado en este repo (verificado con un departamento real y otro inexistente
# entre los pendientes): los que sí tenían un departamento válido quedaron
# ACEPTADO, el que apuntaba a uno inexistente quedó RECHAZADO — ambos sin
# reiniciar nada manualmente, solo por la próxima petición real tras el reseteo.
```

### Evidencias y colección de pruebas

Todo en [`docs/reto-03/`](docs/reto-03/):

| Entregable | Archivo |
|---|---|
| **Manual paso a paso**: levantar el sistema, ver el estado de cada componente y ejecutar todas las pruebas del reto | [`README.md`](docs/reto-03/README.md) |
| Resultados reales de las pruebas 3.1, 3.2 y 3.3 del reto (acceso directo rechazado, salto de tiempo, recuperación automática, reconciliación) | [`pruebas.md`](docs/reto-03/pruebas.md) |
| Script que ejecuta esas pruebas de punta a punta: `bash docs/reto-03/demo.sh` | [`demo.sh`](docs/reto-03/demo.sh) |
| Colección **Bruno** con la URL base del sistema (`http://localhost:8080`) y tests por petición: `cd docs/reto-03/bruno && npx @usebruno/cli run --env Local` | [`bruno/`](docs/reto-03/bruno/) |
| Guion para grabar las capturas / video | [`guion-video.md`](docs/reto-03/guion-video.md) |

## Decisiones técnicas (Reto 2)

### 1. Motor de base de datos por servicio

Se usó un motor distinto por servicio: **PostgreSQL** para `empleados-service` y **MySQL** para `departamentos-service`, en vez de un único motor compartido.

- **Qué ganamos**: cada equipo/servicio puede elegir el motor que mejor le sirva sin acoplar decisiones de un servicio al otro (persistencia poliglota), y fuerza a que la comunicación entre servicios sea real (HTTP), nunca un JOIN entre bases de datos — evita el antipatrón de compartir base de datos entre microservicios.
- **Qué cuesta**: el equipo debe operar y conocer dos motores distintos (dos formas de hacer backup, de revisar logs, de escribir `healthcheck`, etc.) en vez de una sola.
- Elegido así porque el reto exige que `departamentos-service` esté en un lenguaje distinto (PHP) y MySQL es la pareja natural de un servicio PHP simple sin ORM, mientras que Postgres ya era la base del Reto 1.

### 2. Creación del esquema

Se usó la estrategia de **script de inicialización** (`init.sql` montado en `/docker-entrypoint-initdb.d/`) en ambos servicios, no auto-DDL de un ORM ni una herramienta de migraciones.

- Es simple, explícito y reproducible: cualquiera que clona el repo y corre `docker compose up --build` obtiene el esquema completo sin pasos manuales.
- Limitación conocida y aceptada para este reto: el script **solo se ejecuta si el volumen de datos está vacío**. Si el esquema cambia más adelante (ver Reto 32 — herramientas de migraciones), hay que borrar el volumen (`down -v`) y se pierden los datos existentes. Con el volumen de datos actual del equipo, evolucionar el esquema implica coordinar quién ejecuta `down -v` y cuándo.

### 3. Garantía de unicidad (email y numeroEmpleado en empleados; id en departamentos)

Se usan **ambas** estrategias, no solo una: **consulta previa** (para responder con un mensaje de error descriptivo y el código HTTP correcto) **y restricción real en el esquema** (`UNIQUE` en `email`/`numero_empleado` en Postgres, `PRIMARY KEY` en `id` en MySQL) como red de seguridad.

- Si dos peticiones con el mismo email llegan casi al mismo tiempo, ambas pueden pasar la consulta previa antes de que la primera termine de insertar. Sin restricción en el esquema, se insertarían las dos filas duplicadas.
- Con la restricción, la segunda inserción falla en la base de datos (`23505` en Postgres, `23000` en MySQL) y el código atrapa ese error puntual para devolver el mismo `400` descriptivo que daría la consulta previa — el resultado es correcto incluso bajo condición de carrera, y solo se paga el costo de la consulta previa en el caso feliz (que es el más común).

### Timeout y reintentos hacia `departamentos-service` (Reto 2, actualizado en el Reto 3)

`empleados-service` llama a `departamentos-service` con `timeout` de 5s por intento y hasta 3 reintentos con espera fija de 200ms entre cada uno (no exponencial). Si se agotan los reintentos, **el registro del empleado se acepta igual, marcado `validacionDepartamento: "PENDIENTE"`** (decisión revisada en el Reto 3 — ver la sección de Circuit Breaker más arriba para la justificación y cómo se reconcilia). Desde el Reto 3 esta llamada además está protegida por un **Circuit Breaker** que deja de intentar contra una dependencia que ya se sabe caída. Ver `microservicios/gestion-empleados/src/clients/departamentos.client.js`.

## Alineación con la clase de API RESTful

Tras la clase "Introducción a APIs RESTful" se auditó el proyecto contra sus diapositivas y se aplicó lo que era compatible con lo ya evaluado en el Reto 2:

- **Formato de error enriquecido**: todas las respuestas de error (ambos servicios) devuelven `{ status, error, message, timestamp, path, errors? }` en vez de solo `{ error }` — la diapositiva 39 marca el formato antiguo como "Mala Práctica".
- **Header `Location` en `201 Created`**: al crear un empleado o un departamento, la respuesta incluye `Location: /empleados/{id}` o `/departamentos/{id}` (diapositiva 21).

**Pendiente de consultar con el profesor** (la clase de REST recomienda una cosa, pero choca con lo que el enunciado del Reto 2 exige literalmente o con su flujo de pruebas — se dejó como está para no romper lo ya evaluado):

- **Código `409 Conflict` en duplicados**: la clase (diapositivas 7 y 31) usa un ejemplo casi idéntico a "email ya registrado" y dice que debe ser `409`, no `400`. El `reto2.pdf` dice explícitamente "las tres validaciones deben responder `400`". Se mantiene `400` porque es lo que exige el reto que se va a evaluar.
- **Versionado de API en la URL** (`/api/v1/...`, diapositiva 32): no se aplicó porque el flujo de pruebas del `reto2.pdf` (sección 8) usa rutas sin versión (`http://localhost:8080/empleados`).
- **Paginación, filtrado y ordenamiento en las listas** (diapositivas 36-37): no se aplicó porque cambiaría la forma de la respuesta de `GET /empleados` y `GET /departamentos` que ya se probó como arreglo plano.

## Mapa de retos

| Reto | Microservicio(s) | Estado | Qué se agregó | Cómo correr |
|------|---|---|---|---|
| 1 | gestion-empleados | ✅ Completado | POST/GET, modelo canónico, validaciones, Docker, 19 pruebas | `npm run dev:empleados` |
| 2 | gestion-empleados + gestion-departamentos | ✅ Completado | Persistencia en Postgres/MySQL, segundo servicio en PHP, comunicación HTTP con timeout/reintentos, healthchecks, OpenAPI | `docker compose up --build` |
| 3 | api-gateway + gestion-empleados + gestion-departamentos | ✅ Completado | API Gateway como único punto de entrada (`:8080`), microservicios y bases de datos sin puertos al host, Circuit Breaker (`opossum`) en `empleados → departamentos` con fallback de registro `PENDIENTE` + reconciliación automática al recuperarse | `docker compose up --build` (base: `http://localhost:8080`) |

Cada microservicio tiene su propio README con sus endpoints y configuración: [`gestion-empleados`](microservicios/gestion-empleados/README.md), [`gestion-departamentos`](microservicios/gestion-departamentos/README.md).

## Comandos comunes

Instalar dependencias (una sola vez, desde la raíz):

```bash
npm install
```

Ejecutar las pruebas de todos los servicios:

```bash
npm test
```

Correr todos los servicios (Docker):

```bash
docker compose up --build
```

## Stack del semestre

- **Node.js + Express** - servicios de negocio core (empleados, departamentos evolucionará en retos futuros)
- **PHP** - segundo servicio de negocio (departamentos), diversidad tecnológica exigida desde el Reto 2
- **Python + FastAPI** - servicios de datos y cálculos de negocio
- **Go** - infraestructura (gateway/eventos)
- **TypeScript + React** - frontend del proyecto final
