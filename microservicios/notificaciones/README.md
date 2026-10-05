# Servicio de Notificaciones (Go)

Microservicio **puramente reactivo** (Reto 4): ningún servicio lo invoca por REST. Consume eventos de RabbitMQ, **simula el envío** de la notificación con un log estructurado y guarda el historial en su propia base de datos (PostgreSQL). Sus endpoints REST solo consultan ese historial.

## Eventos que consume (cola `notificaciones.eventos`)

| Evento (Catálogo) | Notificación | Destinatario |
|---|---|---|
| `empleado.creado` (3.1) | `BIENVENIDA` — "Bienvenido {nombre} {apellido} a la empresa…" | `data.email` |
| `empleado.retirado` (3.3) | `DESVINCULACION` — "Su cuenta ha sido desactivada…" | `data.email` |
| `vacaciones.programadas` (3.8) | `VACACIONES` — "Sus vacaciones del {inicio} al {fin} ({diasHabiles} días hábiles)…" | `data.email` |

Simulación del envío (salida estándar del contenedor):

```
[NOTIFICACIÓN] Tipo: BIENVENIDA | Para: juan.perez@empresa.com | Mensaje: "Bienvenido Juan Pérez a la empresa. Tu registro como empleado quedó completo."
```

El envío es un puerto (`Canal`): hoy lo implementa la consola (`CanalConsola`); un envío real por SMTP (bonus del reto, p. ej. Mailhog) sería otra implementación sin tocar el caso de uso.

## Deduplicación y manejo de fallos

- `eventos_procesados(id, procesado_en)`: el `id` del mensaje y la notificación se guardan en **una sola transacción** (`INSERT … ON CONFLICT DO NOTHING`); si el `id` ya existía, es un duplicado y no se repite el efecto.
- `ack` **solo después del commit**; la notificación se "envía" después de registrarla (así un reintento nunca la envía dos veces).
- Mensaje corrupto, tipo no soportado o datos que la BD rechaza (SQLSTATE 22/23) → se descarta con log. Fallo transitorio (BD caída) → `nack` con reencolado tras 5 s.
- Reconexión automática al broker con espera exponencial (1 s → 30 s).

## Endpoints

| Método | Ruta | Descripción |
|---|---|---|
| GET | `/notificaciones` | Todas las notificaciones, en orden de envío |
| GET | `/notificaciones/{empleadoId}` | Las de un empleado (`[]` si no tiene) |
| GET | `/notificaciones/docs` · `/notificaciones/openapi.json` | Swagger UI / especificación |
| GET | `/health` | Interno: `status`, `db`, `broker` |

Estructura de una notificación: `{ id, tipo, destinatario, mensaje, fechaEnvio, empleadoId }`.

## Estructura del código

```
cmd/notificaciones/main.go    # arranque: configuración, BD, consumidor, HTTP, cierre ordenado
internal/eventos/             # envelope del catálogo: parseo y validación
internal/dominio/             # evento → notificación (reglas puras)
internal/aplicacion/          # caso de uso: deduplicar, registrar, enviar (puertos Repositorio y Canal)
internal/postgres/            # repositorio (transacción de deduplicación)
internal/amqp/                # consumidor RabbitMQ (ack/nack, reconexión)
internal/api/                 # REST + OpenAPI embebido
```

## Variables de entorno

| Variable | Descripción |
|---|---|
| `PORT` | Puerto HTTP (8084) |
| `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASS` | PostgreSQL propio |
| `RABBITMQ_HOST`, `RABBITMQ_PORT`, `RABBITMQ_USER`, `RABBITMQ_PASS` | Broker |
| `RABBITMQ_COLA` | Cola a consumir (`notificaciones.eventos`) |

## Pruebas y ejecución

```bash
go test ./...                  # 17 pruebas (dominio, envelope, caso de uso con deduplicación, API)
docker compose up --build      # desde la raíz del repo, con todo el sistema
```

Imagen: compilación multi-stage y `distroless/static:nonroot` (~14 MB, sin shell, usuario sin privilegios); el healthcheck lo hace el propio binario (`/notificaciones -healthcheck`).
