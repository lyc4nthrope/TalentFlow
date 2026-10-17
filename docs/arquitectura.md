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

## Arquitectura actual (Reto 4)

```
Cliente ──REST──▶ api-gateway :8080 ──REST──▶ empleados · departamentos · perfiles · notificaciones · vacaciones
                                                    │  (Circuit Breaker empleados → departamentos)
                   empleados ──publica──┐           │
                   vacaciones ─publica──┤           ▼
                                        ▼      cada uno con su propia BD
                              RabbitMQ (exchange topic "talentflow.eventos")
                                        │ fan-out
                        ┌───────────────┼───────────────┐
                        ▼               ▼               ▼
                    perfiles     notificaciones     vacaciones (réplica de empleados)
```

Diagrama detallado, servicios ↔ lenguajes, eventos y justificación de decisiones: ver el
[README raíz](../README.md).

## Estado actual (Reto 4)

| Componente | Estado |
|---|---|
| `microservicios/api-gateway` (Node + Express) | ✅ Único punto de entrada para los 5 servicios, `503`/`404` JSON, `/health` agregado con `problemas` |
| `microservicios/gestion-empleados` (Node + Express + Postgres) | ✅ POST/GET/PUT, baja lógica (`DELETE` → RETIRADO), auditoría por fechas, Circuit Breaker hacia departamentos, publica `empleado.creado/actualizado/retirado` |
| `microservicios/gestion-departamentos` (PHP + MySQL) | ✅ POST/GET, persistencia, `/health` con verificación de BD, pruebas de integración PHP |
| `microservicios/gestion-perfiles` (Python + FastAPI + Postgres) | ✅ Consume eventos de empleado (crea, sincroniza, archiva perfiles) + REST |
| `microservicios/notificaciones` (Go + Postgres) | ✅ Solo consume eventos; simula el envío (log), envía el correo real por SMTP a Mailhog (bonus) y guarda el historial |
| `microservicios/gestion-vacaciones` (Java + Spring Boot + Postgres) | ✅ REST con 4 validaciones, réplica de empleados por eventos, publica `vacaciones.programadas` |
| Message broker (RabbitMQ) | ✅ Exchange topic, una cola por consumidor, deduplicación por id de mensaje en todos los consumidores |
| Auth (JWT) y scheduler de vacaciones | ⏳ Reto 5 |
| Observabilidad, CI/CD | ⏳ Retos futuros (observabilidad: Reto 8) |

## Decisiones de arquitectura

- Monorepo con npm workspaces (`microservicios/*`); **no existe código compartido entre
  microservicios** desde el Reto 2 — el paquete `shared/` del Reto 1 se eliminó por ser un
  patrón de monolito. Cada servicio implementa su propia validación; la única comunicación
  entre ellos es HTTP o eventos.
- Capas por microservicio: HTTP (app.js) → lógica de negocio (service) → almacenamiento (repository).
- Persistencia poliglota desde el Reto 2: PostgreSQL para `empleados-service`, MySQL para
  `departamentos-service` — motor distinto a propósito, para forzar comunicación real por HTTP
  en vez de compartir base de datos entre servicios.
- Desde el Reto 3, **único punto de entrada**: solo `api-gateway` publica puerto al host
  (`ports:`); todo lo demás (microservicios y bases de datos) usa `expose:` y solo es
  alcanzable dentro de la red de Docker. Excepciones (Reto 4): las UIs de dos herramientas,
  RabbitMQ (`15672`) y Mailhog (`8025`); sus puertos de servicio (AMQP, SMTP) siguen internos.
- Gateway de aplicación (código, no un enrutador declarativo como Nginx/Traefik): los retos
  futuros (5, 10) exigen que el Gateway valide JWT/JWKS y propague identidad, lo cual requiere
  lógica propia, no solo enrutamiento por labels.
- Circuit Breaker con la librería del ecosistema (`opossum`), no implementado a mano, para
  poder usar sus métricas de estado en el Reto 11 (Grafana). Fallback de negocio:
  **disponibilidad sobre consistencia inmediata** — el empleado se registra con
  `validacionDepartamento: "PENDIENTE"` en vez de rechazarse con `503`, y se reconcilia
  automáticamente (sin job periódico ni webhook) cuando el circuito vuelve a `CLOSED`.
- Comunicación asincrónica (Reto 4) con **RabbitMQ**: exchange *topic* `talentflow.eventos` con
  routing key = tipo de evento y una cola por consumidor (fan-out); agregar un consumidor no toca
  al productor. Eventos con el envelope del Catálogo; los consumidores deduplican por `id`.
- Validación del empleado en vacaciones por **réplica local alimentada por eventos**
  (disponibilidad sobre consistencia inmediata).
- Diversidad tecnológica: 5 lenguajes (Node.js, PHP, Python, Go, Java), cada servicio con su BD.
