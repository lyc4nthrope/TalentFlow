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

- **Estados:** `PENDIENTE_ACTIVACION` → `ACTIVA`. Además existen `SUSPENDIDA_TEMPORAL` (vacaciones) y `DESACTIVADA_PERMANENTE` (retiro). Solo `ACTIVA` puede iniciar sesión.
- **Tokens de activación y recuperación:**
  - Son UUID aleatorios de un solo uso.
  - Solo se guarda su hash SHA-256, junto con el vencimiento: 24 h la activación y 1 h la recuperación.
  - Hay un token activo por cuenta: pedir otro anula el anterior.
- **Token de acceso:** JWT HS256 con `sub` (el `empleadoId`, o `admin`), `role` (`ADMIN` | `USER`), `email`, `iss`, `iat` y `exp`. Vence en 1 h.
- **Política de contraseñas:** de 8 a 72 bytes, con al menos una mayúscula, una minúscula y un dígito. Nunca se recortan espacios.

## Configuración

| Variable | Por defecto | Uso |
|---|---|---|
| `JWT_SECRET` | **obligatoria** (el servicio no arranca sin ella) | Clave HMAC, compartida con el Gateway |
| `JWT_ISSUER` / `JWT_VIGENCIA` | `auth-service` / `1h` | Emisor y vigencia del token de acceso |
| `TOKEN_ACTIVACION_VIGENCIA` / `TOKEN_RECUPERACION_VIGENCIA` | `24h` / `1h` | Vigencia de los tokens de activación y recuperación |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | sin valor | Administrador semilla, creado al arrancar si no existe. La contraseña debe cumplir la política y nunca se escribe en el log |
| `DB_*`, `RABBITMQ_*`, `AMQP_EXCHANGE` | ver `config/config.go` | Conexiones |

## Comportamientos que conviene conocer

- **Cambiar la contraseña no invalida los JWT ya emitidos.** Los tokens son *stateless* y siguen siendo válidos hasta su `exp` (como máximo 1 h). Lo que sí se anula es cualquier token de recuperación pendiente.
- **La recuperación sobre una cuenta pendiente la activa.** Si un empleado pierde el correo de bienvenida, puede pedir una recuperación y usar ese token: la cuenta pasa a `ACTIVA` y se publica `cuenta.activada`. En una cuenta suspendida la contraseña cambia pero la suspensión se mantiene. Una cuenta retirada no recibe tokens.
- **`recover-password` responde antes de hacer el trabajo.** La búsqueda, el token y el evento se procesan en segundo plano (plazo de 10 s). Así ni la respuesta ni su tiempo revelan qué emails existen; los errores solo quedan en el log.
- **Un evento puede perderse si el broker falla.** Los eventos se publican después de guardar en la BD, con confirmación del broker pero con `mandatory=false`. Si el broker no está disponible, o no hay una cola ligada a esa routing key, el error queda en el log y el cambio no se revierte. Es la misma limitación aceptada en el Reto 4; el patrón Outbox llega en retos posteriores.
- **Un volumen existente hay que recrearlo.** `schema.sql` solo se ejecuta con un volumen nuevo, y la tabla cambió en el Reto 5:

  ```
  docker compose rm -sf auth-service database-auth
  docker volume rm talentflow_vol-auth
  ```

## Pruebas

```
go test -race ./...
```

Las pruebas de `internal/infra/postgres` corren contra un PostgreSQL real solo si se define `AUTH_TEST_DATABASE_URL`; en ese archivo están las instrucciones.
