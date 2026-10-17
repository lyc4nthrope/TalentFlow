# auth-service (Reto 5)

Servicio de identidad del ecosistema, en **Go** con **PostgreSQL** propia. Emite el token de acceso (JWT) y maneja las cuentas de los empleados. No hay registro público: cada cuenta nace del evento `empleado.creado`.

## Endpoints

Todos se usan a través del Gateway (`http://localhost:8080`).

| Método | Ruta | Descripción | Respuesta |
|---|---|---|---|
| POST | `/auth/login` | `{email, password}` → token de acceso | 200 `{token, tipo}` · 401 credenciales inválidas |
| POST | `/auth/recover-password` | `{email}` → publica `usuario.recuperacion` con un token nuevo | **202 siempre** (exista o no la cuenta) |
| POST | `/auth/reset-password` | `{token, newPassword}` → fija la contraseña con un token de activación o recuperación | 200 · 400 política · 401 token inválido o vencido |
| POST | `/auth/change-password` | `{currentPassword, newPassword}` con `Authorization: Bearer <token>` | 200 · 400 contraseña actual incorrecta o política · 401 token de acceso inválido |

Los errores usan el formato común del ecosistema (`status`, `error`, `message`, `timestamp`, `path`) y nunca exponen detalles internos.

## Modelo

- **Estados:** `PENDIENTE_ACTIVACION` → `ACTIVA`. Además existen `SUSPENDIDA_TEMPORAL` (vacaciones) y `DESACTIVADA_PERMANENTE` (retiro); ver [Ciclo de vida por eventos](#ciclo-de-vida-por-eventos). Solo `ACTIVA` puede iniciar sesión.
- **Tokens de activación y recuperación:**
  - Son UUID aleatorios de un solo uso.
  - Solo se guarda su hash SHA-256, junto con el vencimiento: 24 h la activación y 1 h la recuperación.
  - Hay un token activo por cuenta: pedir otro anula el anterior.
- **Token de acceso:** JWT HS256 con `sub` (el `empleadoId`, o `admin`), `role` (`ADMIN` | `USER`), `email`, `iss`, `iat` y `exp`. Vence en 1 h.
- **Política de contraseñas:** de 8 a 72 bytes, con al menos una mayúscula, una minúscula y un dígito. Nunca se recortan espacios.

## Ciclo de vida por eventos

La cola `auth.eventos` escucha cuatro eventos. Cada uno se aplica en una sola transacción junto con el registro de su `id` en `eventos_procesados`: un mensaje repetido no vuelve a aplicarse ni a publicar. El evento de salida se publica después del commit y **solo si el estado cambió**.

| Evento recibido | Estado de la cuenta | Resultado | Evento publicado |
|---|---|---|---|
| `empleado.creado` | no existe | Se crea en `PENDIENTE_ACTIVACION`, sin contraseña, con token de activación | `usuario.creado` |
| `empleado.retirado` | cualquiera salvo retirada | `DESACTIVADA_PERMANENTE`; se borran el token pendiente y la suspensión | `cuenta.desactivada` (`RETIRO`, `permanente: true`) |
| `vacaciones.iniciadas` | `ACTIVA` | `SUSPENDIDA_TEMPORAL`, recordando el `vacacionesId` | `cuenta.desactivada` (`VACACIONES`, `permanente: false`) |
| `vacaciones.iniciadas` | `SUSPENDIDA_TEMPORAL` | Solo cambia el `vacacionesId` (períodos contiguos) | — |
| `vacaciones.iniciadas` | pendiente o retirada | Sin cambios | — |
| `vacaciones.finalizadas` | suspendida por **esas** vacaciones | `ACTIVA` | `cuenta.activada` (`FIN_VACACIONES`) |
| `vacaciones.finalizadas` | retirada, activa, o suspendida por otras vacaciones | Sin cambios | — |

- **Caso borde (retirado durante sus vacaciones):** el retiro deja la cuenta en `DESACTIVADA_PERMANENTE`; cuando llega `vacaciones.finalizadas`, la cuenta no se reactiva y el log dice "cuenta retirada: no se reactiva". La BD lo garantiza también: la reactivación es un `UPDATE ... WHERE estado = 'SUSPENDIDA_TEMPORAL' AND vacaciones_suspension = <id>`.
- **Por qué se recuerda el `vacacionesId`:** con dos períodos contiguos el scheduler publica en la misma pasada el inicio del segundo antes que el fin del primero. Sin el id, el fin del primero reactivaría la cuenta durante el segundo.
- **Mensajes que se descartan:** payload ilegible, datos inválidos o rechazados por la BD, y tipos desconocidos (`ack` + log). Un evento de un empleado sin cuenta se registra y no hace nada. Solo un fallo de conexión con la BD se reintenta.
- **Un evento nunca toca al administrador:** las cuentas se buscan por `empleado_id`, que el administrador no tiene. Un `empleado.creado` con el id reservado `admin`, o que choca con otra cuenta que no es la suya, se rechaza con un log explícito.
- **Una cuenta pendiente no se suspende:** si llega `vacaciones.iniciadas` antes de que el empleado active su cuenta, no hay nada que suspender. Si la activa durante las vacaciones, queda `ACTIVA` hasta el siguiente ciclo. Es intencional (D19): una cuenta no puede reactivarse sin contraseña.
- **El orden importa, por eso el prefetch es 1:** con más mensajes entregados a la vez, uno que falla de forma transitoria se reencola detrás de los demás. `vacaciones.finalizadas` procesado antes que su `vacaciones.iniciadas` dejaría la cuenta suspendida para siempre, y `empleado.retirado` antes que su `empleado.creado` le daría cuenta a un retirado. Con prefetch 1 el mensaje reencolado vuelve a la cabeza. El volumen de eventos de cuentas es bajo, así que el rendimiento no es una preocupación.
- **Manejadores concurrentes de la misma cuenta se serializan:** dentro de la transacción la cuenta se lee con `SELECT ... FOR UPDATE`, así cada uno decide con el estado que dejó el otro (pensado para varias réplicas, Reto 11).

## Configuración

| Variable | Por defecto | Uso |
|---|---|---|
| `JWT_SECRET` | **obligatoria** (el servicio no arranca sin ella) | Clave HMAC, compartida con el Gateway |
| `JWT_ISSUER` / `JWT_VIGENCIA` | `auth-service` / `1h` | Emisor y vigencia del token de acceso |
| `TOKEN_ACTIVACION_VIGENCIA` / `TOKEN_RECUPERACION_VIGENCIA` | `24h` / `1h` | Vigencia de los tokens de activación y recuperación |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | sin valor | Administrador semilla, creado al arrancar si no existe. La contraseña debe cumplir la política y nunca se escribe en el log |
| `DB_USER` / `DB_PASS` | **obligatorias** (salvo que llegue `DATABASE_URL` completa) | Credenciales de la BD; sin valor por defecto en el código |
| `RABBITMQ_USER` / `RABBITMQ_PASS` | **obligatorias** (salvo que llegue `AMQP_URL` completa) | Credenciales del broker; sin valor por defecto en el código |
| `DB_HOST` / `DB_PORT` / `DB_NAME`, `RABBITMQ_HOST`, `AMQP_EXCHANGE`, `PORT` | `localhost` / `5432` / `db_auth`, `localhost`, `talentflow.eventos`, `8086` | Ubicación (no secretos). Usuario y contraseña se codifican en la URL de conexión |

Si falta una variable obligatoria, el servicio escribe cuál en el log y termina con código 1.

## Salud

- `GET /health` (interno, lo consultan el `/health` agregado del Gateway y Docker): `{status, service, timestamp, components: {app, db, broker}}`. Responde 200 si la BD contesta en menos de 2 s y 503 si no. El broker se informa pero no cambia el estado: sin broker el servicio sigue atendiendo login y contraseñas, y el consumidor se reconecta solo.
- Healthcheck de Docker: `/auth-service -salud`. La imagen distroless no tiene shell ni `wget`, así que el propio binario consulta `http://127.0.0.1:$PORT/health` (plazo de 2 s) y termina con 0 si responde 200, o con 1 en cualquier otro caso.
- El servicio no publica su puerto al host: solo se alcanza a través del Gateway (`expose: 8086`).

## Comportamientos que conviene conocer

- **Cambiar la contraseña no invalida los JWT ya emitidos.** Los tokens son *stateless* y siguen siendo válidos hasta su `exp` (como máximo 1 h). Lo que sí se anula es cualquier token de recuperación pendiente.
- **La recuperación sobre una cuenta pendiente la activa.** Si un empleado pierde el correo de bienvenida, puede pedir una recuperación y usar ese token: la cuenta pasa a `ACTIVA` y se publica `cuenta.activada`. En una cuenta suspendida la contraseña cambia pero la suspensión se mantiene. Una cuenta retirada no recibe tokens.
- **`recover-password` responde antes de hacer el trabajo.** La búsqueda, el token y el evento se procesan en segundo plano (plazo de 10 s). Así ni la respuesta ni su tiempo revelan qué emails existen; los errores solo quedan en el log.
- **Un evento puede perderse si el broker falla.** Los eventos se publican después de guardar en la BD, con confirmación del broker pero con `mandatory=false`. Si el broker no está disponible, o no hay una cola ligada a esa routing key, el error queda en el log y el cambio no se revierte. Es la misma limitación aceptada en el Reto 4; el patrón Outbox llega en retos posteriores.
- **Un volumen existente hay que recrearlo.** `schema.sql` solo se ejecuta con un volumen nuevo, y la tabla cambió en el Reto 5 (F2 y F3):

  ```
  docker compose rm -sf auth-service database-auth
  docker volume rm talentflow_vol-auth
  ```

## Pruebas

```
go test -race ./...
```

Las pruebas de `internal/infra/postgres` corren contra un PostgreSQL real solo si se define `AUTH_TEST_DATABASE_URL`; en ese archivo están las instrucciones.
