# Reto 5 — Pruebas del flujo completo

Resultados reales del "Flujo de prueba sugerido" de `reto5.pdf` (18 pasos), obtenidos con [`demo.sh`](demo.sh) y con la colección [Bruno](bruno/) sobre el sistema levantado en la rama `lyc4nthrope`, el 2026-10-16. La salida de la última corrida está al final.

## Cómo ejecutarlo

```bash
docker compose up --build -d          # el script no levanta el sistema (paso 1 del PDF)
bash docs/reto-05/demo.sh             # ~5 min; código de salida 0 solo si todo pasa
```

Variables opcionales: `BASE_URL`, `ADMIN_EMAIL` / `ADMIN_PASSWORD` (por defecto los de `docker-compose.yml`: `admin@empresa.com` / `Admin123*`), `MAILHOG_UI`, `ID_E001` / `ID_E002`, `DURACION_E001` (60 s) y `DURACION_E002` (120 s).

Colección Bruno (41 peticiones con tests, una o varias por paso del PDF):

```bash
cd docs/reto-05/bruno && npx @usebruno/cli run --env Local
```

Las peticiones de los pasos 13 a 15 esperan (`bru.sleep`) hasta el minuto en que corre el scheduler: la colección tarda también unos 5 minutos. Bruno no lee los logs: el token se toma del correo que recibe Mailhog (mismo texto que la línea `[NOTIFICACIÓN]`), y las comprobaciones de los logs quedan como comandos en el bloque *Docs* de cada petición.

### Tiempos

El scheduler corre cada minuto en el segundo 0 (`VACACIONES_CRON=0 * * * * *`). El script programa cada período con **inicio en el próximo minuto exacto** (a 20 s o más) y fin 1 minuto (E001) o 2 minutos (E002) después:

| Tramo | Espera típica |
|---|---|
| Pasos 1 a 12 | ~5 s |
| Paso 13 (inicio de E001) | 20 a 80 s |
| Paso 14 (fin de E001) | 60 s |
| Paso 15 (inicio de E002, retiro, fin) | 20 a 80 s + 120 s |
| **Total** | **4,5 a 5,5 min** |

Con un cron más frecuente (`VACACIONES_CRON="*/10 * * * * *" docker compose up -d vacaciones-service`) el script funciona igual, pero las esperas las marcan los minutos exactos del inicio, así que el ahorro es poco.

## Desviaciones respecto al PDF

| Paso | PDF | Aquí | Por qué |
|---|---|---|---|
| 2 | `POST /empleados` abierto o con el admin semilla | Con el JWT del administrador semilla (`ADMIN_EMAIL` / `ADMIN_PASSWORD`, D15) | El endpoint está protegido (D8); el PDF prevé este caso |
| 2 | Empleados E001 y E002 | E001 y E002 si están libres; si no, `E001-<sufijo>` y `E002-<sufijo>` | Los ids los envía el cliente y son únicos: así el script se puede repetir sobre una BD con datos. Emails y números de empleado llevan siempre el sufijo |
| 5 | Activar al empleado | Se activan E001 **y E002** | En el paso 15 solo una cuenta `ACTIVA` se suspende por vacaciones (D19) |
| 12 | `fechaInicio = fechaFin = hoy` | Fecha **y hora** de hoy: inicio en el próximo minuto, fin 1 minuto después | Con fecha sola el fin es las 23:59:59 (D6): la cuenta quedaría suspendida el resto del día y el paso 14 no se vería |
| 13 | Verificar el orden de los logs | Enlace por ids + marcas de tiempo de Docker | auth-service nombra el `vacacionesId` (solo viaja en `vacaciones.iniciadas`) y notificaciones-service el `eventoId`. vacaciones-service escribe "Evento publicado" **después** de la confirmación del broker, y auth-service puede consumir el mensaje antes: ambas líneas salen en el mismo milisegundo en cualquier orden, por eso ese par admite 1 s de tolerancia |

## Cambio en un servicio encontrado al escribir la prueba

**Paso 3 sin evidencia en el log de auth-service.** Al consumir `empleado.creado`, auth-service solo escribía `evento publicado usuario.creado (id …)`: no nombraba el evento recibido ni el empleado, así que el paso 3 ("confirmar en los logs del auth-service que se detectó `empleado.creado` y se generó el usuario") no se podía comprobar. Se agregó una línea en `microservicios/auth-service/internal/aplicacion/servicio.go` (`CrearCuentaDesdeEmpleado`), con su prueba (`TestEmpleadoCreadoRegistraEnElLogLaCuentaCreada`):

```
auth-service: empleado.creado recibido (id 13c1f4c7-…): cuenta del empleado E001 creada en PENDIENTE_ACTIVACION
```

El `id` es el del envelope: el script lo busca en el log de empleados-service (`📤 Evento publicado: empleado.creado (id …)`) y comprueba que sea el mismo evento.

También se corrigió `microservicios/gestion-departamentos/test/test_departamentos.php`: llamaba al Gateway sin token y, desde la F5, recibía 401. Ahora inicia sesión como el administrador semilla (`TEST_ADMIN_EMAIL` / `TEST_ADMIN_PASSWORD`) y envía `Authorization: Bearer`; 22/22 verificaciones (`docker run --rm --network host -v "$PWD":/w -w /w php:8.2-cli-alpine php microservicios/gestion-departamentos/test/test_departamentos.php`).

## Resumen: cada paso del PDF

| Paso | Qué exige el PDF | Resultado |
|---|---|---|
| 1 | Servicios levantados | ✅ `GET /health` → 200, `sistema: OK`; 14 contenedores `(healthy)` |
| 2 | Onboarding | ✅ Login del admin semilla 200 (`role: ADMIN`, `sub: admin`); `POST /empleados` 201 ×2 |
| 3 | `empleado.creado` en auth-service y `usuario.creado` en notificaciones-service | ✅ Las dos líneas, enlazadas por el id del evento; línea `[NOTIFICACIÓN] Tipo: SEGURIDAD` con el token |
| 4 | `GET /empleados` sin cabeceras → 401 | ✅ 401 |
| 5 | Token del log → `reset-password` | ✅ 200 (el mismo token llegó por correo a Mailhog); reutilizarlo → 401 |
| 6 | Login USER | ✅ 200, JWT `{sub: <empleadoId>, role: USER, email}` |
| 7 | `GET /empleados` con JWT → 200 | ✅ 200 |
| 8 | Recuperación con token nuevo; repetir 5 y 6 | ✅ 202; `usuario.recuperacion` en los logs de auth y notificaciones; token distinto; reset 200; login 200; la contraseña anterior → 401 |
| 9 | `DELETE /empleados/E001` con USER → 403 | ✅ 403, el empleado sigue `ACTIVO` |
| 10 | `PUT /perfiles/E001` → 200; `PUT /perfiles/E002` → 403 | ✅ 200 / 403 |
| 11 | `change-password`; antigua falla, nueva funciona | ✅ 200; 401; 200 |
| 12 | `POST /vacaciones`; `vacaciones.programadas` y confirmación | ✅ 201; evento en el log (mismo id que consumió notificaciones); notificación `VACACIONES` |
| 13 | Scheduler: `vacaciones.iniciadas` → suspensión → `cuenta.desactivada` (VACACIONES) → login 401 | ✅ En ese orden; login 401 |
| 14 | `vacaciones.finalizadas` → `cuenta.activada` (FIN_VACACIONES) → login 200 | ✅ "Bienvenido de regreso"; login 200 |
| 15 | **Caso borde**: retiro durante las vacaciones; al terminar no se reactiva | ✅ Suspendida → retirada (`DESACTIVADA_PERMANENTE`) → `vacaciones.finalizadas` publicado e **ignorado** por auth-service; login 401; sin "bienvenida de regreso" |
| 16 | Offboarding → 200/204, `RETIRADO`, `empleado.retirado` | ✅ 200 `RETIRADO` con `fechaRetiro`; evento publicado y consumido por auth-service |
| 17 | Login del retirado → 401/403 | ✅ 401 |
| 18 | `GET /empleados?estado=RETIRADO` lista al empleado con `fechaRetiro` | ✅ |

Corridas: `demo.sh` 3 veces seguidas sobre la misma BD (la primera con E001 y E002 libres, así que usó esos ids literales; las otras, con sufijo), 65/65 verificaciones en las dos últimas (4 min 40 s y 5 min 16 s). La primera corrida detectó el falso negativo del orden del paso 13 descrito arriba. Colección Bruno: **41/41** tests (5 min 17 s).

## Evidencia del caso borde (paso 15)

Log de auth-service de la última corrida, filtrado por el empleado (`docker compose logs auth-service | grep E002-1792158141`):

```
2026/10/16 13:42:21 auth-service: empleado.creado recibido (id 7c28c4bd-6320-4cec-aa2e-7a1692983197): cuenta del empleado E002-1792158141 creada en PENDIENTE_ACTIVACION
2026/10/16 13:45:00 auth-service: cuenta del empleado E002-1792158141 suspendida por las vacaciones V-2026-0008
2026/10/16 13:45:01 auth-service: cuenta del empleado E002-1792158141 desactivada de forma permanente (retiro)
2026/10/16 13:47:00 auth-service: cuenta retirada: no se reactiva (empleado E002-1792158141, vacaciones V-2026-0008)
```

Lo que dice cada línea: la cuenta se crea pendiente (paso 2), se suspende al iniciar las vacaciones `V-2026-0008` (`SUSPENDIDA_TEMPORAL`), se desactiva para siempre al consumir `empleado.retirado`, y cuando el scheduler publica `vacaciones.finalizadas` la regla D5 lo ignora porque la cuenta ya no está suspendida sino retirada. El login sigue en 401 y notificaciones-service no envía la "bienvenida de regreso" (sale de `cuenta.activada`, que no se publicó).

## Salida de la última corrida de `demo.sh`

Sin editar, salvo los tokens, que el propio script recorta.

```

=== Paso 1 — Iniciar todos los servicios ===
   Este script no levanta el sistema. Antes de ejecutarlo: docker compose up --build -d
   [PASS] GET /health -> HTTP 200, sistema: OK
   Contenedores: 14 con estado (healthy)

=== Paso 2 — Onboarding de empleados (POST /empleados con el JWT del administrador semilla) ===
   [PASS] POST /auth/login del administrador semilla (admin@empresa.com) -> HTTP 200
   claims: {'sub': 'admin', 'role': 'ADMIN', 'email': 'admin@empresa.com'}
   [PASS] Departamento IT ya existe -> HTTP 200 (no se crea de nuevo)
   E001 ya existe (corrida anterior): esta corrida usa los ids E001-1792158141 y E002-1792158141
   [PASS] POST /empleados E001-1792158141 (juan.perez.1792158141@empresa.com) -> HTTP 201
   [PASS] POST /empleados E002-1792158141 (ana.munoz.1792158141@empresa.com) -> HTTP 201

=== Paso 3 — Verificar eventos en los logs (auth-service y notificaciones-service) ===
   [PASS] auth-service detectó empleado.creado y generó el usuario de E001-1792158141
      auth-service | 2026/10/16 13:42:21 auth-service: empleado.creado recibido (id 9eb5ca55-8e35-40b6-a73a-93958716aa82): cuenta del empleado E001-1792158141 creada en PENDIENTE_ACTIVACION
   [PASS] es el mismo empleado.creado que publicó empleados-service (id 9eb5ca55-8e35-40b6-a73a-93958716aa82)
   [PASS] notificaciones-service detectó usuario.creado de E001-1792158141 (eventoId 2739e4e6-dd93-40c5-92f3-97c3e360d642)
   [PASS] auth-service publicó ese usuario.creado
      auth-service | 2026/10/16 13:42:21 auth-service: evento publicado usuario.creado (id 2739e4e6-dd93-40c5-92f3-97c3e360d642)
   [PASS] se simuló el envío de las credenciales (línea [NOTIFICACIÓN] para juan.perez.1792158141@empresa.com)
      notificaciones-service | [NOTIFICACIÓN] Tipo: SEGURIDAD | Para: juan.perez.1792158141@empresa.com | Mensaje: "Para establecer o recuperar su contraseña use el token 28fdc80f-… (vence 2026-10-17T13:42:21Z). Bienvenido a TalentFlow."
   [PASS] auth-service detectó empleado.creado y generó el usuario de E002-1792158141
      auth-service | 2026/10/16 13:42:21 auth-service: empleado.creado recibido (id 7c28c4bd-6320-4cec-aa2e-7a1692983197): cuenta del empleado E002-1792158141 creada en PENDIENTE_ACTIVACION
   [PASS] es el mismo empleado.creado que publicó empleados-service (id 7c28c4bd-6320-4cec-aa2e-7a1692983197)
   [PASS] notificaciones-service detectó usuario.creado de E002-1792158141 (eventoId 51d99de4-fdb0-48b1-adf3-0d0d21a86769)
   [PASS] auth-service publicó ese usuario.creado
      auth-service | 2026/10/16 13:42:21 auth-service: evento publicado usuario.creado (id 51d99de4-fdb0-48b1-adf3-0d0d21a86769)
   [PASS] se simuló el envío de las credenciales (línea [NOTIFICACIÓN] para ana.munoz.1792158141@empresa.com)
      notificaciones-service | [NOTIFICACIÓN] Tipo: SEGURIDAD | Para: ana.munoz.1792158141@empresa.com | Mensaje: "Para establecer o recuperar su contraseña use el token 1372ae05-… (vence 2026-10-17T13:42:21Z). Bienvenido a TalentFlow."

=== Paso 4 — Petición denegada: GET /empleados sin cabeceras ===
   [PASS] GET /empleados sin token -> HTTP 401
   respuesta: {"error":"UNAUTHORIZED","message":"Token requerido para acceder a este recurso.","path":"/empleados"}

=== Paso 5 — Establecer contraseña con el token del log de la notificación (POST /auth/reset-password) ===
   token de juan.perez.1792158141@empresa.com leído del log de notificaciones-service: 28fdc80f-…
   (el mismo token llegó por correo a Mailhog)
   [PASS] POST /auth/reset-password de juan.perez.1792158141@empresa.com -> HTTP 200
   token de ana.munoz.1792158141@empresa.com leído del log de notificaciones-service: 1372ae05-…
   (el mismo token llegó por correo a Mailhog)
   [PASS] POST /auth/reset-password de ana.munoz.1792158141@empresa.com -> HTTP 200
   [PASS] el mismo token por segunda vez (un solo uso) -> HTTP 401

=== Paso 6 — Login USER con la contraseña recién establecida ===
   [PASS] POST /auth/login de E001-1792158141 -> HTTP 200
   claims: {'sub': 'E001-1792158141', 'role': 'USER', 'email': 'juan.perez.1792158141@empresa.com'}

=== Paso 7 — Lectura exitosa: GET /empleados con Authorization: Bearer <token USER> ===
   [PASS] GET /empleados con el JWT de E001-1792158141 -> HTTP 200
   empleados en la respuesta: 11

=== Paso 8 — Recuperación: POST /auth/recover-password, nuevo token, repetir los pasos 5 y 6 ===
   [PASS] POST /auth/recover-password (juan.perez.1792158141@empresa.com) -> HTTP 202
   [PASS] nuevo token en el log de notificaciones-service: ac3a2286-… (distinto del de activación)
   [PASS] notificaciones-service registró el evento usuario.recuperacion (eventoId 2b3bd6d3-61bb-45df-b9d2-83aa8178912c)
   [PASS] auth-service publicó ese usuario.recuperacion
   [PASS] repetición del paso 5: reset-password con el token nuevo -> HTTP 200
   [PASS] repetición del paso 6: login con la contraseña recuperada -> HTTP 200
   [PASS] la contraseña anterior ya no sirve -> HTTP 401

=== Paso 9 — Escritura denegada: DELETE /empleados/E001-1792158141 con el JWT del usuario regular ===
   [PASS] DELETE /empleados/E001-1792158141 con rol USER -> HTTP 403
   [PASS] el empleado sigue ACTIVO (estado: ACTIVO)

=== Paso 10 — Propiedad del recurso: PUT /perfiles con el JWT de E001-1792158141 ===
   [PASS] PUT /perfiles/E001-1792158141 (su propio perfil) -> HTTP 200
   perfil: ciudad=Armenia, telefono=3001234567
   [PASS] PUT /perfiles/E002-1792158141 (perfil ajeno) -> HTTP 403

=== Paso 11 — Cambio de contraseña: POST /auth/change-password con el JWT de E001-1792158141 ===
   [PASS] POST /auth/change-password -> HTTP 200
   [PASS] login con la contraseña antigua -> HTTP 401
   [PASS] login con la contraseña nueva -> HTTP 200

=== Paso 12 — Programar vacaciones (ADMIN): POST /vacaciones para E001-1792158141 ===
   Desviación: en lugar de fechaInicio = fechaFin = hoy (fin a las 23:59:59, D6) se usan horas reales de hoy:
   fechaInicio=2026-10-16T08:43:00 fechaFin=2026-10-16T08:44:00 (America/Bogota)
   [PASS] POST /vacaciones (E001-1792158141) -> HTTP 201
   vacacionesId=V-2026-0007 estado=PROGRAMADA
   [PASS] vacaciones-service publicó vacaciones.programadas (id 845c41b0-e60f-4679-83e9-7af6a9ba404a)
      vacaciones-service | 2026-10-16T13:42:24.494Z  INFO 1 --- [nio-8085-exec-5] c.t.v.i.m.RabbitPublicadorEventos        : 📤 Evento publicado: vacaciones.programadas (id 845c41b0-e60f-4679-83e9-7af6a9ba404a)
   [PASS] llegó la notificación de confirmación (VACACIONES): "Sus vacaciones del 2026-10-16T08:43:00-05:00 al 2026-10-16T08:44:00-05:00 (1 días hábiles) quedaron programadas."

=== Paso 13 — Desactivación automática al iniciar las vacaciones ===
   El scheduler corre cada minuto (VACACIONES_CRON); el inicio se detecta a las 2026-10-16T08:43:00 (espera de ~36 s)
   [PASS] GET /vacaciones/V-2026-0007 -> estado EN_CURSO
   [PASS] 1) vacaciones-service publicó vacaciones.iniciadas
      vacaciones-service | 2026-10-16T13:43:00.007Z  INFO 1 --- [   scheduling-1] c.t.v.i.m.RabbitPublicadorEventos        : 📤 Evento publicado: vacaciones.iniciadas (id 8a72e34f-324a-4357-a9c2-2fa8afbcca52)
   [PASS] 2) auth-service consumió el evento y suspendió la cuenta
      auth-service | 2026/10/16 13:43:00 auth-service: cuenta del empleado E001-1792158141 suspendida por las vacaciones V-2026-0007
   [PASS] 3) notificaciones-service recibió cuenta.desactivada (motivo VACACIONES, temporal)
      notificaciones-service | [NOTIFICACIÓN] Tipo: CUENTA | Para: juan.perez.1792158141@empresa.com | Mensaje: "Su cuenta fue desactivada temporalmente por vacaciones."
   [PASS] en ese orden: 2026-10-16T13:43:00.008228788Z <= 2026-10-16T13:43:00.007643924Z <= 2026-10-16T13:43:00.016031972Z (marcas de Docker)
   [PASS] POST /auth/login de E001-1792158141 durante las vacaciones -> HTTP 401

=== Paso 14 — Reactivación automática al llegar la fechaFin ===
   El fin se detecta a las 2026-10-16T08:44:00 (espera de ~58 s)
   [PASS] GET /vacaciones/V-2026-0007 -> estado FINALIZADA
   [PASS] vacaciones-service publicó vacaciones.finalizadas
      vacaciones-service | 2026-10-16T13:44:00.009Z  INFO 1 --- [   scheduling-1] c.t.v.i.m.RabbitPublicadorEventos        : 📤 Evento publicado: vacaciones.finalizadas (id 6dc90b66-c753-4fed-b239-09263ea7bb69)
   [PASS] auth-service reactivó la cuenta
      auth-service | 2026/10/16 13:44:00 auth-service: cuenta del empleado E001-1792158141 reactivada al terminar las vacaciones V-2026-0007
   [PASS] cuenta.activada con motivo FIN_VACACIONES ("Bienvenido de regreso")
      notificaciones-service | [NOTIFICACIÓN] Tipo: CUENTA | Para: juan.perez.1792158141@empresa.com | Mensaje: "Bienvenido de regreso. Su cuenta fue reactivada."
   [PASS] POST /auth/login de E001-1792158141 vuelve a funcionar -> HTTP 200

=== Paso 15 — Caso borde: retiro de E002-1792158141 durante sus vacaciones ===
   [PASS] antes: login de E002-1792158141 -> HTTP 200
   [PASS] POST /vacaciones E002-1792158141 (2026-10-16T08:45:00 a 2026-10-16T08:47:00) -> HTTP 201
   vacacionesId=V-2026-0008; inicio en ~58 s, fin en ~178 s
   [PASS] las vacaciones iniciaron y la cuenta de E002-1792158141 quedó suspendida
      auth-service | 2026/10/16 13:45:00 auth-service: cuenta del empleado E002-1792158141 suspendida por las vacaciones V-2026-0008
   [PASS] login de E002-1792158141 durante las vacaciones -> HTTP 401
   [PASS] DELETE /empleados/E002-1792158141 con el JWT de ADMIN (mientras está de vacaciones) -> HTTP 200
   [PASS] auth-service: cuenta de E002-1792158141 DESACTIVADA_PERMANENTE
      auth-service | 2026/10/16 13:45:01 auth-service: cuenta del empleado E002-1792158141 desactivada de forma permanente (retiro)
   Esperando la fechaFin (2026-10-16T08:47:00, ~119 s)...
   [PASS] vacaciones-service publicó vacaciones.finalizadas de V-2026-0008 (estado FINALIZADA)
      vacaciones-service | 2026-10-16T13:47:00.006Z  INFO 1 --- [   scheduling-1] c.t.v.i.m.RabbitPublicadorEventos        : 📤 Evento publicado: vacaciones.finalizadas (id 120b0ed1-10f7-43f5-bd01-104bf383bb26)
   [PASS] auth-service ignoró vacaciones.finalizadas sobre la cuenta retirada (D5)
      auth-service | 2026/10/16 13:47:00 auth-service: cuenta retirada: no se reactiva (empleado E002-1792158141, vacaciones V-2026-0008)
   [PASS] después de la fechaFin: login de E002-1792158141 sigue fallando -> HTTP 401
   [PASS] E002-1792158141 no recibió la "bienvenida de regreso" (notificaciones: 0)

=== Paso 16 — Offboarding (ADMIN): DELETE /empleados/E001-1792158141 ===
   [PASS] DELETE /empleados/E001-1792158141 con el JWT del administrador semilla -> HTTP 200
   [PASS] estado RETIRADO (estado: RETIRADO, fechaRetiro: 2026-10-16T13:47:01Z)
   [PASS] empleados-service publicó empleado.retirado
      empleados-service | 📤 Evento publicado: empleado.retirado (id 3281aefb-7b4f-4f77-b8a6-85ceb9c69a8c)
      auth-service | 2026/10/16 13:47:01 auth-service: cuenta del empleado E001-1792158141 desactivada de forma permanente (retiro)

=== Paso 17 — Verificar inhabilitación: login del empleado retirado ===
   [PASS] POST /auth/login de E001-1792158141 (retirado) -> HTTP 401

=== Paso 18 — Verificar auditoría: GET /empleados?estado=RETIRADO ===
   [PASS] GET /empleados?estado=RETIRADO -> HTTP 200
   [PASS] lista a E001-1792158141 con su fechaRetiro (2026-10-16T13:47:01Z)

=== Resumen ===
   PASS  1 — Iniciar todos los servicios
   PASS  2 — Onboarding de empleados (POST /empleados con el JWT del administrador semilla)
   PASS  3 — Verificar eventos en los logs (auth-service y notificaciones-service)
   PASS  4 — Petición denegada: GET /empleados sin cabeceras
   PASS  5 — Establecer contraseña con el token del log de la notificación (POST /auth/reset-password)
   PASS  6 — Login USER con la contraseña recién establecida
   PASS  7 — Lectura exitosa: GET /empleados con Authorization: Bearer <token USER>
   PASS  8 — Recuperación: POST /auth/recover-password, nuevo token, repetir los pasos 5 y 6
   PASS  9 — Escritura denegada: DELETE /empleados/E001-1792158141 con el JWT del usuario regular
   PASS  10 — Propiedad del recurso: PUT /perfiles con el JWT de E001-1792158141
   PASS  11 — Cambio de contraseña: POST /auth/change-password con el JWT de E001-1792158141
   PASS  12 — Programar vacaciones (ADMIN): POST /vacaciones para E001-1792158141
   PASS  13 — Desactivación automática al iniciar las vacaciones
   PASS  14 — Reactivación automática al llegar la fechaFin
   PASS  15 — Caso borde: retiro de E002-1792158141 durante sus vacaciones
   PASS  16 — Offboarding (ADMIN): DELETE /empleados/E001-1792158141
   PASS  17 — Verificar inhabilitación: login del empleado retirado
   PASS  18 — Verificar auditoría: GET /empleados?estado=RETIRADO
   Verificaciones: 65 correctas, 0 fallidas | duración: 4m40s
   RESULTADO: OK
```
