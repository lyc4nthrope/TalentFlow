# Arquitectura — TalentFlow

Documento vivo: refleja la arquitectura actual del proyecto en cada reto.

## Visión general

Sistema de onboarding y offboarding de empleados basado en microservicios (proyecto final).
El ciclo de vida del empleado: **onboarding → gestión de perfil → vacaciones → offboarding**,
con observabilidad y despliegue unificado.

## Arquitectura objetivo (documento del proyecto)

```
Cliente ──REST──▶ API Gateway ──REST──▶ Microservicios ──eventos──▶ Message Broker
                                           │                          │
                                    (empleados, departamentos,   (consumidores:
                                     auth, perfiles, vacaciones,   perfiles, auth,
                                     notificaciones)               notificaciones)
```

- **Síncrono (REST)**: solo Cliente ↔ API Gateway ↔ Microservicios
- **Asíncrono (eventos)**: entre microservicios vía message broker (RabbitMQ/Kafka/Redis)

## Estado actual (Reto 1)

| Componente | Estado |
|---|---|
| `microservicios/gestion-empleados` (Node + Express) | ✅ Implementado: POST/GET, modelo canónico, validaciones, Docker |
| `shared/` | ✅ Modelo canónico del empleado + AppError (adición propia) |
| API Gateway | ⏳ Reto futuro (27+) |
| Auth / Perfiles / Vacaciones / Notificaciones / Departamentos | ⏳ Retos 2-5 |
| Message broker, observabilidad, CI/CD | ⏳ Retos 3, 7, 8 |

## Decisiones de arquitectura

- Monorepo con npm workspaces: `shared/` + `microservicios/*` (decisión del usuario).
- Capas por microservicio: HTTP (app.js) → lógica de negocio (service) → almacenamiento (repository).
- Modelo canónico en `shared/` para no duplicarlo entre servicios.
- En Reto 1 los datos viven en memoria (Map); se migrará a BD por servicio en retos posteriores.
