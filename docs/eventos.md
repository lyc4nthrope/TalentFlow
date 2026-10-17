# Eventos — TalentFlow

Documento vivo: registro de los eventos del ecosistema y de su implementación. La **fuente normativa** es el **Catálogo de Eventos** del curso (nombres, envelope, cargas útiles, productores y consumidores); este documento solo registra qué está implementado y dónde. En caso de discrepancia, prevalece el catálogo.

## Convención y envelope

- Nombre: `<entidad>.<acción-en-pasado>`, en minúsculas y sin acentos (`empleado.creado`).
- Todo evento viaja en el mismo sobre; `id` identifica al **mensaje** (no a la entidad) y es la clave de deduplicación:

```json
{
  "id": "3f9b2c10-8e4a-4d6b-9f21-7c0a5d8e1234",
  "type": "empleado.creado",
  "version": 1,
  "occurredAt": "2026-03-01T14:32:05Z",
  "producer": "empleados-service",
  "data": { }
}
```

## Transporte (Reto 4)

| Elemento | Valor |
|---|---|
| Broker | RabbitMQ 3.13 |
| Exchange | `talentflow.eventos` (topic, durable) — **routing key = `type`** |
| Mensajes | Persistentes, `content_type: application/json`, `message_id` = `id` del envelope |
| Topología | [`infra/rabbitmq/definitions.json`](../infra/rabbitmq/definitions.json), importada por `broker-init` |

## Implementados (Reto 4)

| Evento | Catálogo | Productor | Consumidores (cola) | Carga útil (`data`) |
|---|---|---|---|---|
| `empleado.creado` | 3.1 | empleados-service (`POST /empleados`) | perfiles (`perfiles.eventos`), notificaciones (`notificaciones.eventos`), vacaciones (`vacaciones.eventos`) | `empleadoId, nombre, apellido, email, numeroEmpleado, cargo, area, departamentoId, fechaIngreso, estado` |
| `empleado.actualizado` | 3.2 | empleados-service (`PUT /empleados/{id}`) | perfiles, vacaciones | `empleadoId, nombre, apellido, email, cargo, area, departamentoId` |
| `empleado.retirado` | 3.3 | empleados-service (`DELETE /empleados/{id}`) | perfiles, notificaciones, vacaciones | `empleadoId, email, fechaRetiro, motivo` |
| `vacaciones.programadas` | 3.8 | vacaciones-service (`POST /vacaciones`) | notificaciones | `vacacionesId, empleadoId, email, fechaInicio, fechaFin, diasHabiles` |
| `vacaciones.iniciadas` | 3.9 | vacaciones-service (scheduler, Reto 5) | consumidores en el Reto 5: auth-service | `vacacionesId, empleadoId, email, fechaInicio, fechaFin` |
| `vacaciones.finalizadas` | 3.10 | vacaciones-service (scheduler, Reto 5) | consumidores en el Reto 5: auth-service | `vacacionesId, empleadoId, email, fechaFin` |

**Fechas de los eventos `vacaciones.*` (desde el Reto 5).** Los períodos tienen fecha y hora, así que `fechaInicio` y `fechaFin` viajan en ISO-8601 **con el desfase** de `ZONA_HORARIA`: `"2026-10-20T08:00:00-05:00"`. Es una desviación documentada del catálogo, cuyos ejemplos muestran solo la fecha (`"2026-03-15"`); los nombres y el conjunto de campos no cambian. El desfase evita que un consumidor interprete la hora en otra zona. El scheduler publica `vacaciones.iniciadas` al llegar `fechaInicio` y `vacaciones.finalizadas` al llegar `fechaFin` (decisiones D6 y D7 de [`docs/reto-05/PLAN.md`](reto-05/PLAN.md)).

Consumidores adicionales al catálogo: **vacaciones-service** consume los tres eventos de empleado para mantener su réplica local (decisión (b) del Reto 4, ver README raíz). No se agregaron ni renombraron eventos ni campos.

Reacciones por consumidor:

| Consumidor | `empleado.creado` | `empleado.actualizado` | `empleado.retirado` | `vacaciones.programadas` |
|---|---|---|---|---|
| perfiles-service | Crea el perfil por defecto | Sincroniza nombre y email (crea el perfil si no existía) | Archiva el perfil | — |
| notificaciones-service | BIENVENIDA ¹ | — | DESVINCULACION ¹ | VACACIONES |
| vacaciones-service | Registra al empleado en la réplica | Actualiza su email | Lo marca retirado | — |

¹ Según el Reto 4. El catálogo mueve la bienvenida a `usuario.creado` y la despedida a `cuenta.desactivada` (auth-service, Reto 5).

Garantías de los consumidores: deduplicación por `id` (tabla `eventos_procesados` en la misma transacción que el efecto), `ack` solo tras el commit, mensajes inválidos descartados con log, fallos transitorios reencolados con espera.

## Pendientes (Reto 5)

| Evento | Catálogo | Productor | Consumidores |
|---|---|---|---|
| `usuario.creado` | 3.4 | auth-service | notificaciones |
| `usuario.recuperacion` | 3.5 | auth-service | notificaciones |
| `cuenta.activada` | 3.6 | auth-service | notificaciones |
| `cuenta.desactivada` | 3.7 | auth-service | notificaciones |
| `vacaciones.iniciadas` | 3.9 | vacaciones-service (scheduler) — **ya se publica** | auth-service, notificaciones |
| `vacaciones.finalizadas` | 3.10 | vacaciones-service (scheduler) — **ya se publica** | auth-service, notificaciones |
