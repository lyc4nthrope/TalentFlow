# Servicio de Gestión de Vacaciones (Java + Spring Boot)

Microservicio del Reto 4 que **produce eventos propios**: expone REST a RR. HH. para programar períodos de vacaciones y publica `vacaciones.programadas`, que otros dominios consumen sin que este servicio sepa quién reacciona. Base de datos propia (PostgreSQL).

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

Período: `{ "id": "V-2026-0042", "empleadoId", "fechaInicio", "fechaFin", "estado", "fechaCreacion" }`. Estados: `PROGRAMADA → EN_CURSO → FINALIZADA`, o `CANCELADA` (en el Reto 4 solo se alcanza `PROGRAMADA`; las transiciones por fecha llegan con el scheduler del Reto 5).

## Validaciones (todas `400` con mensaje descriptivo)

1. **Fechas incoherentes**: `fechaFin` debe ser posterior a `fechaInicio`.
2. **Fechas en el pasado**: `fechaInicio` anterior a hoy en `America/Bogota` (hoy es válido).
3. **Solapamiento** con un período `PROGRAMADA` o `EN_CURSO` del empleado: incluye `periodoEnConflicto`. Garantizado ante concurrencia con `SELECT … FOR UPDATE` sobre el empleado y una restricción `EXCLUDE USING gist` en la BD.
4. **Empleado inexistente o retirado**: se valida contra la **réplica local** (ver abajo).

Campos desconocidos en el cuerpo → `400` (Jackson `fail-on-unknown-properties`).

## Réplica de empleados por eventos (cola `vacaciones.eventos`)

Decisión (b) del reto — **disponibilidad sobre consistencia inmediata**: en vez de consultar a empleados-service por REST en cada solicitud, este servicio consume `empleado.creado`, `empleado.actualizado` y `empleado.retirado` y mantiene su tabla `empleados_replica(empleado_id, email, retirado)`. Así funciona aunque empleados-service esté caído, y tiene el `email` que exige `vacaciones.programadas`. Deduplicación por `id` de mensaje en la misma transacción que el efecto; `ack` manual (el reencolado automático de Spring AMQP ante cualquier excepción sería un bucle infinito).

## Evento que publica

`vacaciones.programadas` (Catálogo 3.8): `{ vacacionesId, empleadoId, email, fechaInicio, fechaFin, diasHabiles }`, con el envelope del catálogo, **después del commit** y con confirmación del broker. Si la publicación falla, el período queda registrado y el error va al log. `diasHabiles` = lunes a viernes, ambos extremos incluidos, sin festivos. Cancelar un período no publica evento (el catálogo no define uno).

## Estructura del código (hexagonal)

```
com.talentflow.vacaciones
├── dominio/                    # Vacaciones, estados, validaciones, diasHabiles (Java puro)
├── aplicacion/                 # ServicioVacaciones, ProcesadorEventosEmpleado y puertos
└── infraestructura/
    ├── persistencia/           # JDBC (JdbcClient): SQL explícito, deduplicación, transacciones
    ├── mensajeria/             # RabbitMQ: publicador (confirms) y consumidor (ack manual)
    └── web/                    # controlador REST, errores, /health, Swagger
```

## Variables de entorno

| Variable | Descripción |
|---|---|
| `PORT` | Puerto HTTP (8085) |
| `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASS` | PostgreSQL propio |
| `RABBITMQ_HOST`, `RABBITMQ_PORT`, `RABBITMQ_USER`, `RABBITMQ_PASS` | Broker |
| `RABBITMQ_COLA` | Cola a consumir (`vacaciones.eventos`) |
| `ZONA_HORARIA` | Zona para "hoy" (`America/Bogota`) |

## Pruebas y ejecución

```bash
mvn test                       # 29 pruebas (dominio, casos de uso con fakes, eventos, controlador)
docker compose up --build      # desde la raíz del repo
```

Imagen: compilación con Maven y ejecución sobre `eclipse-temurin:17-jre-alpine`, usuario sin privilegios, heap al 75 % de la memoria del contenedor.
