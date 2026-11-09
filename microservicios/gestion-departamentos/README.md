> **Borrador** — README nuevo, escrito a partir del código existente. Pendiente de confirmación por quien implementó este servicio antes de darlo por definitivo.

# Microservicio de Gestión de Departamentos

Segundo microservicio de negocio del sistema, introducido en el Reto 2 para cumplir el requisito de diversidad tecnológica del proyecto final: implementado en **PHP** (servidor nativo, sin framework; Composer solo para las pruebas) con **MySQL** como base de datos, independiente de `empleados-service`.

## Endpoints

| Método y ruta | Éxito | Errores |
|---|---|---|
| `POST /departamentos` | **201** + `Location: /departamentos/{id}` | **400**: faltan `id`/`nombre`, JSON inválido o `id` ya registrado |
| `GET /departamentos` | **200**: arreglo, en orden de creación | — |
| `GET /departamentos/{id}` | **200** | **404**: `El departamento con id {id} no existe` |
| `PUT /departamentos/{id}` | **200**: el departamento actualizado | **400**: falta `nombre`, JSON inválido o `id` del cuerpo distinto del de la ruta (`No se puede cambiar el id`); **404** si no existe |
| `DELETE /departamentos/{id}` | **204** sin cuerpo | **404** si no existe |

Cuerpo de `POST` (y de `PUT`, sin `id`):

```json
{
  "id": "IT",
  "nombre": "Tecnología",
  "descripcion": "Departamento de TI"
}
```

`PUT` es un reemplazo completo: si no se envía `descripcion`, queda en `null`. El `id` no se puede cambiar; enviarlo igual al de la ruta es válido.

Las respuestas de error siguen el formato `{ status, error, message, timestamp, path, errors? }` (alineado con la clase de API RESTful). Si la base de datos no responde, las rutas de negocio dan **500** `No fue posible conectar con la base de datos`.

Permisos (Gateway, D8): ADMIN puede todo; USER solo lee (`PUT` y `DELETE` le dan **403**).

### Eliminar un departamento con empleados

`DELETE` no revisa empleados: este servicio no conoce a `gestion-empleados` y no hay clave foránea entre bases de datos. Decisión: los empleados ya asignados conservan su `departamentoId`; registrar un empleado **nuevo** en un departamento eliminado responde **400** desde `empleados-service`, que valida el departamento contra este servicio.

### Health check (interno)

```
GET /health
```

- **200 OK**: `{ "status": "UP", "components": { "app": "UP", "db": "UP" } }`.
- **503 Service Unavailable**: la base de datos no responde (`"db": "DOWN"`).

Lo usa el `healthcheck` de `docker-compose.yml` (del que depende `empleados-service` con `condition: service_healthy`). No es alcanzable desde fuera: el Gateway solo enruta `/departamentos/*`.

### Rutas no soportadas

Cualquier otra ruta o método responde **404** con `Recurso no encontrado`.

## Estructura del código

Capas (proyecto final, F1): el HTTP no conoce SQL y las reglas no conocen HTTP, así cada una se prueba sola.

```
public/index.php                    # Entrada: /metrics, trazas, log JSON, caos; delega en Aplicacion y emite la respuesta
src/
├── Aplicacion.php                  # Adaptador HTTP: método + ruta + cuerpo -> Respuesta (rutas, JSON, errores, /health, docs)
├── Respuesta.php                   # Valor status + cabeceras + cuerpo
├── DepartamentoService.php         # Reglas de negocio (crear, listar, obtener, actualizar, eliminar)
├── ErrorDeNegocio.php              # Regla incumplida: status, mensaje y errors[]
├── RepositorioDepartamentos.php    # Puerto de persistencia
├── DepartamentoRepository.php      # Implementación PDO (sentencias preparadas)
├── Database.php                    # Conexión PDO a MySQL (variables de entorno)
├── OpenApi.php                     # Especificación OpenAPI + HTML de Swagger UI
├── Metricas.php, Trazas.php        # Observabilidad (Reto 8)
└── cargar.php                      # require_once de todo src/ (no hay autoload en producción)
tests/                              # PHPUnit (ver Pruebas)
scripts/test_departamentos.php      # Prueba de humo de caja negra contra el sistema levantado
```

La imagen de ejecución no lleva Composer, `vendor/` ni pruebas: solo `src/` y `public/` (etapa `runner` del `Dockerfile`).

## Variables de entorno

| Variable | Descripción |
|---|---|
| `PORT` | Puerto donde escucha el servicio (8082 en Docker, no publicado al host desde el Reto 3 — ver README raíz) |
| `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASS` | Conexión a MySQL |

## Base de datos

MySQL. El esquema se crea automáticamente desde `init.sql` (montado en `/docker-entrypoint-initdb.d/`) la primera vez que el volumen `vol-departamentos` está vacío. La unicidad del `id` se garantiza con `PRIMARY KEY` en el esquema, además de una consulta previa en el código para devolver un mensaje de error descriptivo.

## Construcción y ejecución (Docker)

Desde la raíz del proyecto, junto con el resto del sistema:

```bash
docker compose up --build
```

## Documentación OpenAPI

Swagger UI en `http://localhost:8080/departamentos/docs` y la especificación en `/departamentos/openapi.json`, a través del Gateway. Viven bajo el prefijo `/departamentos` porque el servicio no publica puerto al host y el Gateway solo enruta ese prefijo (hasta el Reto 3 estaban en `/docs` y no eran alcanzables).

## Pruebas

Sin PHP en el host: todo corre en Docker.

```bash
make -C microservicios/gestion-departamentos test
```

Construye la etapa `test` del `Dockerfile` (PHPUnit 11 + pcov), exporta `reports/junit.xml` y `reports/clover.xml` y reescribe las rutas del contenedor a las del host para SonarQube. Falla si una prueba falla o si la cobertura de líneas de `src/` baja del 70 % (`tests/cobertura-minima.php`). Excluidos de la cobertura: `src/Database.php` (conexión real por entorno) y `src/cargar.php` (solo `require_once`).

| Archivo | Nivel | Qué cubre |
|---|---|---|
| `tests/DepartamentoServiceTest.php` | Unidad, repositorio en memoria | Todas las reglas y ramas de error |
| `tests/DepartamentoRepositoryTest.php` | Integración, SQLite en memoria con `init.sql` | SQL real: guardar, buscar, orden de `listar`, `actualizar`, `eliminar`, `SQLSTATE 23000` |
| `tests/AplicacionTest.php` | HTTP, servicio real + SQLite | Cada operación, 400/404/500, docs, `/health` 200/503 |
| `tests/MetricasTest.php`, `tests/TrazasTest.php`, `tests/OpenApiTest.php` | Unidad | Observabilidad y especificación |

En Jenkins, el stage Test ejecuta el mismo `make test` y publica ambos reportes; SonarQube importa los dos.

### Prueba de humo (caja negra)

`scripts/test_departamentos.php` recorre el servicio vía HTTP a través del Gateway. No es parte de `make test`: requiere el sistema levantado (`docker compose up --build`) y PHP en el host.

```bash
php microservicios/gestion-departamentos/scripts/test_departamentos.php
TEST_BASE_URL=http://localhost:8080 php microservicios/gestion-departamentos/scripts/test_departamentos.php
```

El escenario de punta a punta de `PUT`/`DELETE` y de los permisos está en `e2e-tests/features/departamentos/crud.feature`.

## Pendientes conocidos

- No tiene ejecución local documentada sin Docker (requiere PHP 8.2+ y una instancia de MySQL accesible).
