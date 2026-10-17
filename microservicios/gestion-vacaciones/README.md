# Servicio de Gestión de Vacaciones (Java + Spring Boot)

Microservicio del Reto 4 que **produce eventos propios**: expone REST a RR. HH. para programar períodos de vacaciones y publica `vacaciones.programadas`, que otros dominios consumen sin que este servicio sepa quién reacciona. Desde el Reto 5, un **scheduler** publica `vacaciones.iniciadas` y `vacaciones.finalizadas` al llegar cada fecha. Base de datos propia (PostgreSQL).

## Endpoints

| Método | Ruta | Descripción |
|---|---|---|
| POST | `/vacaciones` | Programa un período (`201` + `Location`) y publica `vacaciones.programadas` |
| GET | `/vacaciones` | Todos los períodos |
| GET | `/vacaciones?empleadoId={id}` | Los de un empleado |
| GET | `/vacaciones/{id}` | Un período (`404` si no existe) |
| DELETE | `/vacaciones/{id}` | Cancela un período que aún no ha iniciado: queda `CANCELADA` (no se borra); si ya inició o no está programado → `409` |
| GET | `/vacaciones/docs` · `/vacaciones/openapi.json` | Swagger UI / especificación |
| GET | `/health` | Interno: `status`, `db`, `broker` |

Período: `{ "id": "V-2026-0042", "empleadoId", "fechaInicio", "fechaFin", "estado", "fechaCreacion" }`. Estados: `PROGRAMADA → EN_CURSO → FINALIZADA` (por el scheduler), o `CANCELADA` (por RR. HH.).

### Fechas con hora (Reto 5)

`fechaInicio` y `fechaFin` son **fecha y hora locales** en `ZONA_HORARIA` (`America/Bogota`):

| Entrada | Se interpreta como |
|---|---|
| `"2026-10-20T08:00"` o `"2026-10-20T08:00:00"` | Esa fecha y hora (sin fracciones de segundo) |
| `"2026-10-20"` (compatibilidad con el Reto 4) | Inicio: `2026-10-20T00:00:00`; fin: `2026-10-20T23:59:59` |
| `"2026-10-20"` como inicio, **si es hoy** | Empieza **ahora** (como en el Reto 4, "hoy es válido"); un día anterior sigue siendo `400` |

**Cambio deliberado frente al Reto 4:** el mismo día con la fecha sola (`"fechaInicio": "2026-12-14", "fechaFin": "2026-12-14"`) ahora es **válido**: un período de un día completo (00:00:00 a 23:59:59), que usa el paso 12 del Reto 5. En el Reto 4 era `400` porque `fechaFin` debía ser un día posterior.

Las respuestas devuelven `"2026-10-20T08:00:00"` (fecha y hora local). Con hora se puede demostrar el ciclo completo en minutos: programar inicio = ahora + 2 min y fin = ahora + 5 min.

## Validaciones (todas `400` con mensaje descriptivo)

1. **Fechas incoherentes**: `fechaFin` debe ser posterior a `fechaInicio`.
2. **Fechas en el pasado**: `fechaInicio` anterior a **ahora** en `America/Bogota` (una hora antes, hoy mismo, ya es pasado).
3. **Solapamiento** con un período `PROGRAMADA` o `EN_CURSO` del empleado: incluye `periodoEnConflicto`. Rangos semiabiertos `[inicio, fin)`: un período puede empezar justo cuando termina otro. Garantizado ante concurrencia con `SELECT … FOR UPDATE` sobre el empleado y una restricción `EXCLUDE USING gist` (`tsrange(..., '[)')`) en la BD.
4. **Empleado inexistente o retirado**: se valida contra la **réplica local** (ver abajo).

Campos desconocidos en el cuerpo → `400` (Jackson `fail-on-unknown-properties`).

## Réplica de empleados por eventos (cola `vacaciones.eventos`)

Decisión (b) del reto — **disponibilidad sobre consistencia inmediata**: en vez de consultar a empleados-service por REST en cada solicitud, este servicio consume `empleado.creado`, `empleado.actualizado` y `empleado.retirado` y mantiene su tabla `empleados_replica(empleado_id, email, retirado)`. Así funciona aunque empleados-service esté caído, y tiene el `email` que exige `vacaciones.programadas`. Deduplicación por `id` de mensaje en la misma transacción que el efecto; `ack` manual (el reencolado automático de Spring AMQP ante cualquier excepción sería un bucle infinito).

## Eventos que publica

| Evento | Catálogo | Cuándo | `data` |
|---|---|---|---|
| `vacaciones.programadas` | 3.8 | `POST /vacaciones` | `vacacionesId, empleadoId, email, fechaInicio, fechaFin, diasHabiles` |
| `vacaciones.iniciadas` | 3.9 | El scheduler ve `fechaInicio <= ahora` (pasa a `EN_CURSO`) | `vacacionesId, empleadoId, email, fechaInicio, fechaFin` |
| `vacaciones.finalizadas` | 3.10 | El scheduler ve `fechaFin <= ahora` (pasa a `FINALIZADA`) | `vacacionesId, empleadoId, email, fechaFin` |

Todos con el envelope del catálogo, **después del commit** y con confirmación del broker; si la publicación falla, el cambio queda registrado y el error va al log. Las fechas viajan en ISO-8601 **con desfase** (`"2026-10-20T08:00:00-05:00"`): desviación documentada del ejemplo del catálogo, que muestra solo la fecha. `diasHabiles` = lunes a viernes entre la fecha de inicio y la de fin, ambas incluidas, sin festivos. Cancelar un período no publica evento (el catálogo no define uno).

## Scheduler (Reto 5)

`@Scheduled(cron = "${VACACIONES_CRON}")`, por defecto **cada minuto** (`0 * * * * *`; `-` lo desactiva). Cada pasada:

1. `PROGRAMADA` con `fechaInicio <= ahora` → `EN_CURSO` y publica `vacaciones.iniciadas`.
2. `EN_CURSO` con `fechaFin <= ahora` → `FINALIZADA` y publica `vacaciones.finalizadas`.

Un período cuyas dos fechas ya pasaron recorre ambas transiciones en la misma pasada, en orden. Cada período va en su propia transacción y el cambio de estado es condicional (`WHERE estado = <esperado>`): si RR. HH. lo canceló justo antes, o la pasada se repite, no se publica nada dos veces. Un período que falla no detiene a los demás y se reintenta en la siguiente pasada.

**Limitación conocida (escalado):** `@Scheduled` corre en cada instancia. Con N instancias, todas intentan la misma transición; la actualización condicional deja pasar solo a una, pero el trabajo se repite. La coordinación entre instancias (un único ejecutor) es el Reto 31 con ShedLock.

## Estructura del código (hexagonal)

```
com.talentflow.vacaciones
├── dominio/                    # Vacaciones, estados, validaciones, diasHabiles (Java puro)
├── aplicacion/                 # ServicioVacaciones, ProcesadorCicloVacaciones, ProcesadorEventosEmpleado y puertos
└── infraestructura/
    ├── persistencia/           # JDBC (JdbcClient): SQL explícito, deduplicación, transacciones
    ├── mensajeria/             # RabbitMQ: publicador (confirms) y consumidor (ack manual)
    ├── scheduler/              # dispara el ciclo de vida por fecha (VACACIONES_CRON)
    └── web/                    # controlador REST, errores, /health, Swagger
```

## Variables de entorno

| Variable | Descripción |
|---|---|
| `PORT` | Puerto HTTP (8085) |
| `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASS` | PostgreSQL propio |
| `RABBITMQ_HOST`, `RABBITMQ_PORT`, `RABBITMQ_USER`, `RABBITMQ_PASS` | Broker |
| `RABBITMQ_COLA` | Cola a consumir (`vacaciones.eventos`) |
| `ZONA_HORARIA` | Zona de las fechas y de "ahora" (`America/Bogota`) |
| `VACACIONES_CRON` | Frecuencia del scheduler, cron de Spring (`0 * * * * *` = cada minuto) |

## Pruebas y ejecución

```bash
mvn test                       # 61 pruebas (dominio, casos de uso y scheduler con fakes, eventos publicados, controlador, arranque del contexto)
docker compose up --build      # desde la raíz del repo
```

Imagen: compilación con Maven y ejecución sobre `eclipse-temurin:17-jre-alpine`, usuario sin privilegios, heap al 75 % de la memoria del contenedor.

> **Base de datos existente:** el Reto 5 cambió `fecha_inicio`/`fecha_fin` de `DATE` a `TIMESTAMP`. `init.sql` solo corre con un volumen nuevo: para aplicar el esquema, recrear el volumen de vacaciones (`docker compose down -v` borra todos los volúmenes del proyecto).
