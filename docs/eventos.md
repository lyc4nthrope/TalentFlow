# Eventos — TalentFlow

Documento vivo. La fuente de verdad de nombres, productores, consumidores y cargas útiles es el
**Catálogo de Eventos** (`catalogo-de-eventos.docx`); este archivo solo registra lo implementado.

## Topología del broker (RabbitMQ)

| Elemento | Valor |
|---|---|
| Exchange | `talentflow.events` — tipo `topic`, durable (lo declara cada servicio al conectar) |
| Routing key | el nombre del evento: `empleado.creado`, `vacaciones.programadas`, ... |
| Mensaje | JSON con el envelope `id, type, version, occurredAt, producer, data`; `delivery_mode=2`; `message_id` = `id` |
| Colas | una durable por servicio consumidor (abajo) |

> **Contrato para `empleados-service`:** publicar en el exchange `talentflow.events` con
> routing key = `type`. Variables: `BROKER_URL`, `BROKER_EXCHANGE`.

## Implementados

| Evento | Productor | Consumidores (cola) | Reto |
|---|---|---|---|
| `empleado.creado` | empleados-service (⏳ pendiente) | perfiles (`perfiles-service.q`), notificaciones (`notificaciones-service.q`), vacaciones (`vacaciones-service.q`) | 4 |
| `empleado.actualizado` | empleados-service (⏳ pendiente) | perfiles | 4 |
| `empleado.retirado` | empleados-service (⏳ pendiente) | perfiles, notificaciones, vacaciones | 4 |
| `vacaciones.programadas` | **vacaciones-service** ✅ | notificaciones | 4 |

Reacciones: ver README raíz, sección *Reto 4*.

### Desviación documentada respecto del Catálogo

`vacaciones-service` consume también `empleado.creado` y `empleado.retirado`, que en el catálogo
(3.1, 3.3) no lo listan como consumidor. Es la opción (b) del enunciado (réplica local), que el
Reto 4 permite explícitamente. **Pendiente:** agregar a `vacaciones-service` como consumidor en
el catálogo (la regla del catálogo es "se agrega aquí primero").

### Nota sobre la bienvenida

El enunciado del Reto 4 pide que `notificaciones-service` registre la BIENVENIDA al consumir
`empleado.creado`. El catálogo indica que, desde el Reto 5, la bienvenida pasa a dispararse con
`usuario.creado` (que trae el token de activación). Se sigue el enunciado del Reto 4 y habrá que
moverlo en el Reto 5.

## Deduplicación (catálogo 2.1)

Cada consumidor tiene su propia tabla `eventos_procesados(id, procesado_en)`. El `INSERT` del id y el
efecto del evento van en **la misma transacción**: duplicado → no inserta → se confirma (ack) sin repetir
el efecto; si el efecto falla → se deshace también el id → la reentrega reintenta.

Política de confirmación (igual en los tres servicios):

| Situación | Acción |
|---|---|
| procesado / duplicado / evento ajeno | `ack` |
| mensaje inválido (JSON roto, faltan campos) | `reject` sin reencolar (la DLQ formal es del Reto 29) |
| fallo transitorio (p. ej. BD caída) | pausa de 2 s y `nack` con reencolado |
