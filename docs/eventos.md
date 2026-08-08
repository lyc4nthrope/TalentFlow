# Eventos — TalentFlow

Documento vivo: registro de los eventos implementados del sistema de microservicios.

## Catálogo del proyecto final (referencia)

| Evento | Publicado por | Consumido por |
|---|---|---|
| `empleado.creado` | Gestión de Empleados | Autenticación, Perfiles, Notificaciones |
| `empleado.actualizado` | Gestión de Empleados | Perfiles |
| `empleado.retirado` | Gestión de Empleados | Autenticación, Perfiles, Notificaciones |
| `usuario.creado` | Autenticación | Notificaciones |
| `usuario.recuperacion` | Autenticación | Notificaciones |
| `cuenta.activada` / `cuenta.desactivada` | Autenticación | Notificaciones |
| `vacaciones.programadas` / `vacaciones.iniciadas` / `vacaciones.finalizadas` | Gestión de Vacaciones | Autenticación, Notificaciones |

Todos viajan en un envelope común: `id`, `type`, `version`, `occurredAt`, `producer`, `data`.
Los consumidores DEBEN descartar mensajes duplicados usando `id`.

## Implementados hasta ahora

| Reto | Evento | Estado |
|---|---|---|
| 1 | — | Ninguno aún (no se requiere: sin message broker) |

El message broker entra en retos posteriores; la definición normativa del catálogo se documentará
aquí cuando se implemente.
