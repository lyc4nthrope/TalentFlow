# Reto 4 — Plan de implementación

Fuente de requisitos: `reto4.pdf` + **Catálogo de Eventos** (normativo) + anticipación del `reto5.pdf` (sin implementarlo).
Cada fase termina **verificada** (pruebas + comprobación real en Docker) y con su propio commit.

## Decisiones de diseño

| # | Decisión | Elección | Por qué |
|---|---|---|---|
| D1 | Message broker | **RabbitMQ** (`rabbitmq:3-management`) | Exchange *topic* = fan-out natural con una cola por consumidor; nuevos consumidores (auth-service, Reto 5) se agregan sin tocar al productor. Su UI permite **publicar mensajes a mano**, que es como el reto exige probar la deduplicación. Kafka: sobredimensionado (streaming, ZooKeeper/KRaft) y sin publicación manual en la UI estándar. Redis Streams / NATS: sin UI de administración equivalente integrada |
| D2 | Lenguajes | **vacaciones → Java 17 (Spring Boot)** · **perfiles → Python (FastAPI)** · **notificaciones → Go** | Vacaciones tiene el dominio más rico (4 validaciones, estados, scheduler en el Reto 5 con `@Scheduled`). Perfiles es CRUD + consumidor: FastAPI genera OpenAPI solo. Notificaciones es un consumidor puro y liviano: Go. Ecosistema final: Node, PHP, Java, Python, Go = **5 lenguajes** (el proyecto exige ≥ 4) |
| D3 | Topología | Exchange `talentflow.eventos` (topic, durable). Routing key = `type` del evento. Una cola durable por servicio: `notificaciones.eventos`, `perfiles.eventos`, `vacaciones.eventos` | Cada servicio recibe su copia (fan-out) y solo los eventos que le interesan (bindings). Mensajes persistentes |
| D4 | Envelope | Exactamente el del catálogo: `id` (UUID del mensaje), `type`, `version: 1`, `occurredAt` (UTC ISO-8601), `producer`, `data` | Contrato normativo; el Reto 5 construye sobre él |
| D5 | Deduplicación | Tabla `eventos_procesados(id PK, procesado_en)` en la BD de **cada** consumidor; registro del id **y** efecto en la **misma transacción**; `ack` solo después del commit | Si el efecto se aplica pero el registro no (o al revés), la deduplicación falla. En una transacción, o pasan ambos o ninguno |
| D6 | Publicación | Después del commit en BD; si falla, se registra el error y **no** se revierte (lo exige el reto) | Limitación conocida: el evento puede perderse. Se documenta (Outbox Pattern, retos posteriores) |
| D7 | Validación del empleado en vacaciones | **(b) Réplica local por eventos** (`empleado.creado`, `empleado.actualizado`, `empleado.retirado`) | Autonomía (disponibilidad): vacaciones funciona aunque empleados esté caído. Además `vacaciones.programadas` exige el **email**, que la réplica ya tiene. Se consume `empleado.actualizado` para mantener el email al día |
| D8 | Bienvenida / despedida | Según Reto 4: `empleado.creado` → BIENVENIDA, `empleado.retirado` → DESVINCULACION | El catálogo y el Reto 5 las mueven a `usuario.creado` / `cuenta.desactivada`; se implementa como mapa evento → notificación para cambiarlo sin reescribir |
| D9 | `DELETE /empleados/{id}` | Baja lógica: `estado = RETIRADO`, `fechaRetiro = ahora`; cuerpo opcional `{"motivo"}` con lista cerrada, por defecto `RENUNCIA` | El catálogo exige `motivo` en `empleado.retirado` y el reto no define cómo llega |
| D10 | Fechas de vacaciones | "Pasado" = anterior a **hoy** en `America/Bogota` (configurable); hoy es válido. `diasHabiles` = lunes a viernes, inclusive | El Reto 5 prueba con `fechaInicio = hoy`. El ejemplo del catálogo (12) no cuadra con ninguna regla, se documenta la elegida |
| D11 | Puertos | Servicios nuevos con `expose:` (8083 perfiles, 8084 notificaciones, 8085 vacaciones). RabbitMQ: **solo** la UI `15672` publicada; AMQP `5672` interno | Regla del Reto 3 (solo el Gateway) + excepción mínima justificada: la UI es herramienta de administración exigida por el reto |
| D12 | Swagger | Cada servicio sirve su doc dentro de su prefijo (`/perfiles/docs`, …) | La rúbrica evalúa Swagger UI; así es alcanzable por el Gateway sin rutas nuevas |
| D13 | BD de los servicios nuevos | PostgreSQL, **una instancia por servicio** | Base de datos por servicio (requisito). Postgres ya es conocido por el equipo |

## Fases

| Fase | Contenido | Verificación | Estado |
|---|---|---|---|
| **F1** Broker | RabbitMQ en compose, exchange y colas declarados, healthcheck, UI | UI accesible; exchange y colas visibles | ✅ |
| **F2** Empleados | `PUT`, `DELETE` (baja lógica + `fechaRetiro` + `motivo`), `GET ?estado&desde&hasta`, publicador AMQP con envelope | Pruebas unitarias; los 3 eventos llegan a las colas con el formato exacto | ✅ |
| **F3** Notificaciones (Go) | Consumidor (3 eventos), log `[NOTIFICACIÓN]`, historial en BD, deduplicación, `GET /notificaciones[/{empleadoId}]`, OpenAPI, Dockerfile | Pruebas; evento duplicado → 1 notificación | ⏳ |
| **F4** Perfiles (Python) | Consumidor (3 eventos): crear, sincronizar, archivar; REST `GET`/`PUT`; deduplicación; OpenAPI; Dockerfile | Pruebas; perfil creado por evento, editado y archivado | ⏳ |
| **F5** Vacaciones (Java) | Réplica de empleados, CRUD, 4 validaciones, publica `vacaciones.programadas`, deduplicación, OpenAPI, Dockerfile | Pruebas; las 4 validaciones → 400; evento publicado | ⏳ |
| **F6** Gateway | Rutas `/perfiles`, `/notificaciones`, `/vacaciones`; `/health` con los servicios nuevos y el broker | Todo alcanzable solo por `:8080` | ⏳ |
| **F7** Pruebas E2E | Flujo completo de la sección 6 del PDF, deduplicación desde la UI, persistencia tras reinicio, colección Bruno | Desde cero, todos los pasos del PDF | ⏳ |
| **F8** Documentación | README (broker, lenguajes, despliegue, eventos, D7, evidencia de deduplicación, pruebas), `docs/eventos.md`, manual `docs/reto-04/` | Cada entregable del PDF presente | ⏳ |

## Aprendizajes por fase

- **F1**: si RabbitMQ carga `definitions.json` al arrancar (`load_definitions`), **no crea el usuario por defecto** (log: *"Will not seed default virtual host and user: have definitions to load"*). Se descartó versionar el usuario en el JSON (sería un secreto en el repo). Solución: contenedor `broker-init` de un solo uso que importa la topología por la API de administración cuando el broker ya está sano; los servicios dependerán de él con `condition: service_completed_successfully`. Verificado: login, exchange, 9 bindings, enrutamiento por tipo (un tipo desconocido no se enruta), persistencia tras reinicio e idempotencia del import.

- **F2**: `POST /empleados` aceptaba `estado: RETIRADO`, que habría violado la restricción de coherencia del retiro (500); ahora es 400. `npm audit` detectó `qs` vulnerable (parsea los query params que usa la auditoría): corregido. El publicador usa *publisher confirms*, timeout de 3 s y reconexión perezosa; con el broker caído el registro responde 201 y el evento se pierde (limitación aceptada por el reto → Outbox en retos posteriores). Verificado en Docker: los 3 eventos llegan solo a sus colas, con envelope y `data` idénticos al catálogo, persistentes y con `message_id` = `id`.

## Trampas del PDF ya identificadas

- El paso 7 del PDF usa `2026-06-15`, que ya pasó → `400` por fecha pasada. Usar fechas futuras en las pruebas.
- El ejemplo de compose del PDF publica `5672` y `15672`; aquí solo `15672` (D11).
- `PUT` y `DELETE /empleados` no existen todavía (F2).
- Espacio en disco del equipo de desarrollo: ~6 GB libres; vigilar el tamaño de las imágenes.
