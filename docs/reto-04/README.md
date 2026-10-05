# Reto 4 — Manual paso a paso

Cómo levantar el sistema, dónde ver el estado de cada componente y cómo demostrar, en orden, todo lo que pide `reto4.pdf`. URL base de la API: **`http://localhost:8080`** · UI del broker: **`http://localhost:15672`** (`admin` / `admin` por defecto) · correos enviados (bonus): **`http://localhost:8025`**.

| Archivo | Para qué |
|---|---|
| **Este README** | Manual: requisitos del reto, estados, pruebas paso a paso, deduplicación desde la UI, bonus de correo real (Mailhog), respuestas para la sustentación |
| [`demo.sh`](demo.sh) | Ejecuta automáticamente los 11 pasos de la sección 6 del PDF y lista los correos del bonus |
| [`pruebas.md`](pruebas.md) | Evidencia: salida real de esos pasos (incluida la deduplicación) |
| [`bruno/`](bruno/) | Colección Bruno del Reto 4 (21 peticiones con tests) |
| [`PLAN.md`](PLAN.md) | Decisiones de diseño (D1-D13) y aprendizajes de cada fase |
| [README raíz](../../README.md#comunicación-asincrónica-reto-4) | Documento de entrega: justificaciones completas |

## 0. Criterios de evaluación → dónde se demuestra

| # | Criterio (valor) | Dónde |
|---|---|---|
| 1 | Message Broker (0.5) | [README raíz: por qué RabbitMQ](../../README.md#message-broker-por-qué-rabbitmq) · §2 y §3 de este manual |
| 2 | Publicación de eventos y baja lógica (1.0) | §4 pasos 4, 10 · [eventos](../eventos.md) · [baja lógica y auditoría](../../README.md#baja-lógica-y-auditoría-empleados-service) |
| 3 | Notificaciones (1.0) | §4 pasos 5, 7, 10 · §5 deduplicación · §6 bonus: correo real por SMTP · `/notificaciones/docs` |
| 4 | Perfiles (1.0) | §4 pasos 5, 6, 10 · `/perfiles/docs` |
| 5 | Vacaciones (1.0) | §4 pasos 7, 8 · [opción (b)](../../README.md#validación-del-empleado-en-vacaciones-réplica-local-por-eventos-opción-b) · `/vacaciones/docs` |
| 6 | Pruebas y documentación (0.5) | Todo este manual · [tabla servicio ↔ lenguaje](../../README.md#servicios-y-lenguajes) · §7 Swagger |

## 1. Prerrequisitos

- Docker con Docker Compose v2 y ~6 GB libres en disco (el primer build compila Java, Go, Python y Node).
- Puertos **8080**, **15672** y **8025** libres en el host.
- Volúmenes limpios si vienes de una versión anterior (los esquemas cambiaron en el Reto 4):

```bash
docker compose down -v
```

## 2. Levantar el sistema

```bash
docker compose up --build -d
docker compose ps       # esperar ~40 s a que los 12 servicios estén (healthy)
```

`broker-init` aparece como `Exited (0)`: es correcto, importa la topología de eventos y termina. `mailhog` aparece `Up` sin `(healthy)`: no tiene healthcheck y ningún servicio espera por él.

## 3. ¿Dónde veo el estado de cada cosa?

| Qué | Cómo |
|---|---|
| **Todo el sistema** | `curl http://localhost:8080/health` → `sistema: OK/DEGRADADO`, `problemas` (por qué), y por servicio: `status`, `db`, `broker` y (empleados) `circuitoDepartamentos` |
| El broker, sus colas y mensajes | UI `http://localhost:15672` → pestañas **Queues** (mensajes por cola) y **Exchanges** → `talentflow.eventos` (bindings) |
| Las notificaciones "enviadas" | `docker compose logs -f notificaciones-service \| grep NOTIFICACIÓN` |
| Los correos reales (bonus) | UI de Mailhog `http://localhost:8025` o `curl http://localhost:8025/api/v2/messages` ([§6](#6-bonus-correo-real-por-smtp-mailhog)) |
| Qué evento procesó (o descartó) cada consumidor | `docker compose logs -f perfiles-service vacaciones-service notificaciones-service` |
| Qué evento publicó cada productor | `docker compose logs empleados-service vacaciones-service \| grep "Evento publicado"` |

Qué reporta `GET /health` ante cada fallo (verificado con caídas reales):

| Situación | `problemas` |
|---|---|
| Todo sano | `[]` y `sistema: OK` |
| Un servicio detenido | `"perfiles-service: No responde"` |
| La BD de un servicio detenida | `"vacaciones-service: su base de datos no responde"` |
| Broker detenido | `"<servicio>: sin conexión al broker de mensajería"` para los 4 servicios que lo usan |
| Todo restaurado | vuelve a `[]` solo (los servicios reconectan en segundo plano) |

## 4. Pruebas del sistema, paso a paso (sección 6 del PDF)

Automático: `bash docs/reto-04/demo.sh` (≈1 min; requiere volúmenes limpios). Manual:

**Paso 1-2 — Servicios y broker.** `docker compose ps` (12 healthy) y abrir `http://localhost:15672`.

**Paso 3 — Departamento**

```bash
curl -X POST http://localhost:8080/departamentos -H "Content-Type: application/json" \
  -d '{"id": "IT", "nombre": "Tecnología", "descripcion": "Departamento de TI"}'
```

**Paso 4 — Empleado** (publica `empleado.creado`)

```bash
curl -X POST http://localhost:8080/empleados -H "Content-Type: application/json" \
  -d '{"id": "E001", "nombre": "Juan", "apellido": "Pérez",
       "email": "juan.perez@empresa.com", "numeroEmpleado": "EMP-2026-001",
       "cargo": "Desarrollador Senior", "area": "Tecnología",
       "departamentoId": "IT", "fechaIngreso": "2026-03-01"}'
```

**Paso 5 — Fan-out: dos servicios reaccionaron solos**

```bash
curl http://localhost:8080/perfiles/E001          # perfil por defecto (nombre "Juan Pérez", resto vacío)
curl http://localhost:8080/notificaciones/E001    # BIENVENIDA
docker compose logs notificaciones-service | grep NOTIFICACIÓN
```

**Paso 6 — Actualizar el perfil** (actualización parcial: lo no enviado no cambia)

```bash
curl -X PUT http://localhost:8080/perfiles/E001 -H "Content-Type: application/json" \
  -d '{"telefono": "3001234567", "ciudad": "Armenia", "biografia": "Ingeniero de sistemas"}'
```

**Paso 7 — Vacaciones y confirmación asincrónica**

> ⚠️ Las fechas del PDF (junio de **2026**) ya pasaron: la validación "fechas en el pasado" las rechaza con `400`. Usar fechas futuras, p. ej. las mismas un año después.

```bash
curl -i -X POST http://localhost:8080/vacaciones -H "Content-Type: application/json" \
  -d '{"empleadoId": "E001", "fechaInicio": "2027-06-15", "fechaFin": "2027-06-30"}'   # 201 + Location
curl http://localhost:8080/notificaciones/E001    # ahora también VACACIONES
```

**Paso 8 — Validaciones** (todas `400`)

```bash
# Fechas incoherentes
curl -X POST http://localhost:8080/vacaciones -H "Content-Type: application/json" \
  -d '{"empleadoId": "E001", "fechaInicio": "2027-06-30", "fechaFin": "2027-06-15"}'
# Fechas en el pasado
curl -X POST http://localhost:8080/vacaciones -H "Content-Type: application/json" \
  -d '{"empleadoId": "E001", "fechaInicio": "2026-06-15", "fechaFin": "2026-06-30"}'
# Solapamiento (la respuesta incluye periodoEnConflicto)
curl -X POST http://localhost:8080/vacaciones -H "Content-Type: application/json" \
  -d '{"empleadoId": "E001", "fechaInicio": "2027-06-20", "fechaFin": "2027-07-05"}'
# Empleado inexistente
curl -X POST http://localhost:8080/vacaciones -H "Content-Type: application/json" \
  -d '{"empleadoId": "NO-EXISTE", "fechaInicio": "2027-08-01", "fechaFin": "2027-08-15"}'
```

**Paso 9 — Deduplicación**: ver [§5](#5-deduplicación-desde-la-ui-de-rabbitmq).

**Paso 10 — Retiro (baja lógica)**

```bash
curl -X DELETE http://localhost:8080/empleados/E001       # 200, estado RETIRADO (cuerpo opcional {"motivo": "DESPIDO"})
curl http://localhost:8080/empleados/E001                 # sigue existiendo: RETIRADO + fechaRetiro
curl "http://localhost:8080/empleados?estado=RETIRADO"
curl "http://localhost:8080/empleados?estado=RETIRADO&desde=2026-01-01&hasta=2026-12-31"
curl http://localhost:8080/notificaciones/E001            # + DESVINCULACION
curl http://localhost:8080/perfiles/E001                  # archivado: true
```

**Paso 11 — Persistencia**

```bash
docker compose down        # SIN -v
docker compose up -d
curl http://localhost:8080/empleados/E001                 # sigue RETIRADO; perfiles, notificaciones y vacaciones intactos
```

## 5. Deduplicación desde la UI de RabbitMQ

La prueba que exige el reto: publicar **el mismo mensaje dos veces** (mismo `id` de envelope) y comprobar que solo hay **una** notificación y **un** perfil.

1. Abrir `http://localhost:15672` e iniciar sesión (`admin` / `admin`).
2. Pestaña **Exchanges** → clic en **`talentflow.eventos`**.
3. Desplegar **Publish message** y llenar:
   - **Routing key**: `empleado.creado`
   - **Delivery mode**: `2 - Persistent`
   - **Properties**: `content_type` = `application/json`
   - **Payload**:
     ```json
     {"id":"3f9b2c10-8e4a-4d6b-9f21-7c0a5d8e1234","type":"empleado.creado","version":1,"occurredAt":"2026-03-01T14:32:05Z","producer":"empleados-service","data":{"empleadoId":"E900","nombre":"Laura","apellido":"Gil","email":"laura.gil@empresa.com","numeroEmpleado":"EMP-2026-900","cargo":"QA","area":"Tecnología","departamentoId":"IT","fechaIngreso":"2026-03-01","estado":"ACTIVO"}}
     ```
4. Clic en **Publish message** → aparece *"Message published"*. **Volver a hacer clic** (mismo mensaje, mismo `id`).
5. Verificar:

```bash
curl http://localhost:8080/notificaciones/E900   # 1 sola notificación BIENVENIDA
curl http://localhost:8080/perfiles              # 1 solo perfil de E900
docker compose logs notificaciones-service perfiles-service vacaciones-service | grep 3f9b2c10
# cada consumidor: "evento aplicado"/"notificación registrada" una vez y "evento duplicado descartado" la segunda
```

Para repetir la prueba, cambiar el `id` del envelope (y el `empleadoId`), o no habrá ni el primer efecto: ese `id` ya quedó procesado.

## 6. Bonus: correo real por SMTP (Mailhog)

Además del log `[NOTIFICACIÓN]` (que sigue saliendo siempre), cada notificación se envía como correo real por SMTP a **Mailhog**, un servidor de correo de pruebas: recibe los correos y los muestra, sin entregarlos a ningún buzón real.

1. Abrir **`http://localhost:8025`** (sin usuario ni contraseña). La bandeja empieza vacía.
2. Ejecutar las acciones de la §4 y ver llegar un correo por cada una:

   | Acción | Correo (asunto) | Para |
   |---|---|---|
   | Paso 4 — `POST /empleados` | Bienvenida a la empresa | email del empleado |
   | Paso 7 — `POST /vacaciones` | Vacaciones programadas | email del empleado |
   | Paso 10 — `DELETE /empleados/{id}` | Desvinculación de la empresa | email del empleado |

   Remitente: `notificaciones@talentflow.local`. El cuerpo es el mismo `mensaje` de `GET /notificaciones`.
3. Verificar sin navegador, con la API de Mailhog:

   ```bash
   curl -s http://localhost:8025/api/v2/messages          # JSON completo: total, items[].Content.Headers, items[].Content.Body
   curl -s http://localhost:8025/api/v2/messages | python3 -c "
   import json,sys,quopri
   from email.header import decode_header, make_header
   d=json.load(sys.stdin); print('total:', d['total'])
   for c in d['items']:
       h=c['Content']['Headers']
       print(h['To'][0], '|', make_header(decode_header(h['Subject'][0])), '|', quopri.decodestring(c['Content']['Body']).decode().strip())"
   ```

   La API devuelve el correo tal como viajó: las tildes van codificadas (`Subject: =?utf-8?q?Desvinculaci=C3=B3n_de_la_empresa?=`, cuerpo en *quoted-printable*: `qued=C3=B3`). El segundo comando las decodifica.
4. **Un evento duplicado no envía un segundo correo.** Hacer la prueba de la [§5](#5-deduplicación-desde-la-ui-de-rabbitmq) (publicar dos veces el mismo mensaje) y comprobar que para `laura.gil@empresa.com` hay **un solo** correo: el correo se envía únicamente cuando la notificación se registra por primera vez.
5. **Si el servidor de correo falla, la notificación no se pierde:**

   ```bash
   docker compose stop mailhog
   curl -X POST http://localhost:8080/empleados -H "Content-Type: application/json" \
     -d '{"id": "E003", "nombre": "Luis", "apellido": "Peña", "email": "luis.pena@empresa.com",
          "numeroEmpleado": "EMP-2026-003", "cargo": "Analista", "area": "Tecnología",
          "departamentoId": "IT", "fechaIngreso": "2026-03-01"}'
   curl http://localhost:8080/notificaciones/E003               # la BIENVENIDA quedó registrada
   docker compose logs --since 1m notificaciones-service        # línea [NOTIFICACIÓN] + error "no se pudo enviar el correo"
   docker compose start mailhog
   ```

   El servicio registra el error y sigue consumiendo (cada envío tiene un plazo máximo de 5 s). El correo fallido **no se reintenta**: reencolar el mensaje podría duplicar correos, y la notificación ya está en el historial.

`demo.sh` lista los correos recibidos justo después del paso 10. Mailhog los guarda **en memoria**: se borran al reiniciar su contenedor (por eso la bandeja queda vacía tras el paso 11).

## 7. Swagger de todos los servicios

| Servicio | Swagger UI |
|---|---|
| empleados | `http://localhost:8080/empleados/docs` |
| departamentos | `http://localhost:8080/departamentos/docs` |
| perfiles | `http://localhost:8080/perfiles/docs` |
| notificaciones | `http://localhost:8080/notificaciones/docs` |
| vacaciones | `http://localhost:8080/vacaciones/docs` |

## 8. Colección Bruno y pruebas automatizadas

```bash
cd docs/reto-04/bruno && npx @usebruno/cli run --env Local   # 21 peticiones, 21 tests (repetible)
```

Pruebas automatizadas de cada servicio (165, sin Docker): ver [Comandos comunes](../../README.md#comandos-comunes).

## 9. Preguntas probables en la sustentación

| Pregunta | Respuesta corta | Dónde mostrarlo |
|---|---|---|
| ¿Por qué RabbitMQ y no Kafka? | Fan-out nativo con exchange topic, su UI permite publicar a mano (lo exige la prueba de deduplicación), un solo contenedor. Kafka es para streaming masivo con retención, que aquí no se necesita | README raíz |
| ¿Qué es el fan-out? | Un evento (`empleado.creado`) llega a varias colas y cada servicio reacciona por su cuenta, sin que empleados sepa quién escucha | UI → Exchanges → bindings; §4 paso 5 |
| ¿Qué pasa si publico dos veces el mismo mensaje? | El segundo se descarta: cada consumidor guarda el `id` procesado en la misma transacción que el efecto | §5 |
| ¿Qué pasa si un consumidor se cae? | Los mensajes esperan en su cola (durable, persistentes) y se procesan cuando vuelve; el `ack` solo se envía tras el commit | UI → Queues con el servicio detenido |
| ¿Y si se cae el broker al publicar? | La operación se guarda y el error queda en el log (no se revierte, como pide el reto); el evento se pierde. Solución formal: patrón Outbox | README raíz, "Limitaciones conocidas" |
| ¿El correo es real o simulado? | Ambos: el log `[NOTIFICACIÓN]` que pide el reto sale siempre y, además, se envía un correo real por SMTP a Mailhog (bonus). Es otra implementación del mismo puerto `Canal`; el caso de uso no cambió | §6; `http://localhost:8025` |
| ¿Y si el servidor de correo está caído? | La notificación ya quedó guardada y en el log; el fallo del correo se registra y no se reintenta (reencolar duplicaría correos). El envío tiene un plazo de 5 s para no frenar al consumidor | §6 paso 5 |
| ¿Por qué la opción (b) en vacaciones? | Disponibilidad: vacaciones funciona aunque empleados esté caído; además la réplica tiene el email que exige el evento | README raíz |
| ¿Por qué 5 lenguajes? | El reto pide los 3 nuevos distintos entre sí y de los existentes (Node, PHP): Python, Go, Java | README raíz, tabla de lenguajes |
| ¿Por qué el DELETE de empleados no borra? | Regla de dominio del Reto 00: retirar = marcar RETIRADO; se conserva para auditoría | §4 paso 10 |
| ¿Por qué la bienvenida sale con `empleado.creado` si el catálogo dice `usuario.creado`? | `usuario.creado` lo produce el auth-service del Reto 5, que aún no existe; el Reto 4 pide hacerlo con `empleado.creado` | README raíz, eventos |
| ¿Qué pasa si dos solicitudes de vacaciones llegan a la vez? | Solo una se crea: bloqueo `FOR UPDATE` + restricción `EXCLUDE` en PostgreSQL | README raíz, validaciones |
| ¿Por qué las fechas del PDF dan 400? | Junio de 2026 ya pasó: es la validación "fechas en el pasado" | §4 paso 7 |

## 10. Problemas comunes

| Síntoma | Causa y solución |
|---|---|
| Un servicio no arranca y su BD está en `Restarting` | Volumen de una versión anterior o inicialización interrumpida (p. ej. disco lleno): `docker compose down -v`, liberar espacio (`docker image prune -f`) y volver a levantar |
| `POST /vacaciones` responde 400 "no puede ser anterior a hoy" | Las fechas ya pasaron (el PDF usa 2026-06): usar fechas futuras |
| `POST /vacaciones` responde 400 "no corresponde a un empleado registrado" justo después de crear el empleado | La réplica se alimenta por eventos: esperar un instante y reintentar (consistencia eventual, milisegundos) |
| La prueba de deduplicación no crea ni el primer registro | Ese `id` de envelope ya se había procesado antes: usar un `id` nuevo |
| `demo.sh` dice "E001 ya existe" | Necesita volúmenes limpios: `docker compose down -v && docker compose up --build -d` |
| La bandeja de Mailhog está vacía | Guarda los correos en memoria: se borran al reiniciar su contenedor (`down`/`up`, paso 11). Generar una notificación nueva |
| `/health` muestra `broker: DOWN` en un servicio | El broker cayó o está reiniciando: los servicios reconectan solos en unos segundos |
