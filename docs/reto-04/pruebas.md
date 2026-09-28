# Reto 4 — Evidencia de pruebas

Resultados reales de la sección 6 ("Pruebas del Sistema") de `reto4.pdf`, obtenidos con [`demo.sh`](demo.sh) sobre el sistema levantado **desde cero** (`docker compose down -v && docker compose up --build -d`), en la rama `lyc4nthrope`, el 2026-09-28. La salida completa está al final, sin editar.

## Resumen: cada paso del PDF

| Paso | Qué exige el PDF | Resultado |
|---|---|---|
| 1 | Todos los servicios levantados | ✅ 12 servicios `(healthy)`; solo el Gateway (`8080`) y la UI del broker (`15672`) publicados |
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
| Pruebas automatizadas | 160: empleados 80 · gateway 6 · notificaciones 17 · perfiles 28 · vacaciones 29 |
| Colección Bruno | 21/21 tests, dos corridas seguidas |

## Salida completa de `demo.sh`

```

=== Paso 1 — Servicios levantados ===
SERVICE                   STATUS                    PORTS
api-gateway               Up 12 seconds (healthy)   0.0.0.0:8080->8080/tcp, [::]:8080->8080/tcp
database-departamentos    Up 34 seconds (healthy)   3306/tcp, 33060/tcp
database-empleados        Up 34 seconds (healthy)   5432/tcp
database-notificaciones   Up 34 seconds (healthy)   5432/tcp
database-perfiles         Up 33 seconds (healthy)   5432/tcp
database-vacaciones       Up 33 seconds (healthy)   5432/tcp
message-broker            Up 33 seconds (healthy)   4369/tcp, 5671-5672/tcp, 15671/tcp, 15691-15692/tcp, 25672/tcp, 0.0.0.0:15672->15672/tcp, [::]:15672->15672/tcp
departamentos-service     Up 18 seconds (healthy)   8082/tcp
empleados-service         Up 12 seconds (healthy)   8081/tcp
notificaciones-service    Up 22 seconds (healthy)   8084/tcp
perfiles-service          Up 22 seconds (healthy)   8083/tcp
vacaciones-service        Up 22 seconds (healthy)   8085/tcp

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
   {'status': 400, 'message': 'La fechaInicio (2026-06-15) no puede ser anterior a hoy (2026-09-28)'}
Mismas fechas, un año después:
   HTTP/1.1 201 Created
   location: /vacaciones/V-2026-0001
   {"id":"V-2026-0001","empleadoId":"E001","fechaInicio":"2027-06-15","fechaFin":"2027-06-30","estado":"PROGRAMADA","fechaCreacion":"2026-09-28T22:18:25Z"}
Notificación de tipo VACACIONES (GET /notificaciones/E001):
   BIENVENIDA | Bienvenido Juan Pérez a la empresa. Tu registro como empleado quedó completo.
   VACACIONES | Sus vacaciones del 2027-06-15 al 2027-06-30 (12 días hábiles) quedaron programadas.

=== Paso 8 — Validaciones de vacaciones (todas 400) ===
   Fechas incoherentes    -> 400 La fechaFin (2027-06-15) debe ser posterior a la fechaInicio (2027-06-30)
   Fechas en el pasado    -> 400 La fechaInicio (2026-06-15) no puede ser anterior a hoy (2026-09-28)
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
   {'id': 'E001', 'estado': 'RETIRADO', 'fechaRetiro': '2026-09-28T22:18:31Z', 'motivoRetiro': 'RENUNCIA'}
El empleado NO desaparece (GET /empleados/E001):
   {'id': 'E001', 'estado': 'RETIRADO', 'fechaRetiro': '2026-09-28T22:18:31Z'}
Auditoría (GET /empleados?estado=RETIRADO):
   E001 RETIRADO 2026-09-28T22:18:31Z
Auditoría por rango (desde=2026-09-28&hasta=2026-09-28):
   ['E001']
Notificación de desvinculación:
   BIENVENIDA
   VACACIONES
   DESVINCULACION
Perfil archivado:
   {'empleadoId': 'E001', 'archivado': True, 'fechaArchivado': '2026-09-28T22:18:31Z'}

=== Paso 11 — Reiniciar los contenedores y verificar que los datos persisten ===
Antes:
   empleado E001: RETIRADO | perfil archivado: True | notificaciones E001: 3 | vacaciones E001: 1
Después de docker compose down + up:
   empleado E001: RETIRADO | perfil archivado: True | notificaciones E001: 3 | vacaciones E001: 1

=== Estado del sistema (GET /health) ===
   sistema: OK | problemas: []
```
