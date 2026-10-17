> **Borrador** — actualizado para reflejar el estado real del código en el Reto 4. Pendiente de confirmación por el resto del equipo antes de darlo por definitivo.

# Microservicio de Gestión de Empleados

Servicio web para la gestión de empleados. Evolucionó del Reto 1 (almacenamiento en memoria) al Reto 2 (persistencia en PostgreSQL y validación cruzada del departamento contra `departamentos-service`) al Reto 3 (Circuit Breaker en esa comunicación, detrás de un API Gateway) y al Reto 4 (actualización, baja lógica con auditoría y **publicación de eventos** en RabbitMQ).

## Endpoints

### Registrar un empleado

```
POST /empleados
```

Cuerpo (modelo canónico, sin recortes respecto al Reto 1):

```json
{
  "id": "E001",
  "nombre": "Juan",
  "apellido": "Pérez",
  "email": "juan.perez@empresa.com",
  "numeroEmpleado": "EMP-2026-001",
  "cargo": "Desarrollador Senior",
  "area": "Tecnología",
  "departamentoId": "IT",
  "fechaIngreso": "2026-02-10"
}
```

Validaciones, en este orden:

1. Unicidad de `email` — si ya existe, `400 Bad Request`.
2. Unicidad de `numeroEmpleado` — si ya existe, `400 Bad Request`.
3. Existencia del `departamentoId` — se consulta a `departamentos-service` por HTTP, a través de un Circuit Breaker (ver sección siguiente):
   - Si el departamento **no existe** (404 real de `departamentos-service`), `400 Bad Request`.
   - Si **no se pudo verificar** (Circuit Breaker abierto o reintentos agotados), el registro **no se rechaza**: se acepta con `validacionDepartamento: "PENDIENTE"`.
4. Si todo pasa: `201 Created` con el empleado registrado (`estado: "ACTIVO"` por defecto, `validacionDepartamento: "ACEPTADO"` o `"PENDIENTE"`) y header `Location: /empleados/{id}`.

Las respuestas de error siguen el formato `{ status, error, message, timestamp, path, errors? }` (alineado con la clase de API RESTful).

### Listar empleados (y auditoría de retiros)

```
GET /empleados
GET /empleados?estado=RETIRADO
GET /empleados?estado=RETIRADO&desde=2026-01-01&hasta=2026-06-30
```

- **200 OK**: arreglo de empleados; cada uno incluye `fechaRetiro` y `motivoRetiro` (`null` si no está retirado).
- `desde`/`hasta` (`YYYY-MM-DD`, inclusivos) filtran por el **día** del retiro en `America/Bogota` y exigen `estado=RETIRADO`.
- **400**: estado desconocido, fecha mal formada o inexistente, `desde` > `hasta`, o `desde`/`hasta` sin `estado=RETIRADO`.

### Consultar un empleado por id

```
GET /empleados/{id}
```

- **200 OK**: información del empleado.
- **404 Not Found**: `El empleado con id {id} no existe`.

### Actualizar un empleado (Reto 4)

```
PUT /empleados/{id}
```

Cuerpo: exactamente los campos que replica el evento `empleado.actualizado` — `nombre`, `apellido`, `email`, `cargo`, `area`, `departamentoId` (los 6 obligatorios).

- **200 OK**: empleado actualizado; publica `empleado.actualizado`.
- **400**: falta un campo, un campo es desconocido o se intenta cambiar uno de solo lectura (`id`, `numeroEmpleado`, `fechaIngreso`, `estado`…; se aceptan si no cambian, para poder hacer GET → editar → PUT), email inválido o usado por otro empleado, departamento inexistente (solo se valida si cambió).
- **404** si no existe · **409** si está `RETIRADO`.

### Retirar un empleado — baja lógica (Reto 4)

```
DELETE /empleados/{id}
```

Cuerpo opcional: `{"motivo": "RENUNCIA" | "DESPIDO" | "JUBILACION" | "FIN_CONTRATO" | "OTRO"}` (por defecto `RENUNCIA`).

- **200 OK**: el empleado **no se borra**: queda `estado: "RETIRADO"` con `fechaRetiro` (instante UTC) y `motivoRetiro`; publica `empleado.retirado`.
- **400** motivo no válido · **404** si no existe · **409** si ya estaba retirado (atómico: con dos `DELETE` simultáneos, uno gana y se publica un solo evento).

### Estado del Circuit Breaker (diagnóstico)

```
GET /empleados/circuito-departamentos
```

- **200 OK**: `{ "dependencia": "departamentos-service", "estado": "CLOSED" | "OPEN" | "HALF_OPEN" }`.

### Health check (interno)

```
GET /health
```

- **200 OK**: `{ "status": "UP", "service": "empleados-service", "components": { "app": "UP", "db": "UP", "circuitoDepartamentos": "CLOSED" } }`.
- **503 Service Unavailable**: Postgres no responde en 2 s (`"db": "DOWN"`).

Un circuito `OPEN` no lo marca `DOWN` (el servicio sigue atendiendo con el fallback). Lo usan el `healthcheck` de `docker-compose.yml` y el `GET /health` agregado del Gateway; no es alcanzable desde fuera (el Gateway solo enruta `/empleados/*`).

### Rutas no soportadas

Cualquier otra ruta o método responde **404** con `Recurso no encontrado`.

## Comunicación con departamentos-service (Reto 3: Circuit Breaker)

`empleados-service` valida `departamentoId` llamando a `GET {DEPARTAMENTOS_SERVICE_URL}/departamentos/{id}`, envuelto en un Circuit Breaker (`opossum`):

- **Timeout** de 5 segundos por intento, con **hasta 3 reintentos** (espera fija de 200ms) antes de que el circuito cuente el fallo.
- **Apertura**: 50 % de fallos con un mínimo de 4 llamadas (`volumeThreshold`) dentro de una ventana de 60s.
- **`OPEN`**: las llamadas no tocan la red; se ejecuta el fallback de inmediato.
- **`HALF_OPEN`**: tras 30s (`resetTimeout`), la siguiente petición real se deja pasar como prueba.

**Fallback**: el empleado se registra con `validacionDepartamento: "PENDIENTE"` (campo independiente del `estado` laboral). Cuando el circuito vuelve a `CLOSED`, `reconciliarPendientes()` revisa automáticamente cada pendiente: `ACEPTADO` si el departamento existe, `RECHAZADO` si no. Justificación completa y prueba reproducible en el [README raíz](../../README.md).

## Eventos que publica (Reto 4)

| Evento (Catálogo) | Cuándo | `data` |
|---|---|---|
| `empleado.creado` (3.1) | `POST /empleados` exitoso (también con validación `PENDIENTE`: el empleado sí quedó registrado) | `empleadoId, nombre, apellido, email, numeroEmpleado, cargo, area, departamentoId, fechaIngreso, estado` |
| `empleado.actualizado` (3.2) | `PUT /empleados/{id}` exitoso | `empleadoId, nombre, apellido, email, cargo, area, departamentoId` |
| `empleado.retirado` (3.3) | `DELETE /empleados/{id}` exitoso | `empleadoId, email, fechaRetiro, motivo` |

Envelope del catálogo (`id` UUID del mensaje, `type`, `version: 1`, `occurredAt` UTC, `producer: "empleados-service"`, `data`) al exchange `talentflow.eventos` con routing key = `type`, mensaje persistente. Se publica **después** de guardar en la BD, con confirmación del broker y tiempo máximo de 3 s; si falla, se registra el error y la operación **no** se revierte. La conexión se reintenta en segundo plano; `/health` reporta `broker: UP/DOWN`.

## Estructura del código

```
src/
├── clients/departamentos.client.js          # Cliente HTTP + Circuit Breaker (opossum) hacia departamentos-service
├── db/pool.js                               # Pool de conexión a PostgreSQL
├── dominio/empleado.js                      # Modelo canónico + validaciones, actualización, retiro y filtros
├── eventos/
│   ├── envelope.js                          # Envelope del Catálogo de Eventos
│   ├── eventos-empleado.js                  # Cargas útiles 3.1-3.3, campo por campo
│   └── publicador.js                        # Publicador AMQP (confirms, timeout, reconexión)
├── repository/
│   ├── empleados.repository.postgres.js     # Persistencia real (usada en Docker/producción)
│   └── empleados.repository.memoria.js      # Implementación en memoria (usada en tests unitarios)
├── routes/empleados.routes.js               # Definición de rutas Express
├── services/empleados.service.js            # Lógica de negocio, validaciones y reconciliarPendientes()
├── openapi.js                               # Especificación OpenAPI + Swagger UI (/empleados/docs)
├── errores.js                               # Clase AppError
├── app.js                                   # Capa HTTP (Express) + manejo de errores
└── server.js                                # Punto de entrada; conecta el cierre del circuito con la reconciliación
```

Todo el código de este servicio vive dentro de esta carpeta: no depende de ningún paquete compartido con otros microservicios (antes existía `@talentflow/shared`; se eliminó en el Reto 2 porque ese patrón es de monolito, no de microservicios — ver nota en el README raíz).

## Variables de entorno

| Variable | Descripción |
|---|---|
| `PORT` | Puerto donde escucha el servicio (8081 en Docker, no publicado al host desde el Reto 3 — ver README raíz) |
| `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASS` | Conexión a PostgreSQL |
| `DEPARTAMENTOS_SERVICE_URL` | URL base de `departamentos-service` (dentro de la red de Docker: `http://departamentos-service:8082`, nunca `localhost`) |
| `DEPARTAMENTOS_TIMEOUT_MS` | Timeout por intento hacia `departamentos-service` (por defecto 5000ms desde el Reto 3) |
| `DEPARTAMENTOS_MAX_REINTENTOS` | Reintentos antes de que el Circuit Breaker cuente el fallo (por defecto 3) |
| `RABBITMQ_HOST`, `RABBITMQ_PORT`, `RABBITMQ_USER`, `RABBITMQ_PASS` | Broker para publicar eventos (Reto 4) |
| `RABBITMQ_EXCHANGE` | Exchange de eventos (`talentflow.eventos`) |
| `ZONA_HORARIA` | Zona para la auditoría por fechas (`America/Bogota`) |

## Base de datos

PostgreSQL. El esquema se crea automáticamente desde `init.sql` (montado en `/docker-entrypoint-initdb.d/`) la primera vez que el volumen `vol-empleados` está vacío. Desde el Reto 3 la tabla incluye la columna `validacion_departamento` (`PENDIENTE` / `ACEPTADO` / `RECHAZADO`); si tu volumen es anterior, hay que recrearlo con `docker compose down -v`. Desde el Reto 4 incluye `fecha_retiro` y `motivo_retiro`, con una restricción que garantiza que `RETIRADO` ⇔ tiene fecha y motivo.

## Construcción y ejecución (Docker)

Desde la raíz del proyecto, junto con el resto del sistema:

```bash
docker compose up --build
```

## Ejecución local (sin Docker)

Desde la raíz del proyecto:

```bash
npm install
npm run dev:empleados
```

Requiere una instancia de PostgreSQL accesible con las variables de entorno de arriba.

## Pruebas

```bash
npm test
```

80 pruebas. Usan el repositorio en memoria (`empleados.repository.memoria.js`) y un broker simulado: no requieren PostgreSQL ni RabbitMQ. Incluyen el contrato de los eventos campo por campo contra el catálogo.

## Documentación OpenAPI

Swagger UI en `http://localhost:8080/empleados/docs` y la especificación en `/empleados/openapi.json`, a través del Gateway. Viven bajo el prefijo `/empleados` porque el servicio no publica puerto al host y el Gateway solo enruta ese prefijo (hasta el Reto 3 estaban en `/docs` y no eran alcanzables).

## Evidencia

Los resultados de las pruebas del Reto 1 se documentan en `docs/reto-01/pruebas.md`.
