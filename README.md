# TalentFlow

Sistema de onboarding y offboarding de empleados basado en una arquitectura orientada a microservicios.

## Arquitectura (Reto 2)

```
                    Cliente HTTP (curl, Postman, Bruno)
                           │
              ┌────────────┴────────────┐
              ▼                         ▼
     empleados-service :8080   departamentos-service :8081
     (Node.js + Express)       (PHP nativo)
              │  HTTP REST (timeout + reintentos)
              └───────────────► departamentos-service
              │                         │
              ▼                         ▼
     database-empleados :5432   database-departamentos :5433
     (PostgreSQL, healthcheck)  (MySQL, healthcheck)
              │                         │
              ▼                         ▼
        vol-empleados              vol-departamentos
```

| Servicio | Lenguaje | Motor de BD | Puerto (host) |
|---|---|---|---|
| empleados-service | Node.js + Express | PostgreSQL 16 | 8080 |
| departamentos-service | PHP 8.2 (servidor nativo) | MySQL 8.0 | 8081 |
| database-empleados | — | PostgreSQL 16 | 5432 |
| database-departamentos | — | MySQL 8.0 | 5433 (host) → 3306 (contenedor) |

`empleados-service` no accede a la base de datos de `departamentos-service` ni viceversa: cada servicio solo habla con su propia base de datos y con el otro servicio por HTTP.

## Estructura

```
TalentFlow/
├── microservicios/
│   ├── gestion-empleados/             # Reto 1 + Reto 2 (Node.js + Postgres) — autónomo
│   └── gestion-departamentos/         # Reto 2 (PHP + MySQL) — autónomo
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

`depends_on` por sí solo solo espera a que el *contenedor* arranque, no a que el servicio esté listo. Por eso cada base de datos tiene un `healthcheck` (`pg_isready` / `mysqladmin ping`) y cada microservicio usa `depends_on: condition: service_healthy`. Verificado en este repo: al ejecutar `docker compose up --build` desde cero, los logs de `empleados-service` y `departamentos-service` no reportan errores de conexión, y `docker compose ps` muestra:

```
NAME               SERVICE                  STATUS
db-departamentos   database-departamentos   Up (healthy)
db-empleados       database-empleados       Up (healthy)
ms-departamentos   departamentos-service    Up
ms-empleados       empleados-service        Up
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

| Servicio | Swagger UI | Especificación |
|---|---|---|
| empleados-service | http://localhost:8080/docs | http://localhost:8080/openapi.json |
| departamentos-service | http://localhost:8081/docs | http://localhost:8081/openapi.json |

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

### Timeout y reintentos hacia `departamentos-service` (punto 6)

`empleados-service` llama a `departamentos-service` con `timeout` de 2s y hasta 3 reintentos con espera creciente (1s → 2s → 4s). Si se agotan los reintentos, **el registro del empleado se rechaza** (no se acepta como "pendiente de validación"): el modelo canónico no tiene un estado para eso, y aceptar un `departamentoId` sin verificar podría dejar datos inconsistentes. Se responde `503` (no `400`): la falla es de una dependencia caída, no un error del cliente. Ver el comentario en `microservicios/gestion-empleados/src/clients/departamentos.client.js`.

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
| 2 | gestion-empleados + gestion-departamentos | ✅ Funcional — documentación en revisión por el equipo | Persistencia en Postgres/MySQL, segundo servicio en PHP, comunicación HTTP con timeout/reintentos, healthchecks, OpenAPI | `docker compose up --build` |

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
