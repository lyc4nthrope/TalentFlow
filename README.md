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
| `GET /health` | (el propio Gateway) | Health check propio del Gateway, no hace proxy |
| `/empleados/*` | `http://empleados-service:8081` | Cuerpo, cabeceras y código de estado se reenvían sin alterar |
| `/departamentos/*` | `http://departamentos-service:8082` | Ídem |
| cualquier otra ruta | — | `404` (no enrutada) |

**URL base del sistema (desde el Reto 3 en adelante): `http://localhost:8080`**

Si el servicio destino no responde (caído, timeout de conexión), el Gateway responde `503` con un cuerpo JSON descriptivo (`{"error", "message", "path"}`), nunca la página de error por defecto del framework proxy. Verificado: con `http-proxy-middleware@3`, el hook de error es `on: { error }`, no el `onError` plano de la v2 — con la sintaxis vieja el error nunca se intercepta y el cliente recibe un `504` en texto plano de la librería en vez del `503` JSON exigido.

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

`depends_on` por sí solo solo espera a que el *contenedor* arranque, no a que el servicio esté listo. Por eso cada base de datos tiene un `healthcheck` (`pg_isready` / `mysqladmin ping`), `departamentos-service` tiene el suyo propio (`fsockopen` a su propio puerto), y `empleados-service` usa `depends_on: condition: service_healthy` contra **ambos**: su base de datos y `departamentos-service` — esto se agregó porque el profesor señaló, revisando el Reto 2, que faltaba esa dependencia explícita. El Gateway sigue con `depends_on` simple (sin `condition`) hacia los dos microservicios: si arranca antes de que alguno esté listo, sus peticiones fallan con el `503` descrito arriba hasta que el servicio responde — no se cae, se degrada. Verificado en este repo: al ejecutar `docker compose up --build` desde cero, `departamentos-service` queda `(healthy)` ANTES de que `empleados-service` arranque, y `docker compose ps` muestra:

```
NAME               SERVICE                  STATUS                 PORTS
api-gateway        api-gateway              Up                     0.0.0.0:8080->8080/tcp
db-departamentos   database-departamentos   Up (healthy)           (sin publicar)
db-empleados       database-empleados       Up (healthy)           (sin publicar)
ms-departamentos   departamentos-service    Up (healthy)           (sin publicar)
ms-empleados       empleados-service        Up                     (sin publicar)
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
| Timeout de reseteo (`resetTimeout`) | 30000 ms | Dentro del rango sugerido (30-60s). Coincide con el `sleep 35` que usa el propio guion de pruebas del reto (sección 3.3), confirmando que a los 35s el circuito ya tuvo oportunidad de pasar a `HALF_OPEN` y cerrar solo. |
| Timeout total de la función (`timeout` interno de opossum) | `(timeoutMs × (maxReintentos+1)) + 5000` = 25000 ms | Margen de seguridad por encima del peor caso real de los reintentos internos (~20.6s), para que el timeout de `opossum` nunca dispare antes que los reintentos terminen por sí solos. |

**Corrección de documentación:** una versión anterior de este README decía que los reintentos usaban "espera creciente (1s → 2s → 4s)". Se verificó el código (`departamentos.client.js`) y eso no es así: la espera entre reintentos es **fija, 200ms**. Se corrige aquí para que la documentación no contradiga el código.

### Estrategia de fallback: rechazar con `503`

Cuando el circuito está `OPEN`, `empleados-service` responde `503` con `{"message": "No fue posible verificar el departamento, el servicio no resolvió"}` — **rechaza el registro**, no lo acepta como "pendiente de validación".

Se eligió esta opción (de las tres que plantea el reto) porque:
- El modelo canónico de empleado (Reto 1) no tiene un estado para "pendiente de validación"; agregarlo solo para este caso introduciría un estado que el resto del sistema (listar, consultar por id) tendría que empezar a tratar como caso especial.
- Aceptar un `departamentoId` sin verificar puede dejar datos inconsistentes que alguien tendría que reconciliar más tarde — y el reto exige explicar cómo se reconciliaría si se elige esa opción, lo cual habría sido una responsabilidad nueva sin dueño claro en el equipo.
- Es la opción más conservadora frente a la pregunta que el propio reto anticipa que va a repetirse ("¿prefiere disponibilidad o consistencia?", Retos 4 y 12): aquí se prioriza consistencia porque el dato de un empleado es sensible (RR. HH.), no un contador ni una métrica.
- No requiere reconciliación de estado porque nunca se guarda un registro a medias.

### Cómo reproducir la prueba (verificado en este repo)

```bash
docker compose up --build -d

# 1. Crear un departamento con el sistema sano
curl -X POST http://localhost:8080/departamentos \
  -H "Content-Type: application/json" \
  -d '{"id":"IT","nombre":"Tecnologia"}'

# 2. Apagar departamentos-service
docker compose stop departamentos-service

# 3. Registrar empleados y observar el tiempo de respuesta de cada uno
for i in 1 2 3 4 5 6 7 8; do
  curl -s -o /dev/null -w "Petición $i -> HTTP %{http_code} | %{time_total}s\n" \
    -X POST http://localhost:8080/empleados -H "Content-Type: application/json" \
    -d "{\"id\":\"E90$i\",\"nombre\":\"Test$i\",\"apellido\":\"A\",\"email\":\"t$i@test.com\",\"numeroEmpleado\":\"90$i\",\"cargo\":\"Dev\",\"area\":\"IT\",\"departamentoId\":\"IT\",\"fechaIngreso\":\"2026-01-01\"}"
done
# Observado en este repo: peticiones 1-4 ~0.63s (circuito CLOSED, reintentando de verdad),
# peticiones 5-8 ~0.005s (circuito OPEN, fallback instantáneo sin tocar la red).

# 4. Restaurar el servicio y esperar el reseteo del circuito
docker compose start departamentos-service
sleep 35

# 5. Verificar la recuperación automática con un departamento inexistente
curl -X POST http://localhost:8080/empleados -H "Content-Type: application/json" \
  -d '{"id":"E999","nombre":"Recuperado","apellido":"T","email":"r@test.com","numeroEmpleado":"999","cargo":"Dev","area":"IT","departamentoId":"NO-EXISTE","fechaIngreso":"2026-01-01"}'
# Observado en este repo: HTTP 400 "El departamento NO-EXISTE no existe" — es decir,
# volvió a consultar de verdad a departamentos-service, sin reiniciar nada manualmente.
```

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

`empleados-service` llama a `departamentos-service` con `timeout` de 5s por intento y hasta 3 reintentos con espera fija de 200ms entre cada uno (no exponencial). Si se agotan los reintentos, **el registro del empleado se rechaza** (no se acepta como "pendiente de validación"): el modelo canónico no tiene un estado para eso, y aceptar un `departamentoId` sin verificar podría dejar datos inconsistentes. Se responde `503` (no `400`): la falla es de una dependencia caída, no un error del cliente. Desde el Reto 3 esta llamada además está protegida por un **Circuit Breaker** (ver sección dedicada más abajo) que deja de intentar contra una dependencia que ya se sabe caída. Ver `microservicios/gestion-empleados/src/clients/departamentos.client.js`.

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
| 3 | api-gateway + gestion-empleados + gestion-departamentos | ✅ Completado | API Gateway como único punto de entrada (`:8080`), microservicios y bases de datos sin puertos al host, Circuit Breaker (`opossum`) en `empleados → departamentos` con fallback de rechazo `503` | `docker compose up --build` (base: `http://localhost:8080`) |

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
