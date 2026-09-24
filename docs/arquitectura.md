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

## Arquitectura actual (Reto 3)

```
                         Cliente HTTP (curl, Postman, Bruno)
                                       │
                                única URL base
                                       ▼
                         ┌─────────────────────────┐
                         │   api-gateway  :8080     │  ← único puerto publicado al host
                         │  (Node.js + Express +    │
                         │   http-proxy-middleware) │
                         └────────────┬─────────────┘
                       /empleados/*   │   /departamentos/*
                    ┌──────────────────┴──────────────────┐
                    ▼                                     ▼
         empleados-service :8081                departamentos-service :8082
         (Node.js + Express)                     (PHP nativo)
                    │   HTTP REST + Circuit Breaker (opossum)
                    └────────────────────────────────────► departamentos-service
                    │                                     │
                    ▼                                     ▼
         database-empleados                       database-departamentos
         (PostgreSQL, expose interno)             (MySQL, expose interno)
```

Detalle de rutas, parámetros del Circuit Breaker y justificación de decisiones: ver el
[README raíz](../README.md).

## Estado actual (Reto 3)

| Componente | Estado |
|---|---|
| `microservicios/gestion-empleados` (Node + Express + Postgres) | ✅ Implementado: POST/GET, modelo canónico, validaciones, Circuit Breaker hacia departamentos |
| `microservicios/gestion-departamentos` (PHP + MySQL) | ✅ Implementado: POST/GET, persistencia, healthcheck |
| `microservicios/api-gateway` (Node + Express) | ✅ Implementado: único punto de entrada, enrutamiento `/empleados/*` y `/departamentos/*`, `503` JSON ante fallo, `/health` propio |
| Circuit Breaker (`empleados → departamentos`) | ✅ Implementado con `opossum`, fallback `PENDIENTE_VALIDACION` (no rechazo), reconciliación automática al cerrar el circuito verificada |
| Auth / Perfiles / Vacaciones / Notificaciones | ⏳ Retos futuros |
| Message broker, observabilidad, CI/CD | ⏳ Retos futuros (asíncrono: Reto 4; observabilidad: Reto 8) |

## Decisiones de arquitectura

- Monorepo con npm workspaces (`microservicios/*`); **no existe código compartido entre
  microservicios** desde el Reto 2 — el paquete `shared/` del Reto 1 se eliminó por ser un
  patrón de monolito. Cada servicio implementa su propia validación; la única comunicación
  entre ellos es HTTP.
- Capas por microservicio: HTTP (app.js) → lógica de negocio (service) → almacenamiento (repository).
- Persistencia poliglota desde el Reto 2: PostgreSQL para `empleados-service`, MySQL para
  `departamentos-service` — motor distinto a propósito, para forzar comunicación real por HTTP
  en vez de compartir base de datos entre servicios.
- Desde el Reto 3, **único punto de entrada**: solo `api-gateway` publica puerto al host
  (`ports:`); todo lo demás (microservicios y bases de datos) usa `expose:` y solo es
  alcanzable dentro de la red de Docker.
- Gateway de aplicación (código, no un enrutador declarativo como Nginx/Traefik): los retos
  futuros (5, 10) exigen que el Gateway valide JWT/JWKS y propague identidad, lo cual requiere
  lógica propia, no solo enrutamiento por labels.
- Circuit Breaker con la librería del ecosistema (`opossum`), no implementado a mano, para
  poder usar sus métricas de estado en el Reto 11 (Grafana). Fallback de negocio:
  **disponibilidad sobre consistencia inmediata** — el empleado se registra con
  `estado: "PENDIENTE_VALIDACION"` en vez de rechazarse con `503`, y se reconcilia
  automáticamente (sin job periódico ni webhook) cuando el circuito vuelve a `CLOSED`.