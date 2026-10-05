# TalentFlow

Sistema de onboarding y offboarding de empleados basado en una arquitectura orientada a microservicios.

## Arquitectura (Reto 4 — comunicación asincrónica por eventos)

El sistema tiene un **único punto de entrada** (el API Gateway, Reto 3) y, desde el Reto 4, **comunicación asincrónica**: los servicios publican y consumen eventos a través de un **message broker (RabbitMQ)**. Un solo evento — por ejemplo `empleado.creado` — dispara reacciones automáticas e independientes en varios servicios (*fan-out*). Ningún microservicio ni base de datos publica puerto al host: solo el Gateway (`8080`) y dos UIs de herramientas: la de administración del broker (`15672`) y la de Mailhog (`8025`, correos del bonus), ver [por qué](#puertos-publicados-además-del-gateway).

```
                          Cliente HTTP (curl, Postman, Bruno)
                                         │  única URL base
                                         ▼
                           ┌───────────────────────────┐
                           │     api-gateway  :8080     │ ← único puerto de la API
                           └─────────────┬─────────────┘
     /empleados   /departamentos  │  /perfiles   /notificaciones   /vacaciones
        ┌───────────────┬─────────┴──────┬────────────────┬───────────────┐
        ▼               ▼                ▼                ▼               ▼
  empleados :8081  departamentos   perfiles :8083  notificaciones  vacaciones :8085
  (Node.js)        :8082 (PHP)     (Python)        :8084 (Go)      (Java)
     │  │  REST + Circuit Breaker ▲      ▲  ▲             ▲  ▲          │  ▲
     │  └─────────────────────────┘      │  │             │  │          │  │
     │ publica                   consume │  │ consume     │  │ consume  │  │ consume
     │ empleado.creado/actualizado/      │  │             │  │          │  │
     │ retirado                          │  │             │  │ publica  │  │
     ▼                                   │  │             │  │ vacaciones.programadas
  ┌──────────────────────────────────────┴──┴─────────────┴──┴──────────┴──┴──┐
  │  message-broker (RabbitMQ) — exchange topic "talentflow.eventos"           │
  │  colas: perfiles.eventos · notificaciones.eventos · vacaciones.eventos     │
  └────────────────────────────────────────────────────────────────────────────┘
  Cada servicio con su propia base de datos (nunca compartida):
  db-empleados (PostgreSQL) · db-departamentos (MySQL) · db-perfiles · db-notificaciones · db-vacaciones (PostgreSQL)
```

### Servicios y lenguajes

El proyecto final exige al menos **4 lenguajes distintos**; el ecosistema ya tiene **5**. Los tres servicios del Reto 4 están en lenguajes distintos entre sí y distintos de los de empleados y departamentos, como exige el reto:

| Servicio | Lenguaje / framework | Rol | Puerto interno | Base de datos propia |
|---|---|---|---|---|
| **api-gateway** | **Node.js** + Express + `http-proxy-middleware` | Punto de entrada único | 8080 (**publicado**) | — |
| empleados-service | **Node.js** + Express | REST + **productor** de eventos | 8081 | PostgreSQL 16 |
| departamentos-service | **PHP** 8.2 (servidor nativo) | REST | 8082 | MySQL 8.0 |
| perfiles-service | **Python** 3.13 + FastAPI | **Consume** eventos + REST | 8083 | PostgreSQL 16 |
| notificaciones-service | **Go** 1.25 (`net/http`) | **Solo consume** eventos (puramente reactivo) | 8084 | PostgreSQL 16 |
| vacaciones-service | **Java** 17 + Spring Boot 3.5 | REST + **productor** de eventos (+ réplica por eventos) | 8085 | PostgreSQL 16 |
| message-broker | RabbitMQ 3.13 (management) | Mensajería | 5672 interno · UI 15672 (**publicada**) | — |
| mailhog | Mailhog 1.0.1 | Servidor de correo de pruebas (bonus: correo real por SMTP) | 1025 interno · UI 8025 (**publicada**) | — (en memoria) |

Ningún servicio accede a la base de datos de otro: se comunican solo por **HTTP a través de su nombre en la red de Docker** (nunca `localhost` ni IPs fijas) o por **eventos**.

### El Gateway: por qué un Gateway de aplicación (no Nginx/Traefik)

Se evaluaron dos familias: un **enrutador declarativo** (Nginx, Traefik) y un **Gateway de aplicación** (código propio). Se eligió el segundo, en Node.js + Express + `http-proxy-middleware`, por la trayectoria del curso, no por preferencia entre sí:

- **Reto 5**: el Gateway debe validar el JWT y aplicar reglas de autorización — lógica que un enrutador declarativo sin plugins no puede ejecutar.
- **Reto 10**: debe validar tokens por JWKS y propagar la identidad como cabeceras a los servicios internos.
- **Proyecto final**: contempla componer respuestas combinando empleado + perfil, algo que requiere código, no solo enrutamiento.

Ninguna de las tres es natural en un enrutador declarativo. Traefik aparecerá recién en el Reto 11, como balanceador de carga delante de este mismo Gateway.

### Tabla de rutas del Gateway

Enrutamiento por prefijo, sin rutas adicionales en el Gateway. La documentación Swagger de cada servicio vive **dentro de su propio prefijo** (`/empleados/docs`, `/departamentos/docs`…), así que es alcanzable a través del Gateway sin agregarle rutas nuevas (ver [Documentación OpenAPI](#documentación-openapi-swagger)):

| Ruta externa | Servicio interno | Notas |
|---|---|---|
| `GET /health` | (el propio Gateway) | Salud propia del Gateway (no hace proxy) + estado de los 5 servicios, su base de datos, el Circuit Breaker y su conexión al broker; `problemas` explica por qué el sistema está `DEGRADADO` — ver [manual del Reto 4](docs/reto-04/README.md) |
| `/empleados/*` | `http://empleados-service:8081` | Cuerpo, cabeceras y código de estado se reenvían sin alterar |
| `/departamentos/*` | `http://departamentos-service:8082` | Ídem |
| `/perfiles/*` | `http://perfiles-service:8083` | Ídem (Reto 4) |
| `/notificaciones/*` | `http://notificaciones-service:8084` | Ídem (Reto 4) |
| `/vacaciones/*` | `http://vacaciones-service:8085` | Ídem (Reto 4) |
| cualquier otra ruta | — | `404` en JSON (`{"error", "message", "path"}`) del propio Gateway, no la página por defecto de Express |

**URL base del sistema (desde el Reto 3 en adelante): `http://localhost:8080`**

Si el servicio destino no responde (caído, timeout de conexión), el Gateway responde `503` con un cuerpo JSON descriptivo (`{"error", "message", "path"}`), nunca la página de error por defecto del framework proxy. Verificado: con `http-proxy-middleware@3`, el hook de error es `on: { error }`, no el `onError` plano de la v2 — con la sintaxis vieja el error nunca se intercepta y el cliente recibe un `504` en texto plano de la librería en vez del `503` JSON exigido.

## Estructura

```
TalentFlow/
├── microservicios/
│   ├── api-gateway/                   # Reto 3 (Node.js + Express) — único punto de entrada
│   ├── gestion-empleados/             # Retos 1-4 (Node.js + Postgres, Circuit Breaker, productor de eventos)
│   ├── gestion-departamentos/         # Reto 2 (PHP + MySQL)
│   ├── gestion-perfiles/              # Reto 4 (Python + FastAPI + Postgres) — consume eventos + REST
│   ├── notificaciones/                # Reto 4 (Go + Postgres) — solo consume eventos
│   └── gestion-vacaciones/            # Reto 4 (Java + Spring Boot + Postgres) — REST + productor de eventos
├── infra/rabbitmq/                    # Reto 4: topología de eventos (exchange, colas, bindings) + su importación
├── docs/                              # Manuales, evidencia y decisiones por reto (reto-03/, reto-04/, eventos.md…)
├── docker-compose.yml                 # Orquesta todo el sistema (un solo comando)
└── package.json                       # npm workspaces de los servicios Node (solo tooling de desarrollo)
```

> **Nota — no existe código compartido entre microservicios.** Hasta el Reto 1 existía un paquete `shared/` con el modelo de empleado y la clase de error, importado por `gestion-empleados`. Es un patrón de monolito (un "shared kernel" entre servicios) y se corrigió en el Reto 2: ese código ahora vive únicamente dentro de `microservicios/gestion-empleados/src/` (`errores.js` y `dominio/empleado.js`). Ningún microservicio importa código de negocio de otro ni de un paquete común — si dos servicios necesitan la misma validación, cada uno la implementa por su cuenta; la única comunicación permitida entre ellos es HTTP o eventos (desde el Reto 4). Por la misma razón, el envelope de los eventos está implementado por separado en cada servicio (cada uno en su lenguaje), fijado por el Catálogo de Eventos y verificado con pruebas. `package.json` en la raíz sigue usando `npm workspaces`, pero solo como conveniencia de desarrollo (instalar una vez, correr `npm test` en todos) — ninguna dependencia real cruza de un servicio a otro, y el `Dockerfile` de cada servicio no depende del código de ningún otro.

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

Un solo comando, sin pasos manuales: el esquema de cada base de datos se crea desde su `init.sql` la primera vez que el volumen está vacío, y la topología del broker la importa `broker-init`. Levanta 14 contenedores (5 bases de datos, el broker, `broker-init`, que termina tras importar la topología, Mailhog, 5 servicios y el Gateway); tarda ~40 s en quedar todo `healthy` (`docker compose ps`).

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

`docker-compose.yml` define valores por defecto para desarrollo (usuarios, contraseñas y nombres de base de datos de las 5 bases, y el usuario del broker). Para un entorno propio: `cp .env.example .env` y ajustar los valores (`.env` no se versiona). Las credenciales nunca están escritas en el código, solo en variables de entorno; cada servicio arma sus URLs de conexión codificando usuario y contraseña.

## Arranque ordenado — evidencia

`depends_on` por sí solo solo espera a que el *contenedor* arranque, no a que el servicio esté listo. Por eso cada base de datos tiene un `healthcheck` (`pg_isready` / `mysqladmin ping`), `departamentos-service` tiene el suyo propio (consulta su endpoint interno `GET /health`, que también verifica la conexión a MySQL), `empleados-service` también (su `GET /health` interno verifica Postgres) y usa `depends_on: condition: service_healthy` contra **ambos**: su base de datos y `departamentos-service` — esto se agregó porque el profesor señaló, revisando el Reto 2, que faltaba esa dependencia explícita. El Gateway tiene su propio `healthcheck` (su `/health`) y sigue con `depends_on` simple (sin `condition`) hacia los dos microservicios: si arranca antes de que alguno esté listo, sus peticiones fallan con el `503` descrito arriba hasta que el servicio responde — no se cae, se degrada. Desde el Reto 4, los servicios que publican o consumen eventos (empleados, perfiles, notificaciones, vacaciones) esperan además a `broker-init` (`condition: service_completed_successfully`): la topología de eventos existe antes del primer mensaje, así ningún evento se descarta por no tener cola. Cada servicio nuevo espera también a su propia base de datos (`service_healthy`) y tiene su `healthcheck`. Verificado en este repo, desde cero (`docker compose ps`):

```
SERVICE                   STATUS                    PORTS
api-gateway               Up (healthy)              0.0.0.0:8080->8080/tcp
database-departamentos    Up (healthy)              3306/tcp, 33060/tcp
database-empleados        Up (healthy)              5432/tcp
database-notificaciones   Up (healthy)              5432/tcp
database-perfiles         Up (healthy)              5432/tcp
database-vacaciones       Up (healthy)              5432/tcp
mailhog                   Up                        1025/tcp, 0.0.0.0:8025->8025/tcp
message-broker            Up (healthy)              0.0.0.0:15672->15672/tcp (+ puertos internos)
departamentos-service     Up (healthy)              8082/tcp
empleados-service         Up (healthy)              8081/tcp
notificaciones-service    Up (healthy)              8084/tcp
perfiles-service          Up (healthy)              8083/tcp
vacaciones-service        Up (healthy)              8085/tcp
```

Solo tres mapeos al host (`0.0.0.0:…`): el Gateway, la UI del broker y la UI de Mailhog. El resto son puertos internos. `mailhog` no tiene healthcheck (ningún servicio espera por él). `broker-init` no aparece porque terminó (`Exited (0)`) tras importar la topología.

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

Ningún servicio publica puerto al host, así que cada uno sirve su Swagger **bajo su propio prefijo**, que es lo único que enruta el Gateway (no se agregan rutas al Gateway):

| Servicio | Swagger UI | Especificación |
|---|---|---|
| empleados-service | `http://localhost:8080/empleados/docs` | `/empleados/openapi.json` |
| departamentos-service | `http://localhost:8080/departamentos/docs` | `/departamentos/openapi.json` |
| perfiles-service | `http://localhost:8080/perfiles/docs` | `/perfiles/openapi.json` |
| notificaciones-service | `http://localhost:8080/notificaciones/docs` | `/notificaciones/openapi.json` |
| vacaciones-service | `http://localhost:8080/vacaciones/docs` | `/vacaciones/openapi.json` |

Hasta el Reto 3 la documentación vivía en `/docs` (fuera del prefijo) y no era alcanzable desde fuera; se movió en el Reto 4 porque la rúbrica evalúa el funcionamiento de Swagger UI. Consecuencia aceptada: `docs` y `openapi.json` no pueden usarse como id de un recurso (p. ej. `GET /empleados/docs` es la documentación, no el empleado "docs").

## Comunicación asincrónica (Reto 4)

> Manual paso a paso, estados y pruebas: [`docs/reto-04/README.md`](docs/reto-04/README.md). Evidencia: [`docs/reto-04/pruebas.md`](docs/reto-04/pruebas.md). Decisiones y aprendizajes por fase: [`docs/reto-04/PLAN.md`](docs/reto-04/PLAN.md).

### Message broker: por qué RabbitMQ

Se evaluaron las cuatro opciones del reto frente a lo que este sistema necesita:

| Criterio (lo que exige el reto) | **RabbitMQ** ✅ | Apache Kafka | Redis Streams | NATS |
|---|---|---|---|---|
| Fan-out: un evento, varios consumidores independientes | Exchange *topic* + una cola por servicio: nativo | Grupos de consumidores por *topic*: nativo | Grupos de consumidores: sí | Sujetos + JetStream: sí |
| **Publicar un mensaje a mano desde la UI** (prueba de deduplicación exigida) | **Sí**: botón *Publish message* en la UI incluida | No en la UI estándar (herramientas externas) | No tiene UI de administración propia | No tiene UI de administración propia |
| UI de administración (el reto pide configurarla) | Incluida (`rabbitmq:3-management`) | Externa (Kafka UI, AKHQ…) | Externa (RedisInsight) | Externa |
| Operación en un `docker-compose` de desarrollo | 1 contenedor liviano | Broker + KRaft/ZooKeeper, más memoria | 1 contenedor | 1 contenedor |
| Confirmaciones al publicar y *ack* manual al consumir | Sí (*publisher confirms*, `ack`/`nack` por mensaje) | Sí (acks + commit de offsets) | Sí (`XACK`) | Sí (JetStream) |
| Cuándo brilla | Mensajería entre servicios, enrutamiento flexible | Streaming de alto volumen, retención y *replay* | Si ya se usa Redis | Latencia mínima, cloud-native |

**Decisión:** RabbitMQ. Resuelve el fan-out de forma nativa (un exchange *topic* enruta cada evento a la cola de cada servicio suscrito), su UI trae la publicación manual que el reto exige para demostrar la deduplicación, y es un solo contenedor. Kafka queda sobredimensionado: su fortaleza (streaming masivo con retención y *replay*) no es un requisito aquí, y exige operar más infraestructura. Redis Streams y NATS carecen de una UI de administración integrada. Además, agregar un consumidor nuevo (el `auth-service` del Reto 5) es solo crear su cola y sus *bindings*: el productor no cambia.

### Topología

| Elemento | Valor |
|---|---|
| Exchange | `talentflow.eventos`, tipo **topic**, durable. **Routing key = `type` del evento** |
| Cola `notificaciones.eventos` | `empleado.creado`, `empleado.retirado`, `vacaciones.programadas` |
| Cola `perfiles.eventos` | `empleado.creado`, `empleado.actualizado`, `empleado.retirado` |
| Cola `vacaciones.eventos` | `empleado.creado`, `empleado.actualizado`, `empleado.retirado` (réplica local) |

Colas durables y mensajes persistentes: sobreviven a reinicios del broker. La topología es código versionado ([`infra/rabbitmq/definitions.json`](infra/rabbitmq/definitions.json)) y la importa el contenedor de un solo uso **`broker-init`** por la API de administración, antes de que arranque cualquier productor o consumidor (`depends_on: condition: service_completed_successfully`). No se usa `load_definitions` al arrancar el broker porque, en ese modo, RabbitMQ **no crea el usuario por defecto** y habría que versionar credenciales dentro del JSON.

#### Puertos publicados además del Gateway

El puerto AMQP (`5672`) es **interno** (`expose`): solo lo usan los servicios, dentro de la red de Docker. Se publica únicamente la **UI de administración** (`15672`), porque el reto exige usarla (verificar que el broker está activo y publicar mensajes a mano para la prueba de deduplicación). Credenciales por variables de entorno (`RABBITMQ_USER` / `RABBITMQ_PASS`, ver `.env.example`).

La otra excepción es **Mailhog** (bonus del reto: correo real por SMTP), por la misma razón: su UI (`8025`) es una herramienta de pruebas para ver los correos enviados, no un microservicio de negocio ni parte de la API. Su puerto SMTP (`1025`) es **interno**: solo lo usa notificaciones-service dentro de la red de Docker. Los correos no salen a ningún buzón real.

### Eventos implementados (Catálogo de Eventos)

Nombres, envelope y cargas útiles **idénticos** al Catálogo de Eventos (no se inventó ni renombró ningún campo; hay pruebas que lo verifican campo por campo). Detalle en [`docs/eventos.md`](docs/eventos.md).

```json
{ "id": "3f9b2c10-…", "type": "empleado.creado", "version": 1,
  "occurredAt": "2026-03-01T14:32:05Z", "producer": "empleados-service", "data": { … } }
```

| Evento (Catálogo) | Productor | Cuándo | Consumidores y reacción |
|---|---|---|---|
| `empleado.creado` (3.1) | empleados-service | `POST /empleados` exitoso | perfiles: crea el perfil por defecto · notificaciones: BIENVENIDA · vacaciones: registra al empleado en su réplica |
| `empleado.actualizado` (3.2) | empleados-service | `PUT /empleados/{id}` exitoso | perfiles: sincroniza nombre y email · vacaciones: actualiza el email de la réplica |
| `empleado.retirado` (3.3) | empleados-service | `DELETE /empleados/{id}` (baja lógica) | perfiles: archiva el perfil · notificaciones: DESVINCULACION · vacaciones: marca al empleado como retirado |
| `vacaciones.programadas` (3.8) | vacaciones-service | `POST /vacaciones` exitoso | notificaciones: VACACIONES (confirmación del período) |

**Publicación.** El evento se publica **después** de que la operación en la base de datos fue exitosa, con **confirmación del broker** (*publisher confirms*: "publicado" significa que RabbitMQ lo aceptó) y un tiempo máximo de espera. Si la publicación falla, se **registra el error y la operación no se revierte**, como exige el reto: la petición responde igual con éxito.

**Bienvenida y despedida.** El Catálogo de Eventos (y el Reto 5) disparan el correo de bienvenida con `usuario.creado` y el de despedida con `cuenta.desactivada`, porque son los que traen el token de activación o el motivo. Esos eventos los produce el `auth-service` del Reto 5, que todavía no existe; por eso aquí se aplica lo que pide el Reto 4: BIENVENIDA con `empleado.creado` y DESVINCULACION con `empleado.retirado`. En `notificaciones-service` es un mapa evento → notificación: cambiarlo en el Reto 5 no toca el resto del servicio.

### Deduplicación (obligatoria)

Cada consumidor tiene su tabla `eventos_procesados(id, procesado_en)` y aplica esta regla:

1. Registra el `id` del mensaje **y** aplica el efecto **en una sola transacción** de base de datos: o quedan ambos, o ninguno. (Si fueran pasos separados, una caída entre ellos dejaría un evento "procesado" sin efecto, o un efecto que se repetiría.)
2. Si el `id` ya existía, el mensaje es un **duplicado**: no se repite el efecto y se confirma igual.
3. El mensaje se confirma al broker (`ack`) **solo después del commit**. Si el servicio muere antes, RabbitMQ lo reentrega y el paso 2 evita el doble efecto.

Además, cada consumidor distingue **qué hacer con un fallo** (para no caer en el antipatrón del reintento infinito): un mensaje corrupto, de un tipo que no consume o con datos que la base rechaza **nunca** podrá procesarse → se descarta y queda en el log (la cola de mensajes muertos, DLQ, llega en el Reto 29); un fallo **transitorio** (su base de datos caída) → el mensaje vuelve a la cola tras una espera y se procesa cuando la base regresa, sin perderse.

**Evidencia** ([`docs/reto-04/pruebas.md`](docs/reto-04/pruebas.md), paso 9): el mismo `empleado.creado` publicado **dos veces** con el mismo `id` produce **una sola notificación y un solo perfil**, y los tres consumidores registran "evento duplicado descartado". Cómo repetirlo desde la UI: [manual, §5](docs/reto-04/README.md#5-deduplicación-desde-la-ui-de-rabbitmq).

### Servicios nuevos

| Servicio | Endpoints | Qué hace |
|---|---|---|
| **perfiles-service** (Python) | `GET /perfiles`, `GET /perfiles/{empleadoId}`, `PUT /perfiles/{empleadoId}` | Crea el perfil por defecto al consumir `empleado.creado` (`nombre` = nombre completo, resto vacío), sincroniza nombre y email con `empleado.actualizado` y lo **archiva** (no lo borra) con `empleado.retirado`. `PUT` es **parcial** (el propio reto envía solo algunos campos) y solo acepta los campos del perfil: `nombre`/`email` los replica empleados → `400`; perfil archivado → `409`; inexistente → `404`. Si llega `empleado.actualizado` de un empleado sin perfil (su `empleado.creado` se perdió), lo crea: el sistema se recupera solo |
| **notificaciones-service** (Go) | `GET /notificaciones`, `GET /notificaciones/{empleadoId}` | Puramente reactivo: ningún servicio lo llama por REST. Consume `empleado.creado`, `empleado.retirado` y `vacaciones.programadas`, simula el envío con el log `[NOTIFICACIÓN] Tipo: … \| Para: … \| Mensaje: "…"` y guarda el historial `{id, tipo, destinatario, mensaje, fechaEnvio, empleadoId}`. **Bonus:** además envía cada notificación como correo real por SMTP a Mailhog (`http://localhost:8025`); si el correo falla, se registra el error y la notificación queda guardada |
| **vacaciones-service** (Java) | `POST /vacaciones`, `GET /vacaciones`, `GET /vacaciones?empleadoId=`, `GET /vacaciones/{id}`, `DELETE /vacaciones/{id}` | Programa períodos (`V-2026-0042`, estado `PROGRAMADA`) y publica `vacaciones.programadas`. `DELETE` **cancela** (estado `CANCELADA`, no borra) solo si aún no ha iniciado; si ya inició o no está programado → `409` |

**Validaciones de vacaciones** (todas `400` con mensaje descriptivo):

1. **Fechas incoherentes**: `fechaFin` debe ser **posterior** a `fechaInicio` (el mismo día no lo es).
2. **Fechas en el pasado**: `fechaInicio` anterior a **hoy** en `America/Bogota` (hoy es válido).
3. **Solapamiento** con otro período `PROGRAMADA` o `EN_CURSO` del empleado: la respuesta incluye `periodoEnConflicto`. Garantizado también ante solicitudes **simultáneas**: bloqueo de la fila del empleado (`SELECT … FOR UPDATE`) + restricción `EXCLUDE USING gist` en PostgreSQL (la misma "doble garantía" que la unicidad del email en el Reto 2). Verificado: 10 solicitudes simultáneas → 1 creada, 9 rechazadas.
4. **Empleado inexistente** (o retirado).

`diasHabiles` (campo de `vacaciones.programadas`): lunes a viernes, ambos extremos incluidos, sin festivos. (El ejemplo del catálogo — 12 días del 15 al 30 de marzo de 2026 — no coincide con ninguna regla estándar: de lunes a viernes son 11.)

### Validación del empleado en vacaciones: réplica local por eventos (opción b)

El reto plantea dos opciones y pide justificar la elegida:

| | (a) Consulta REST a empleados-service | **(b) Réplica local por eventos** ✅ |
|---|---|---|
| Cómo | `GET /empleados/{id}` en cada solicitud, como el Reto 2 con departamentos | vacaciones-service consume `empleado.creado`/`actualizado`/`retirado` y mantiene su tabla `empleados_replica` |
| Si empleados-service está caído | **No se pueden programar vacaciones** (acoplamiento temporal) | **Sigue funcionando** |
| Consistencia | Inmediata | **Eventual**: un empleado recién creado tarda milisegundos en aparecer en la réplica |
| Complejidad | Baja | Mayor: un consumidor más, con deduplicación |

**Elegimos (b): disponibilidad sobre consistencia inmediata.** Es la misma decisión de negocio que el fallback `PENDIENTE` del Reto 3: RR. HH. no debe quedar bloqueado por la caída de otro servicio. Dos razones más la inclinan: el evento `vacaciones.programadas` exige el **email** del empleado (con la opción (a) serían dos dependencias síncronas por solicitud; la réplica ya lo tiene), y la réplica también sabe si el empleado fue **retirado**, así que un retirado no puede programar vacaciones. El costo — la ventana de consistencia eventual — se acepta porque la réplica se actualiza en milisegundos y es visible en los logs.

### Baja lógica y auditoría (empleados-service)

- `PUT /empleados/{id}`: modifica **solo** los campos que replica `empleado.actualizado` (`nombre`, `apellido`, `email`, `cargo`, `area`, `departamentoId`); un campo desconocido o de solo lectura modificado → `400` (evita la asignación masiva). Publica `empleado.actualizado`.
- `DELETE /empleados/{id}`: **no borra**. Cambia `estado` a `RETIRADO` y guarda `fechaRetiro` (instante UTC) y `motivoRetiro` (cuerpo opcional `{"motivo": …}`: `RENUNCIA` por defecto, `DESPIDO`, `JUBILACION`, `FIN_CONTRATO` u `OTRO`; el catálogo exige un `motivo` en `empleado.retirado`). Publica `empleado.retirado`. Es atómico: dos `DELETE` simultáneos → uno gana, el otro recibe `409`, y se publica **un** evento. Una restricción en la base de datos garantiza que `RETIRADO` ⇔ tiene fecha y motivo.
- Auditoría: `GET /empleados?estado=RETIRADO` y `GET /empleados?estado=RETIRADO&desde=2026-01-01&hasta=2026-06-30` (rango inclusivo sobre el **día** del retiro en `America/Bogota`: un retiro a las 21:00 en Bogotá es el día siguiente en UTC).

### Limitaciones conocidas

- **Evento perdido si el broker está caído al publicar.** La operación se guarda y el evento se descarta con un error en el log (lo que pide el reto: no revertir). La solución formal es el patrón **Outbox** (guardar el evento en la misma transacción y publicarlo después), fuera del alcance de este reto. perfiles-service mitiga el caso más visible: un `empleado.actualizado` posterior recrea el perfil faltante.
- **Vacaciones futuras de un empleado retirado** siguen `PROGRAMADA`: el reto no pide cancelarlas y el catálogo no define un evento de cancelación. El caso "retiro durante vacaciones" lo retoma el Reto 5.
- **Un empleado que la reconciliación del Reto 3 marca `RECHAZADO`** no genera evento: el catálogo no define uno.
- **Correo no enviado si el servidor SMTP está caído** (bonus): el error queda en el log y no se reintenta, porque reencolar el mensaje podría duplicar correos. La notificación sí queda en el historial.

### Cómo probar el flujo asincrónico

```bash
docker compose down -v && docker compose up --build -d   # volúmenes limpios
bash docs/reto-04/demo.sh                                 # los 11 pasos de la sección 6 del reto + correos del bonus
cd docs/reto-04/bruno && npx @usebruno/cli run --env Local  # colección: 21 peticiones con tests
```

Paso a paso manual, dónde ver el estado de cada componente y problemas comunes: [`docs/reto-04/README.md`](docs/reto-04/README.md).

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
| 4 | + message-broker + gestion-perfiles + notificaciones + gestion-vacaciones | ✅ Completado | RabbitMQ (fan-out por exchange topic), empleados publica `empleado.creado/actualizado/retirado` (+ `PUT`, baja lógica `DELETE` y auditoría), perfiles (Python), notificaciones (Go) y vacaciones (Java, publica `vacaciones.programadas`), deduplicación por id de mensaje en todos los consumidores, 5 lenguajes, Swagger de todos los servicios por el Gateway. Bonus: correo real por SMTP (Mailhog) | `docker compose up --build` + `bash docs/reto-04/demo.sh` |

Cada microservicio tiene su propio README con sus endpoints y configuración: [`gestion-empleados`](microservicios/gestion-empleados/README.md), [`gestion-departamentos`](microservicios/gestion-departamentos/README.md), [`gestion-perfiles`](microservicios/gestion-perfiles/README.md), [`notificaciones`](microservicios/notificaciones/README.md), [`gestion-vacaciones`](microservicios/gestion-vacaciones/README.md).

## Comandos comunes

Instalar dependencias (una sola vez, desde la raíz):

```bash
npm install
```

Ejecutar las pruebas automatizadas (165 en total, sin necesidad de Docker):

```bash
npm test                                                   # empleados (80) + api-gateway (6) — Node.js
(cd microservicios/notificaciones && go test ./...)       # notificaciones (22) — Go
(cd microservicios/gestion-perfiles && python -m venv .venv && . .venv/bin/activate \
  && pip install -r requirements-dev.txt && pytest)       # perfiles (28) — Python
(cd microservicios/gestion-vacaciones && mvn test)         # vacaciones (29) — Java
```

Correr todos los servicios (Docker):

```bash
docker compose up --build
```

## Stack del semestre

Implementado hasta el Reto 4 (5 lenguajes; el proyecto final exige al menos 4):

- **Node.js + Express** — `empleados-service` y `api-gateway`
- **PHP** — `departamentos-service` (diversidad tecnológica desde el Reto 2)
- **Python + FastAPI** — `perfiles-service`
- **Go** — `notificaciones-service`
- **Java + Spring Boot** — `vacaciones-service`
- **RabbitMQ** (mensajería), **PostgreSQL** y **MySQL** (bases de datos por servicio), **Mailhog** (servidor SMTP de pruebas)
- Previsto: **TypeScript + React** para el frontend del proyecto final
