# Reto 4 — Evidencia de pruebas

Resultados reales de la sección 6 ("Pruebas del Sistema") de `reto4.pdf`, obtenidos con [`demo.sh`](demo.sh) sobre el sistema levantado **desde cero** (`docker compose down -v && docker compose up --build -d`), en la rama `lyc4nthrope`, el 2026-10-05 (corrida repetida al agregar el bonus de correo real). La salida completa está al final, sin editar.

## Resumen: cada paso del PDF

| Paso | Qué exige el PDF | Resultado |
|---|---|---|
| 1 | Todos los servicios levantados | ✅ 12 servicios `(healthy)` + `mailhog` (sin healthcheck); solo el Gateway (`8080`), la UI del broker (`15672`) y la UI de Mailhog (`8025`) publicados |
| 2 | Broker activo y accesible (UI) | ✅ UI → HTTP 200; RabbitMQ 3.13.7; 3 colas con sus suscripciones |
| 3 | Crear un departamento | ✅ 201 |
| 4 | Crear un empleado | ✅ 201 (publica `empleado.creado`) |
| 5 | El evento fue procesado: perfil, notificación y log | ✅ Perfil por defecto creado solo; notificación BIENVENIDA; línea `[NOTIFICACIÓN]` en el log |
| 6 | Actualizar el perfil | ✅ 200, actualización parcial |
| 7 | Programar vacaciones + notificación VACACIONES | ✅ Con las fechas del PDF: 400 (junio de 2026 ya pasó). Mismas fechas en 2027: 201 + `Location`, y llegó la notificación VACACIONES (12 días hábiles) |
| 8 | Validaciones: todas 400 | ✅ Fechas incoherentes, fechas en el pasado, solapamiento (con `periodoEnConflicto`) y empleado inexistente |
| 9 | **Mismo `empleado.creado` publicado 2 veces → una notificación y un perfil** | ✅ **1 notificación y 1 perfil**; los 3 consumidores registran "evento duplicado descartado" |
| 10 | Retiro: baja lógica, auditoría, desvinculación, perfil archivado | ✅ RETIRADO con `fechaRetiro`, visible en `?estado=RETIRADO` y por rango de fechas; DESVINCULACION; perfil `archivado: true` |
| 11 | Reiniciar y verificar que los datos persisten | ✅ Mismos datos antes y después de `docker compose down` + `up` |
| Bonus | Correo real por un servicio SMTP (Mailhog) | ✅ 4 correos en Mailhog: bienvenida, vacaciones y desvinculación de E001, y **uno solo** para E900 (su evento se publicó 2 veces). Detalle [abajo](#bonus-correo-real-por-smtp-mailhog) |

## Verificaciones adicionales (por fase, en Docker con caídas reales)

| Qué | Resultado |
|---|---|
| Contrato de los eventos | Los 4 tipos de mensaje, leídos del broker, coinciden **campo por campo** con el Catálogo (envelope + `data`), persistentes y con `message_id` = `id` |
| Broker caído al publicar | La operación responde 201/200 y queda guardada; el error queda en el log; al volver el broker, los servicios reconectan solos |
| Consumidor con su BD caída | El mensaje espera en la cola y se procesa cuando la BD vuelve (no se pierde) |
| Mensajes envenenados (JSON corrupto, `id` de 150 caracteres, email de 200) | Se descartan con log; el consumidor sigue funcionando (sin bucle de reintentos) |
| 10 solicitudes simultáneas de vacaciones (mismo empleado y rango) | 1 creada, 9 rechazadas con 400 |
| Empleado retirado programa vacaciones | 400 (la réplica se actualizó con `empleado.retirado`) |
| `empleado.actualizado` sin perfil previo | El perfil se crea: el sistema se recupera de un `empleado.creado` perdido |
| Cancelar vacaciones | CANCELADA (no se borra); ya iniciado o ya cancelado → 409; el rango queda libre |
| `/health` agregado | `sistema: OK`; con un servicio, una BD o el broker caídos, `problemas` explica la causa exacta |
| Swagger | Los 5 servicios, por el Gateway (`/<servicio>/docs`) |
| Correo con el servidor SMTP caído (bonus) | La notificación queda registrada y en el log `[NOTIFICACIÓN]`; el error de SMTP queda en el log; el consumidor sigue con el siguiente mensaje (sin reintento) |
| Pruebas automatizadas | 165: empleados 80 · gateway 6 · notificaciones 22 · perfiles 28 · vacaciones 29 |
| Colección Bruno | 21/21 tests, dos corridas seguidas |

## Bonus: correo real por SMTP (Mailhog)

Obtenido el 2026-10-05 sobre el mismo sistema, después de `demo.sh` (cuyo paso 11 reinicia los contenedores y deja vacía la bandeja de Mailhog, que guarda en memoria).

**Un correo tal como lo entrega la API de Mailhog** (`GET http://localhost:8025/api/v2/messages`, tras crear a la empleada E002 "Ana Muñoz"). Las tildes viajan codificadas en *quoted-printable*:

```
total: 1
Raw.From: notificaciones@talentflow.local | Raw.To: ['ana.munoz@empresa.com']
   Content-Transfer-Encoding : ['quoted-printable']
   Content-Type : ['text/plain; charset=UTF-8']
   Date : ['Mon, 05 Oct 2026 12:43:39 +0000']
   From : ['notificaciones@talentflow.local']
   MIME-Version : ['1.0']
   Subject : ['Bienvenida a la empresa']
   To : ['ana.munoz@empresa.com']
Body: 'Bienvenido Ana Mu=C3=B1oz a la empresa. Tu registro como empleado qued=C3=\r\n=B3 completo.'
```

(Se omiten `Message-ID`, `Received` y `Return-Path`, que agrega Mailhog.) Log del servicio para ese envío:

```
[NOTIFICACIÓN] Tipo: BIENVENIDA | Para: ana.munoz@empresa.com | Mensaje: "Bienvenido Ana Muñoz a la empresa. Tu registro como empleado quedó completo."
{"time":"2026-10-05T12:43:39.770076696Z","level":"INFO","msg":"correo enviado","service":"notificaciones-service","notificacionId":"adff620c-6296-4f30-aaa4-1c51b59ccf02","destinatario":"ana.munoz@empresa.com","servidor":"mailhog:1025"}
```

**Servidor de correo caído** (`docker compose stop mailhog`, luego dos `POST /empleados`: E003 y E004). Ambas respondieron 201 y ambas notificaciones quedaron registradas:

```
GET /notificaciones/E003
[{"id":"238a00ad-f700-4aba-8382-27a6f019ecba","tipo":"BIENVENIDA","destinatario":"luis.pena@empresa.com","mensaje":"Bienvenido Luis Peña a la empresa. Tu registro como empleado quedó completo.","fechaEnvio":"2026-10-05T12:43:42.097313Z","empleadoId":"E003"}]
GET /notificaciones/E004
[{"id":"6b6731f8-15c6-413e-87a7-18fd7e3e744d","tipo":"BIENVENIDA","destinatario":"sofia.nino@empresa.com","mensaje":"Bienvenido Sofía Niño a la empresa. Tu registro como empleado quedó completo.","fechaEnvio":"2026-10-05T12:43:42.149763Z","empleadoId":"E004"}]
```

Log de notificaciones-service: el error de SMTP queda registrado y el consumidor continúa con el mensaje siguiente:

```
[NOTIFICACIÓN] Tipo: BIENVENIDA | Para: luis.pena@empresa.com | Mensaje: "Bienvenido Luis Peña a la empresa. Tu registro como empleado quedó completo."
{"time":"2026-10-05T12:43:42.149583136Z","level":"ERROR","msg":"no se pudo enviar el correo; la notificación queda registrada","service":"notificaciones-service","notificacionId":"238a00ad-f700-4aba-8382-27a6f019ecba","destinatario":"luis.pena@empresa.com","error":"conectar con el servidor SMTP: dial tcp: lookup mailhog on 127.0.0.11:53: no such host"}
{"time":"2026-10-05T12:43:42.149611229Z","level":"INFO","msg":"notificación registrada","service":"notificaciones-service","eventoId":"765bad1b-2562-4bbb-8e15-acee3ab9723f","tipo":"empleado.creado","notificacionId":"238a00ad-f700-4aba-8382-27a6f019ecba","empleadoId":"E003"}
[NOTIFICACIÓN] Tipo: BIENVENIDA | Para: sofia.nino@empresa.com | Mensaje: "Bienvenido Sofía Niño a la empresa. Tu registro como empleado quedó completo."
{"time":"2026-10-05T12:43:42.154875885Z","level":"ERROR","msg":"no se pudo enviar el correo; la notificación queda registrada","service":"notificaciones-service","notificacionId":"6b6731f8-15c6-413e-87a7-18fd7e3e744d","destinatario":"sofia.nino@empresa.com","error":"conectar con el servidor SMTP: dial tcp: lookup mailhog on 127.0.0.11:53: no such host"}
{"time":"2026-10-05T12:43:42.154902208Z","level":"INFO","msg":"notificación registrada","service":"notificaciones-service","eventoId":"5d6b3958-046e-4037-9bea-4f40984af746","tipo":"empleado.creado","notificacionId":"6b6731f8-15c6-413e-87a7-18fd7e3e744d","empleadoId":"E004"}
```

**Al volver Mailhog** (`docker compose start mailhog`, luego `DELETE /empleados/E002`) el correo siguiente se envía con normalidad; los dos fallidos no se reenvían:

```
total: 1
   ana.munoz@empresa.com | =?utf-8?q?Desvinculaci=C3=B3n_de_la_empresa?=
{"time":"2026-10-05T12:43:56.493437137Z","level":"INFO","msg":"correo enviado","service":"notificaciones-service","notificacionId":"581ed1ab-381a-4bac-8ae1-0f3c93ceadea","destinatario":"ana.munoz@empresa.com","servidor":"mailhog:1025"}
sistema: OK []
```

## Salida completa de `demo.sh`

```

=== Paso 1 — Servicios levantados ===
SERVICE                   STATUS                    PORTS
api-gateway               Up 11 seconds (healthy)   0.0.0.0:8080->8080/tcp, [::]:8080->8080/tcp
database-departamentos    Up 38 seconds (healthy)   3306/tcp, 33060/tcp
database-empleados        Up 38 seconds (healthy)   5432/tcp
database-notificaciones   Up 38 seconds (healthy)   5432/tcp
database-perfiles         Up 38 seconds (healthy)   5432/tcp
database-vacaciones       Up 37 seconds (healthy)   5432/tcp
mailhog                   Up 39 seconds             1025/tcp, 0.0.0.0:8025->8025/tcp, [::]:8025->8025/tcp
message-broker            Up 38 seconds (healthy)   4369/tcp, 5671-5672/tcp, 15671/tcp, 15691-15692/tcp, 25672/tcp, 0.0.0.0:15672->15672/tcp, [::]:15672->15672/tcp
departamentos-service     Up 17 seconds (healthy)   8082/tcp
empleados-service         Up 12 seconds (healthy)   8081/tcp
notificaciones-service    Up 16 seconds (healthy)   8084/tcp
perfiles-service          Up 16 seconds (healthy)   8083/tcp
vacaciones-service        Up 16 seconds (healthy)   8085/tcp

=== Paso 2 — Broker activo y accesible (UI de administración) ===
GET http://localhost:15672 (UI) -> HTTP 200
RabbitMQ 3.13.7 | nodo rabbit@message-broker
Colas y a qué eventos están suscritas:
   notificaciones.eventos <- empleado.creado, empleado.retirado, vacaciones.programadas
   perfiles.eventos <- empleado.actualizado, empleado.creado, empleado.retirado
   vacaciones.eventos <- empleado.actualizado, empleado.creado, empleado.retirado

=== Paso 3 — Crear un departamento ===
POST /departamentos -> HTTP 201

=== Paso 4 — Crear un empleado (publica empleado.creado) ===
POST /empleados -> HTTP 201

=== Paso 5 — El evento fue procesado por los consumidores (fan-out) ===
Perfil creado automáticamente (GET /perfiles/E001):
   {'empleadoId': 'E001', 'nombre': 'Juan Pérez', 'email': 'juan.perez@empresa.com', 'telefono': '', 'ciudad': '', 'biografia': '', 'archivado': False}
Notificación registrada (GET /notificaciones/E001):
   BIENVENIDA -> juan.perez@empresa.com
Notificación simulada en el log de notificaciones-service:
   [NOTIFICACIÓN] Tipo: BIENVENIDA | Para: juan.perez@empresa.com | Mensaje: "Bienvenido Juan Pérez a la empresa. Tu registro como empleado quedó completo."

=== Paso 6 — Actualizar el perfil del empleado ===
   {'empleadoId': 'E001', 'nombre': 'Juan Pérez', 'telefono': '3001234567', 'ciudad': 'Armenia', 'biografia': 'Ingeniero de sistemas'}

=== Paso 7 — Programar vacaciones y verificar la confirmación asincrónica ===
Cuerpo LITERAL del PDF (junio de 2026 ya pasó -> validación 'fechas en el pasado'):
   {'status': 400, 'message': 'La fechaInicio (2026-06-15) no puede ser anterior a hoy (2026-10-05)'}
Mismas fechas, un año después:
   HTTP/1.1 201 Created
   location: /vacaciones/V-2026-0001
   {"id":"V-2026-0001","empleadoId":"E001","fechaInicio":"2027-06-15","fechaFin":"2027-06-30","estado":"PROGRAMADA","fechaCreacion":"2026-10-05T12:42:39Z"}
Notificación de tipo VACACIONES (GET /notificaciones/E001):
   BIENVENIDA | Bienvenido Juan Pérez a la empresa. Tu registro como empleado quedó completo.
   VACACIONES | Sus vacaciones del 2027-06-15 al 2027-06-30 (12 días hábiles) quedaron programadas.

=== Paso 8 — Validaciones de vacaciones (todas 400) ===
   Fechas incoherentes    -> 400 La fechaFin (2027-06-15) debe ser posterior a la fechaInicio (2027-06-30)
   Fechas en el pasado    -> 400 La fechaInicio (2026-06-15) no puede ser anterior a hoy (2026-10-05)
   Solapamiento           -> 400 El período se cruza con otro período PROGRAMADA del empleado (V-2026-0001: 2027-06-15 a 2027-06-30)
      periodoEnConflicto: V-2026-0001 (2027-06-15 a 2027-06-30, PROGRAMADA)
   Empleado inexistente   -> 400 El empleado NO-EXISTE no corresponde a un empleado registrado

=== Paso 9 — Deduplicación: el mismo empleado.creado publicado DOS veces (mismo id) ===
   publicación 1 -> {'routed': True}
   publicación 2 -> {'routed': True}
   notificaciones de E900: 1
   perfiles de E900:       1
   logs (cada consumidor lo procesó una vez y descartó el duplicado):
      notificaciones-service: evento duplicado descartado
      perfiles-service: evento duplicado descartado
      vacaciones-service: evento duplicado descartado

=== Paso 10 — Retirar un empleado (baja lógica) ===
   {'id': 'E001', 'estado': 'RETIRADO', 'fechaRetiro': '2026-10-05T12:42:45Z', 'motivoRetiro': 'RENUNCIA'}
El empleado NO desaparece (GET /empleados/E001):
   {'id': 'E001', 'estado': 'RETIRADO', 'fechaRetiro': '2026-10-05T12:42:45Z'}
Auditoría (GET /empleados?estado=RETIRADO):
   E001 RETIRADO 2026-10-05T12:42:45Z
Auditoría por rango (desde=2026-10-05&hasta=2026-10-05):
   ['E001']
Notificación de desvinculación:
   BIENVENIDA
   VACACIONES
   DESVINCULACION
Perfil archivado:
   {'empleadoId': 'E001', 'archivado': True, 'fechaArchivado': '2026-10-05T12:42:45Z'}

=== Bonus — Correos reales recibidos por Mailhog (SMTP) ===
   correos recibidos: 4
   juan.perez@empresa.com | Bienvenida a la empresa | Bienvenido Juan Pérez a la empresa. Tu registro como empleado quedó completo.
   juan.perez@empresa.com | Vacaciones programadas | Sus vacaciones del 2027-06-15 al 2027-06-30 (12 días hábiles) quedaron programadas.
   laura.gil@empresa.com | Bienvenida a la empresa | Bienvenido Laura Gil a la empresa. Tu registro como empleado quedó completo.
   juan.perez@empresa.com | Desvinculación de la empresa | Su cuenta ha sido desactivada por desvinculación de la empresa. Gracias por su trabajo.
   correos para laura.gil@empresa.com (su evento se publicó 2 veces): 1

=== Paso 11 — Reiniciar los contenedores y verificar que los datos persisten ===
Antes:
   empleado E001: RETIRADO | perfil archivado: True | notificaciones E001: 3 | vacaciones E001: 1
Después de docker compose down + up:
   empleado E001: RETIRADO | perfil archivado: True | notificaciones E001: 3 | vacaciones E001: 1

=== Estado del sistema (GET /health) ===
   sistema: OK | problemas: []
```
