> **Borrador** — README nuevo, escrito a partir del código existente. Pendiente de confirmación por quien implementó este servicio antes de darlo por definitivo.

# Microservicio de Gestión de Departamentos

Segundo microservicio de negocio del sistema, introducido en el Reto 2 para cumplir el requisito de diversidad tecnológica del proyecto final: implementado en **PHP** (servidor nativo, sin framework ni Composer) con **MySQL** como base de datos, independiente de `empleados-service`.

## Endpoints

### Registrar un departamento

```
POST /departamentos
```

Cuerpo:

```json
{
  "id": "IT",
  "nombre": "Tecnología",
  "descripcion": "Departamento de TI"
}
```

- **201 Created**: departamento registrado, con header `Location: /departamentos/{id}`.
- **400 Bad Request**: faltan campos obligatorios (`id`, `nombre`), cuerpo JSON inválido, o el `id` ya está registrado.

Las respuestas de error siguen el formato `{ status, error, message, timestamp, path, errors? }` (alineado con la clase de API RESTful).

### Listar departamentos

```
GET /departamentos
```

- **200 OK**: arreglo con todos los departamentos registrados.

### Consultar un departamento por id

```
GET /departamentos/{id}
```

- **200 OK**: información del departamento.
- **404 Not Found**: `El departamento con id {id} no existe`.

### Rutas no soportadas

Cualquier otra ruta o método responde **404** con `Recurso no encontrado`.

## Estructura del código

```
public/index.php                    # Punto de entrada HTTP: enrutamiento y manejo de errores
src/
├── Database.php                    # Conexión PDO a MySQL (lee configuración de variables de entorno)
├── DepartamentoRepository.php      # Acceso a datos (buscarPorId, guardar, listar)
└── OpenApi.php                     # Especificación OpenAPI + HTML de Swagger UI (/docs)
```

Sin Composer ni autoload: el servicio no tiene dependencias externas, así que se mantuvo deliberadamente simple.

## Variables de entorno

| Variable | Descripción |
|---|---|
| `PORT` | Puerto donde escucha el servicio (8081 en Docker) |
| `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASS` | Conexión a MySQL |

## Base de datos

MySQL. El esquema se crea automáticamente desde `init.sql` (montado en `/docker-entrypoint-initdb.d/`) la primera vez que el volumen `vol-departamentos` está vacío. La unicidad del `id` se garantiza con `PRIMARY KEY` en el esquema, además de una consulta previa en el código para devolver un mensaje de error descriptivo.

## Construcción y ejecución (Docker)

Desde la raíz del proyecto, junto con el resto del sistema:

```bash
docker compose up --build
```

## Documentación OpenAPI

Swagger UI disponible en `http://localhost:8081/docs` (especificación en `/openapi.json`).

## Pendientes conocidos

- No tiene tests automatizados todavía (a diferencia de `gestion-empleados`).
- No tiene ejecución local documentada sin Docker (requiere PHP 8.2+ y una instancia de MySQL accesible).
