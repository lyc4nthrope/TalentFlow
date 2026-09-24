> **Borrador** — actualizado para reflejar el estado real del código en el Reto 3. Pendiente de confirmación por el resto del equipo antes de darlo por definitivo.

# Microservicio de Gestión de Empleados

Servicio web para la gestión de empleados. Evolucionó del Reto 1 (almacenamiento en memoria) al Reto 2 (persistencia en PostgreSQL y validación cruzada del departamento) y al Reto 3 (Circuit Breaker en esa comunicación, detrás de un API Gateway).

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
   - Si **no se pudo verificar** (Circuit Breaker abierto o dependencia caída), el registro **no se rechaza**: se acepta con `estado: "PENDIENTE_VALIDACION"` (ver justificación abajo).
4. Si el departamento existe (o no se pudo verificar): `201 Created` con el empleado registrado y header `Location: /empleados/{id}`. El `estado` es `"ACTIVO"` por defecto, salvo que quede pendiente de validación (punto anterior).

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

### Rutas no soportadas

Cualquier otra ruta o método responde **404** con `Recurso no encontrado`.

## Comunicación con departamentos-service (Reto 3: Circuit Breaker)

`empleados-service` valida `departamentoId` llamando a `GET {DEPARTAMENTOS_SERVICE_URL}/departamentos/{id}`, envuelto en un Circuit Breaker (`opossum`) para no degradar el servicio cuando la dependencia está caída:

- **Timeout** de 5 segundos por intento (`DEPARTAMENTOS_TIMEOUT_MS`), con **hasta 3 reintentos** (`DEPARTAMENTOS_MAX_REINTENTOS`) antes de que el circuito cuente el fallo.
- **Umbral de apertura**: 4 llamadas mínimas en la ventana (`volumeThreshold`) con 50 % de fallos (`errorThresholdPercentage`).
- **Estado `OPEN`**: las llamadas se bloquean de inmediato (sin tocar la red) y se ejecuta el fallback.
- **Estado `HALF_OPEN`**: tras 30 segundos (`resetTimeout`), la siguiente petición real se deja pasar como prueba.

**Decisión de negocio del fallback** (disponibilidad sobre consistencia inmediata): cuando no se puede verificar el departamento, el empleado **se registra igual**, con `estado: "PENDIENTE_VALIDACION"` — RRHH sigue pudiendo trabajar mientras `departamentos-service` está caído, en vez de bloquear todos los registros con un `503`. El estado que el cliente pidió originalmente (`ACTIVO`, `EN_VACACIONES`, etc.) se conserva internamente como `estadoDeseado`.

**Reconciliación automática**: cuando el circuito pasa a `CLOSED` (la dependencia volvió a responder), se dispara automáticamente una revalidación de todos los empleados en `PENDIENTE_VALIDACION`: si el departamento ya existe, el empleado pasa a su `estadoDeseado` original; si sigue sin poder verificarse, queda pendiente para el próximo cierre del circuito. No requiere reiniciar ningún contenedor ni intervención manual — ver el listener `breaker.on("close", ...)` en `src/server.js`.

## Estructura del código

```
src/
├── clients/departamentos.client.js          # Cliente HTTP + Circuit Breaker (opossum) hacia departamentos-service
├── db/pool.js                               # Pool de conexión a PostgreSQL
├── dominio/empleado.js                      # Modelo canónico + validaciones + estado PENDIENTE_VALIDACION (propio de este servicio)
├── repository/
│   ├── empleados.repository.postgres.js     # Persistencia real (usada en Docker/producción)
│   └── empleados.repository.memoria.js      # Implementación en memoria (usada en tests unitarios)
├── routes/empleados.routes.js               # Definición de rutas Express
├── services/empleados.service.js            # Lógica de negocio, validaciones y reconciliarPendientes()
├── openapi.js                               # Especificación OpenAPI + Swagger UI (/docs)
├── errores.js                               # Clase AppError
├── app.js                                   # Capa HTTP (Express) + manejo de errores
└── server.js                                # Punto de entrada; conecta el evento "close" del circuito con la reconciliación
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

PostgreSQL. El esquema se crea automáticamente desde `init.sql` (montado en `/docker-entrypoint-initdb.d/`) la primera vez que el volumen `vol-empleados` está vacío. Desde el Reto 3, la tabla `empleados` incluye `PENDIENTE_VALIDACION` en el `CHECK` de `estado`, más la columna `estado_deseado` para la reconciliación.

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

Los tests unitarios usan el repositorio en memoria (`empleados.repository.memoria.js`), no PostgreSQL. Incluyen casos para el registro con `PENDIENTE_VALIDACION` y para `reconciliarPendientes()`.

## Documentación OpenAPI

Desde el Reto 3 este servicio ya no publica puerto al host (`expose`, no `ports` — ver README raíz), así que su Swagger UI (`/docs`, `/openapi.json`) no es alcanzable desde fuera de la red de Docker. Para verla: `docker compose exec empleados-service` no sirve para HTTP; hay que exponer el puerto temporalmente o usar `docker compose port empleados-service 8081` + un túnel puntual. El Gateway (Reto 3) solo enruta `/empleados/*`, no `/docs`, a propósito: la tabla de rutas del reto exige exactamente esas dos rutas y nada más.

## Evidencia

Los resultados de las pruebas del Reto 1 se documentan en `docs/reto-01/pruebas.md`.