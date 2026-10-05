# Servicio de Notificaciones (Go)

Microservicio **puramente reactivo** (Reto 4): ningún servicio lo invoca por REST. Consume eventos de RabbitMQ, **simula el envío** de la notificación con un log estructurado, opcionalmente la **envía como correo real por SMTP** (bonus) y guarda el historial en su propia base de datos (PostgreSQL). Sus endpoints REST solo consultan ese historial.

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

El envío es un puerto (`Canal`) con dos implementaciones; el caso de uso no distingue entre ellas:

- `CanalConsola` (siempre activa): la línea de log anterior, que es lo que exige el reto.
- `smtp.Canal` (bonus, solo si se define `SMTP_HOST`): correo real de texto plano en UTF-8, con solo la biblioteca estándar de Go (`net/smtp`), sin autenticación ni TLS (lo que ofrece Mailhog). Asunto según el tipo (`Bienvenida a la empresa`, `Vacaciones programadas`, `Desvinculación de la empresa`), cuerpo = `mensaje`, destinatario = `destinatario`.

Cuando hay SMTP, `CanalMultiple` envía por ambas. En Docker Compose el servidor es **Mailhog** (`http://localhost:8025`); ver el [manual del Reto 4](../../docs/reto-04/README.md#6-bonus-correo-real-por-smtp-mailhog).

## Deduplicación y manejo de fallos

- `eventos_procesados(id, procesado_en)`: el `id` del mensaje y la notificación se guardan en **una sola transacción** (`INSERT … ON CONFLICT DO NOTHING`); si el `id` ya existía, es un duplicado y no se repite el efecto.
- `ack` **solo después del commit**; la notificación se "envía" después de registrarla (así un reintento nunca la envía dos veces).
- Mensaje corrupto, tipo no soportado o datos que la BD rechaza (SQLSTATE 22/23) → se descarta con log. Fallo transitorio (BD caída) → `nack` con reencolado tras 5 s.
- Reconexión automática al broker con espera exponencial (1 s → 30 s).
- Fallo del correo SMTP (servidor caído o lento): se registra el error y se sigue; **no** se reintenta ni se reencola (la notificación ya está guardada y reencolar podría duplicar correos). Cada envío tiene un plazo total de 5 s para no detener al consumidor. Un evento duplicado no envía un segundo correo: solo se envía al registrar la notificación.

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
internal/aplicacion/          # caso de uso: deduplicar, registrar, enviar (puertos Repositorio y Canal; canales consola y múltiple)
internal/postgres/            # repositorio (transacción de deduplicación)
internal/amqp/                # consumidor RabbitMQ (ack/nack, reconexión)
internal/smtp/                # canal de correo real por SMTP (bonus)
internal/api/                 # REST + OpenAPI embebido
```

## Variables de entorno

| Variable | Descripción |
|---|---|
| `PORT` | Puerto HTTP (8084) |
| `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASS` | PostgreSQL propio |
| `RABBITMQ_HOST`, `RABBITMQ_PORT`, `RABBITMQ_USER`, `RABBITMQ_PASS` | Broker |
| `RABBITMQ_COLA` | Cola a consumir (`notificaciones.eventos`) |
| `SMTP_HOST` | Servidor SMTP. Vacío o sin definir: no se envía correo, solo el log (en Compose: `mailhog`) |
| `SMTP_PORT` | Puerto SMTP (`1025`) |
| `SMTP_FROM` | Remitente (`notificaciones@talentflow.local`) |

## Pruebas y ejecución

```bash
go test ./...                  # 22 pruebas (dominio, envelope, caso de uso con deduplicación, canales, SMTP contra un servidor de prueba, API)
docker compose up --build      # desde la raíz del repo, con todo el sistema
```

Imagen: compilación multi-stage y `distroless/static:nonroot` (~14 MB, sin shell, usuario sin privilegios); el healthcheck lo hace el propio binario (`/notificaciones -healthcheck`).
