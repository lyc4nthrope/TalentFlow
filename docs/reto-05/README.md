# Reto 5 — Manual paso a paso

Cómo levantar el sistema, dónde ver el estado de cada componente y cómo demostrar, en orden, todo lo que pide `reto5.pdf`. URL base de la API: **`http://localhost:8080`** · administrador semilla: **`admin@empresa.com` / `Admin123*`** · UI del broker: **`http://localhost:15672`** (`admin` / `admin`) · correos (tokens de activación y recuperación): **`http://localhost:8025`**.

| Archivo | Para qué |
|---|---|
| **Este README** | Manual: entregables → dónde se demuestran, estado de cada componente, los 18 pasos, preguntas para la sustentación, problemas comunes |
| [`demo.sh`](demo.sh) | Ejecuta automáticamente los 18 pasos del "Flujo de prueba sugerido" del PDF (~5 min) |
| [`pruebas.md`](pruebas.md) | Evidencia: resultado de cada paso, desviaciones respecto al PDF, log del caso borde y salida completa de `demo.sh` |
| [`bruno/`](bruno/) | Colección Bruno del Reto 5 (41 peticiones con tests, entorno `Local`) |
| [`PLAN.md`](PLAN.md) | Diagnóstico de partida, decisiones de diseño (D1-D23) y aprendizajes de cada fase |
| [README raíz](../../README.md#seguridad-e-identidad-reto-5) | Documento de entrega: justificaciones completas y diagramas |

## 0. Entregables del PDF → dónde se demuestran

| Entregable o consideración del PDF | Dónde |
|---|---|
| `docker-compose.yml` con `auth-service` y el API Gateway | [`docker-compose.yml`](../../docker-compose.yml) (`database-auth`, `auth-service`, `api-gateway`) · §2 |
| Cómo obtener un token para pruebas locales (Postman/Bruno) | [README raíz: cómo obtener un token](../../README.md#cómo-obtener-un-token-para-pruebas-locales) · [`bruno/`](bruno/) · §4 |
| Estrategia de validación (Gateway vs. interceptores) y por qué | [README raíz: estrategia de validación](../../README.md#estrategia-de-validación-en-el-gateway-y-no-en-cada-servicio) · §5 pasos 4, 7, 9 y 10 |
| Secreto del JWT documentado y definido en `.env.example` | [README raíz: secreto del JWT](../../README.md#secreto-del-jwt) · [`.env.example`](../../.env.example) |
| Diagrama de secuencia del ciclo de vida de la cuenta | [README raíz: ciclo de vida de la cuenta](../../README.md#ciclo-de-vida-de-la-cuenta) |
| Mecanismo y frecuencia del scheduler, configuración y cómo demostrarlo sin esperar días | [README raíz: scheduler de vacaciones](../../README.md#scheduler-de-vacaciones) · §5 pasos 12 a 15 |
| Limitación del scheduler con N instancias y su solución (Reto 31, ShedLock) | [README raíz: el scheduler con N instancias](../../README.md#limitación-conocida-el-scheduler-con-n-instancias) |
| Evidencia del caso borde (retiro durante las vacaciones) | [README raíz: evidencia del caso borde](../../README.md#evidencia-del-caso-borde-retiro-durante-las-vacaciones) · [`pruebas.md`](pruebas.md#evidencia-del-caso-borde-paso-15) · §5 paso 15 |
| Mismo secreto (HMAC SHA-256) en quien firma y quien valida | [README raíz: secreto del JWT](../../README.md#secreto-del-jwt) (ancla `x-jwt`) |
| Secreto inyectado por variables de entorno en Compose | `<<: *jwt` en `auth-service` y `api-gateway`; sin `JWT_SECRET` ninguno de los dos arranca |
| Swagger con el esquema `bearerAuth` | §3 (Swagger de los 6 servicios) · botón **Authorize** |

## 1. Prerrequisitos

- Docker con Docker Compose v2 y ~6 GB libres en disco (el primer build compila Java, Go, Python y Node).
- Puertos **8080**, **15672** y **8025** libres en el host.
- `curl` y `python3` para `demo.sh`; Node.js (`npx`) para la colección Bruno por línea de comandos.
- **Conexión a internet** para ver Swagger UI: auth, departamentos, notificaciones y vacaciones cargan `swagger-ui` 5.11.0 desde un CDN, y perfiles (FastAPI) también lo toma de un CDN. Solo empleados sirve sus propios archivos (`swagger-ui-express`).
- Volúmenes limpios si vienes del Reto 4: cambiaron los esquemas de `db-vacaciones` (fechas con hora) y de `db-auth`, y cada `init.sql` / `schema.sql` solo corre con un volumen nuevo:

```bash
docker compose down -v
```

## 2. Levantar el sistema

```bash
docker compose up --build -d
docker compose ps       # ~40 s: 14 contenedores (healthy)
```

Son 16 contenedores: los 14 con healthcheck quedan `(healthy)` (6 bases de datos, el broker, 6 servicios y el Gateway); `mailhog` aparece `Up` sin healthcheck y `broker-init` como `Exited (0)` (importa la topología de eventos y termina). Al arrancar, `auth-service` crea el administrador semilla:

```bash
docker compose logs auth-service | grep "administrador semilla"
# auth-service: administrador semilla creado (admin@empresa.com)
# (en los arranques siguientes: "auth-service: el administrador semilla ya existe")
```

## 3. ¿Dónde veo el estado de cada cosa?

| Qué | Cómo |
|---|---|
| **Todo el sistema** | `curl http://localhost:8080/health` (público) → `sistema: OK/DEGRADADO`, `problemas`, y por servicio (los 6, incluido `auth-service`): `status`, `db`, `broker` |
| Las cuentas y sus transiciones | `docker compose logs -f auth-service` (cuenta creada, suspendida, reactivada, retirada, "no se reactiva") |
| El scheduler | `docker compose logs -f vacaciones-service \| grep -E "Evento publicado\|ciclo de vacaciones"` |
| Las notificaciones (con el token completo) | `docker compose logs -f notificaciones-service \| grep NOTIFICACIÓN` |
| Los correos | Mailhog: `http://localhost:8025` |
| Colas y mensajes | UI de RabbitMQ `http://localhost:15672` → **Queues** (`auth.eventos`, `notificaciones.eventos`…) y **Exchanges** → `talentflow.eventos` (bindings) |

Swagger UI de cada servicio, a través del Gateway (rutas públicas; botón **Authorize** con el JWT, sin el prefijo `Bearer`):

| Servicio | Swagger UI |
|---|---|
| auth | `http://localhost:8080/auth/docs` |
| empleados | `http://localhost:8080/empleados/docs` |
| departamentos | `http://localhost:8080/departamentos/docs` |
| perfiles | `http://localhost:8080/perfiles/docs` |
| notificaciones | `http://localhost:8080/notificaciones/docs` |
| vacaciones | `http://localhost:8080/vacaciones/docs` |

## 4. Obtener un token

Los comandos completos (administrador semilla, activación de un empleado nuevo con el token de la notificación, uso en curl, Swagger y Bruno) están en el [README raíz](../../README.md#cómo-obtener-un-token-para-pruebas-locales). En resumen:

```bash
TOKEN_ADMIN=$(curl -s -X POST http://localhost:8080/auth/login -H "Content-Type: application/json" \
  -d '{"email": "admin@empresa.com", "password": "Admin123*"}' | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])')
curl -H "Authorization: Bearer $TOKEN_ADMIN" http://localhost:8080/empleados
```

## 5. Los 18 pasos del PDF

Automático (el sistema debe estar levantado; se puede repetir sobre una base con datos):

```bash
bash docs/reto-05/demo.sh                                    # ~5 min; código 0 solo si todo pasa
cd docs/reto-05/bruno && npx @usebruno/cli run --env Local   # 41 peticiones con tests; ~5 min
```

Los pasos 13 a 15 esperan los minutos reales del scheduler (corre en el segundo 0 de cada minuto): no es un cuelgue. Lo que exige cada paso y cómo se comprueba a mano (el número de la petición Bruno equivalente entre corchetes):

| Paso | Petición | Esperado | Cómo comprobarlo |
|---|---|---|---|
| 1 | `docker compose up --build -d` | Todo sano | `GET /health` → `sistema: OK` [01] |
| 2 | Login del admin semilla; `POST /departamentos` IT; `POST /empleados` E001 y E002 con su JWT | 200; 201 | [02-05] |
| 3 | — | `empleado.creado` en auth-service; `usuario.creado` en notificaciones-service | `docker compose logs auth-service \| grep "empleado.creado recibido"` y `docker compose logs notificaciones-service \| grep "Tipo: SEGURIDAD"` [06] |
| 4 | `GET /empleados` sin cabeceras | **401** | [07] |
| 5 | `POST /auth/reset-password` con el token del log | 200; reutilizarlo → 401 | Token en la línea `[NOTIFICACIÓN]` o en Mailhog [08-11] |
| 6 | `POST /auth/login` de E001 | 200, JWT con `role: USER`, `sub: E001` | [12] |
| 7 | `GET /empleados` con `Authorization: Bearer <JWT USER>` | **200** | [13] |
| 8 | `POST /auth/recover-password`; repetir 5 y 6 con el token nuevo | 202; token distinto; reset 200; login 200 | `grep "Para: <email>"` en notificaciones [14-17] |
| 9 | `DELETE /empleados/E001` con el JWT de USER | **403**; E001 sigue `ACTIVO` | [18] |
| 10 | `PUT /perfiles/E001` y `PUT /perfiles/E002` con el JWT de E001 | **200** / **403** (propiedad) | [19-20] |
| 11 | `POST /auth/change-password` | 200; contraseña antigua → 401; nueva → 200 | [21-23] |
| 12 | `POST /vacaciones` de E001 con el JWT de ADMIN | 201; `vacaciones.programadas`; notificación `VACACIONES` | Fechas con **hora**: inicio en el próximo minuto, fin 1 minuto después ([por qué](pruebas.md#desviaciones-respecto-al-pdf)) [24-25] |
| 13 | — (esperar al scheduler) | En orden: `vacaciones.iniciadas` → cuenta suspendida → `cuenta.desactivada` (`VACACIONES`); login → 401 | Logs de vacaciones, auth y notificaciones [26-28] |
| 14 | — (esperar la `fechaFin`) | `vacaciones.finalizadas` → `cuenta.activada` (`FIN_VACACIONES`, "Bienvenido de regreso"); login → 200 | [29-31] |
| 15 | Vacaciones de E002; cuando inicien, `DELETE /empleados/E002` con ADMIN | **Caso borde**: al llegar la `fechaFin` la cuenta **no** se reactiva; login sigue en 401 | `docker compose logs auth-service \| grep "no se reactiva"` [32-38] |
| 16 | `DELETE /empleados/E001` con ADMIN | 200, `RETIRADO`, `empleado.retirado` | [39] |
| 17 | Login de E001 | 401 | [40] |
| 18 | `GET /empleados?estado=RETIRADO` | Lista a E001 con `fechaRetiro` | [41] |

Resultado real de cada paso y salida completa de la última corrida: [`pruebas.md`](pruebas.md#resumen-cada-paso-del-pdf).

## 6. Preguntas probables en la sustentación

| Pregunta | Respuesta corta | Dónde mostrarlo |
|---|---|---|
| ¿Por qué validar en el Gateway y no en cada servicio? | Una sola implementación y una sola matriz de permisos para seis servicios en cinco lenguajes; los servicios del Reto 4 no cambian. Costo: los servicios confían en la red interna | [README raíz](../../README.md#estrategia-de-validación-en-el-gateway-y-no-en-cada-servicio) |
| ¿Qué impide que un cliente envíe `X-User-Role: ADMIN`? | El Gateway borra las cabeceras `X-User-*` de toda petición y solo las agrega tras validar el token | `microservicios/api-gateway/app.js` |
| ¿401 o 403? | 401: no sabemos quién eres (sin token, inválido o vencido; credenciales malas). 403: sabemos quién eres, pero no puedes (USER escribiendo, perfil ajeno) | §5 pasos 4, 9 y 10 |
| ¿Dónde está el secreto y por qué HS256? | Una sola vez en el ancla `x-jwt` de Compose, recibido por quien firma y quien valida; `.env.example` lo documenta. HS256 es lo que permite el PDF por simplicidad; en el Reto 10, JWKS con clave asimétrica | [README raíz](../../README.md#secreto-del-jwt) |
| ¿Qué pasa si un servicio arranca sin `JWT_SECRET`? | No arranca: no hay valor por defecto en el código | `index.js` del Gateway, `config/config.go` de auth |
| ¿Cómo se crea la cuenta de un empleado? | No hay registro: `empleado.creado` → cuenta `PENDIENTE_ACTIVACION` sin contraseña + `usuario.creado` con un token de un solo uso; el empleado fija su contraseña con `reset-password` | [Diagrama de secuencia](../../README.md#ciclo-de-vida-de-la-cuenta) |
| ¿Por qué el token no aparece en `GET /notificaciones`? | Cualquier USER puede leer ese historial: quien viera el token podría tomar la cuenta. Va completo solo en el log `[NOTIFICACIÓN]` y en el correo; en la API sale enmascarado | `GET /notificaciones` |
| ¿Por qué cuatro estados y no un booleano? | Hay que distinguir la suspensión temporal (vacaciones, se reactiva) de la baja permanente (retiro, nunca se reactiva) | [Tabla de estados](../../README.md#ciclo-de-vida-de-la-cuenta) |
| ¿Cómo evitan que el fin de las vacaciones reactive a un retirado? | `vacaciones.finalizadas` solo reactiva una cuenta `SUSPENDIDA_TEMPORAL` por esas mismas vacaciones; la BD lo refuerza con un `UPDATE` condicional | §5 paso 15 · [log](pruebas.md#evidencia-del-caso-borde-paso-15) |
| ¿Cómo se demuestra sin esperar días? | Las fechas llevan hora: un período que empieza en 1 minuto y dura 1 o 2; el scheduler corre cada minuto | [Scheduler](../../README.md#scheduler-de-vacaciones) |
| ¿Qué pasa con 3 instancias de vacaciones? | Las tres ejecutan el job; la actualización condicional deja pasar a una sola por período, así que no hay eventos duplicados, pero el trabajo se repite y nada coordina tareas no condicionales. Se resuelve con ShedLock (Reto 31) | [Limitación](../../README.md#limitación-conocida-el-scheduler-con-n-instancias) |
| Si retiro a un empleado, ¿su JWT deja de funcionar? | Su login falla de inmediato (401), pero un JWT ya emitido vale hasta su `exp` (máximo 1 h): el JWT no guarda estado. Revocarlo exigiría una lista de revocación | [Costos de la estrategia](../../README.md#estrategia-de-validación-en-el-gateway-y-no-en-cada-servicio) |
| ¿Por qué `recover-password` responde 202 aunque el email no exista? | Para no revelar qué emails tienen cuenta | `POST /auth/recover-password` |

## 7. Problemas comunes

| Síntoma | Causa y solución |
|---|---|
| Swagger UI sale en blanco | Todas las páginas salvo la de empleados cargan `swagger-ui` desde un CDN: requiere internet. La especificación sigue disponible en `/<servicio>/openapi.json` |
| Todo responde 401 tras un rato | El JWT venció (`JWT_VIGENCIA`, 1 h): volver a hacer login. En Swagger, **Logout** y **Authorize** con el token nuevo |
| En `/empleados/docs` hay que volver a autorizar tras recargar | Es la única página sin `persistAuthorization` (`swagger-ui-express` sin opciones); en las demás el token sobrevive a recargar |
| 401 con un token recién copiado | Se pegó con el prefijo `Bearer` en Swagger (lo agrega solo), o con comillas; o el token es de antes de cambiar `JWT_SECRET` |
| 403 en vez de 401 | El token es válido pero el rol no alcanza (USER escribiendo) o el perfil es de otro empleado: es lo esperado |
| `reset-password` responde 401 | El token ya se usó, venció (activación 24 h, recuperación 1 h) o fue reemplazado por un `recover-password` posterior |
| El login de un empleado recién activado responde 401 | La cuenta no está `ACTIVA`: aún pendiente, de vacaciones o retirada (`docker compose logs auth-service \| grep <empleadoId>`) |
| No llega el token del empleado | Si `auth-service` o notificaciones-service estaban detenidos, el evento espera en su cola (durable) y se procesa al volver: ver la pestaña **Queues** y `docker compose logs auth-service notificaciones-service` |
| `POST /vacaciones` responde 400 "en el pasado" | La `fechaInicio` debe ser posterior a **ahora** en `America/Bogota` (no en UTC) |
| El paso 13 o 14 "se queda esperando" | El scheduler solo corre en el segundo 0 de cada minuto; `demo.sh` informa la espera. Cron más rápido: `VACACIONES_CRON="*/10 * * * * *" docker compose up -d vacaciones-service` |
| `auth-service` o `vacaciones-service` no arrancan tras actualizar la rama | Volumen de una versión anterior: `docker compose down -v` y volver a levantar |
| `auth-service` no crea el administrador semilla | `ADMIN_PASSWORD` no cumple la política (8 a 72 caracteres, mayúscula, minúscula y dígito) o `ADMIN_EMAIL` ya pertenece a otra cuenta: lo dice su log |
