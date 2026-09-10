> **Borrador** — actualizado para reflejar el estado real del código en el Reto 2. Pendiente de confirmación por el resto del equipo antes de darlo por definitivo.

# Microservicio de Gestión de Empleados

Servicio web para la gestión de empleados. Evolucionó del Reto 1 (almacenamiento en memoria) al Reto 2: persistencia en PostgreSQL y validación cruzada del departamento contra `departamentos-service`.

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
3. Existencia del `departamentoId` — se consulta a `departamentos-service` por HTTP; si no existe, `400 Bad Request`. Si el servicio de departamentos no responde tras reintentos, `503 Service Unavailable`.
4. Si todo pasa: `201 Created` con el empleado registrado (`estado: "ACTIVO"` por defecto) y header `Location: /empleados/{id}`.

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

## Comunicación con departamentos-service

`empleados-service` valida `departamentoId` llamando a `GET {DEPARTAMENTOS_SERVICE_URL}/departamentos/{id}` con:

- **Timeout** de 2 segundos por intento.
- **Hasta 3 reintentos** con espera creciente: 1s → 2s → 4s.
- Si se agotan los reintentos sin respuesta, el registro se **rechaza** con `503` (no se acepta como "pendiente de validación" — ver el comentario en `src/clients/departamentos.client.js` para la justificación completa).

## Estructura del código

```
src/
├── clients/departamentos.client.js          # Cliente HTTP hacia departamentos-service (timeout + reintentos)
├── db/pool.js                               # Pool de conexión a PostgreSQL
├── dominio/empleado.js                      # Modelo canónico + validaciones (propio de este servicio)
├── repository/
│   ├── empleados.repository.postgres.js     # Persistencia real (usada en Docker/producción)
│   └── empleados.repository.memoria.js      # Implementación en memoria (usada en tests unitarios)
├── routes/empleados.routes.js               # Definición de rutas Express
├── services/empleados.service.js            # Lógica de negocio y validaciones
├── openapi.js                               # Especificación OpenAPI + Swagger UI (/docs)
├── errores.js                               # Clase AppError
├── app.js                                   # Capa HTTP (Express) + manejo de errores
└── server.js                                # Punto de entrada
```

Todo el código de este servicio vive dentro de esta carpeta: no depende de ningún paquete compartido con otros microservicios (antes existía `@talentflow/shared`; se eliminó en el Reto 2 porque ese patrón es de monolito, no de microservicios — ver nota en el README raíz).

## Variables de entorno

| Variable | Descripción |
|---|---|
| `PORT` | Puerto donde escucha el servicio (8080 en Docker) |
| `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASS` | Conexión a PostgreSQL |
| `DEPARTAMENTOS_SERVICE_URL` | URL base de `departamentos-service` (dentro de la red de Docker: `http://departamentos-service:8081`, nunca `localhost`) |

## Base de datos

PostgreSQL. El esquema se crea automáticamente desde `init.sql` (montado en `/docker-entrypoint-initdb.d/`) la primera vez que el volumen `vol-empleados` está vacío.

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

Swagger UI disponible en `http://localhost:8080/docs` (especificación en `/openapi.json`).

## Evidencia

Los resultados de las pruebas del Reto 1 se documentan en `docs/reto-01/pruebas.md`.
