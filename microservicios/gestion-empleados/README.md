> **Borrador** — actualizado para reflejar el estado real del código en el Reto 3. Pendiente de confirmación por el resto del equipo antes de darlo por definitivo.

# Microservicio de Gestión de Empleados

Servicio web para la gestión de empleados. Evolucionó del Reto 1 (almacenamiento en memoria) al Reto 2 (persistencia en PostgreSQL y validación cruzada del departamento contra `departamentos-service`) y al Reto 3 (Circuit Breaker en esa comunicación, detrás de un API Gateway).

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

### Listar empleados

```
GET /empleados
```

- **200 OK**: arreglo con todos los empleados registrados.

### Consultar un empleado por id

```
GET /empleados/{id}
```

- **200 OK**: información del empleado.
- **404 Not Found**: `El empleado con id {id} no existe`.

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

## Estructura del código

```
src/
├── clients/departamentos.client.js          # Cliente HTTP + Circuit Breaker (opossum) hacia departamentos-service
├── db/pool.js                               # Pool de conexión a PostgreSQL
├── dominio/empleado.js                      # Modelo canónico + validaciones (propio de este servicio)
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

## Base de datos

PostgreSQL. El esquema se crea automáticamente desde `init.sql` (montado en `/docker-entrypoint-initdb.d/`) la primera vez que el volumen `vol-empleados` está vacío. Desde el Reto 3 la tabla incluye la columna `validacion_departamento` (`PENDIENTE` / `ACEPTADO` / `RECHAZADO`); si tu volumen es anterior, hay que recrearlo con `docker compose down -v`.

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

Los tests unitarios usan el repositorio en memoria (`empleados.repository.memoria.js`), no PostgreSQL.

## Documentación OpenAPI

Swagger UI en `http://localhost:8080/empleados/docs` y la especificación en `/empleados/openapi.json`, a través del Gateway. Viven bajo el prefijo `/empleados` porque el servicio no publica puerto al host y el Gateway solo enruta ese prefijo (hasta el Reto 3 estaban en `/docs` y no eran alcanzables).

## Evidencia

Los resultados de las pruebas del Reto 1 se documentan en `docs/reto-01/pruebas.md`.
