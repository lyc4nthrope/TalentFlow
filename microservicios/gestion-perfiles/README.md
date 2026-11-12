# Servicio de Gestión de Perfiles (Python + FastAPI)

Microservicio del Reto 4 que **combina comunicación asincrónica y sincrónica**: consume eventos para crear y mantener los perfiles automáticamente, y expone REST para consultarlos y actualizarlos. Base de datos propia (PostgreSQL).

## Eventos que consume (cola `perfiles.eventos`)

| Evento (Catálogo) | Efecto |
|---|---|
| `empleado.creado` (3.1) | Crea el perfil por defecto: `nombre` = "nombre apellido", `email`, y `telefono`/`direccion`/`ciudad`/`biografia` vacíos |
| `empleado.actualizado` (3.2) | Sincroniza los campos replicados (`nombre`, `email`); los datos propios del perfil no se tocan. Si el perfil no existe (su `empleado.creado` se perdió), lo crea |
| `empleado.retirado` (3.3) | **Archiva** el perfil (`archivado: true`, `fechaArchivado`); no lo borra |

Deduplicación por `id` de mensaje (tabla `eventos_procesados`) en la misma transacción que el efecto; `ack` tras el commit; mensajes inválidos o rechazados por la BD se descartan con log; fallos transitorios se reencolan tras 5 s. `aio-pika` (`connect_robust`) reconecta solo al broker.

## Endpoints

| Método | Ruta | Respuestas |
|---|---|---|
| GET | `/perfiles` | 200 con todos los perfiles |
| GET | `/perfiles/{empleadoId}` | 200 · 404 con mensaje descriptivo |
| PUT | `/perfiles/{empleadoId}` | 200 con el perfil actualizado · 400 · 404 · 409 si está archivado |
| GET | `/perfiles/docs` · `/perfiles/openapi.json` | Swagger UI / especificación |
| GET | `/health` | Interno: `status`, `db`, `broker` |

`PUT` es una **actualización parcial** de los campos que gestiona el empleado (`telefono`, `direccion`, `ciudad`, `biografia` y, desde el proyecto final, `foto`, `redesSociales`, `codigoPostal`): lo no enviado no cambia (el reto envía solo algunos). `nombre` y `email` **no** se editan aquí — los replica empleados-service — y cualquier otro campo responde `400` (evita la asignación masiva).

| Campo (proyecto final) | Regla |
|---|---|
| `foto` | URL `http`/`https` con host, máx. 500; `""` la quita. `javascript:`, `data:`, `ftp:` → `400` (evita XSS al pintarla) |
| `redesSociales` | Objeto `{linkedin, github, twitter}`, cada una URL `http`/`https` máx. 300; otra clave → `400`. Si se envía **reemplaza** el objeto guardado: las redes no enviadas quedan `null` |
| `codigoPostal` | Máx. 10, letras, dígitos, espacio y guion |

La respuesta siempre trae `foto`, `codigoPostal` y `redesSociales` con sus tres claves (`null` = no registrada).

> **Base de datos existente:** `init.sql` solo corre con un volumen nuevo. Al arrancar, `RepositorioPostgres.migrar()` añade las columnas `foto`, `codigo_postal` y `redes_sociales` con `ADD COLUMN IF NOT EXISTS` (idempotente, con un candado `pg_advisory_xact_lock` por si arrancan varias réplicas), así un volumen anterior no hay que borrarlo.

## Estructura del código

```
app/
├── main.py          # arranque: pool de BD, consumidor en segundo plano, cierre ordenado
├── config.py        # configuración desde variables de entorno
├── eventos.py       # envelope del catálogo (Pydantic)
├── dominio.py       # Perfil, cargas útiles de los eventos, reglas de actualización
├── aplicacion.py    # caso de uso: deduplicar y aplicar el evento (puerto RepositorioPerfiles)
├── repositorio.py   # PostgreSQL (psycopg 3): transacción de deduplicación, SQL compuesto con psycopg.sql
├── consumidor.py    # consumidor RabbitMQ (aio-pika)
└── api.py           # REST (FastAPI) y formato de errores del ecosistema
```

## Variables de entorno

| Variable | Descripción |
|---|---|
| `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASS` | PostgreSQL propio |
| `RABBITMQ_HOST`, `RABBITMQ_PORT`, `RABBITMQ_USER`, `RABBITMQ_PASS` | Broker |
| `RABBITMQ_COLA` | Cola a consumir (`perfiles.eventos`) |

## Pruebas y ejecución

```bash
python -m venv .venv && . .venv/bin/activate
pip install -r requirements-dev.txt
make test                      # 73 pruebas (eventos, deduplicación, reglas de actualización, API) y cobertura al 100 %
ruff check app tests           # estilo + reglas de seguridad (configuración en pyproject.toml)
```

Imagen: `python:3.13-slim`, multi-stage, usuario sin privilegios (uid 10001).
