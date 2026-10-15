# Reto 5 — Plan de implementación

Fuente de requisitos: `reto5.pdf` + **Catálogo de Eventos** (secciones 3.4 a 3.10, normativo) + `seguridad.pdf`.
Punto de partida: el avance de Valeria (`Vale292005`, `cc4c2a3`) integrado sin conflictos en `lyc4nthrope`.
Cada fase termina **verificada** (pruebas + comprobación real en Docker cuando aplica), con su propio commit y un resumen en memoria, para poder retomar en cualquier punto.

## Diagnóstico de partida (auditoría del 2026-10-15)

| # | Problema | Criterio de la rúbrica |
|---|---|---|
| H1 | `vacaciones-service` no arranca: ninguna clase implementa `EventoPublisherPort` (verificado al arrancar el jar) | 4 (y rompe el Reto 4) |
| H2 | `auth-service` solo consume `empleado.creado`; nunca publica `cuenta.desactivada`; el caso borde no existe | 4 |
| H3 | Token de recuperación predecible (`reset_<nanosegundos>`), sin expiración; el reset reactiva cuentas suspendidas o retiradas | 1 |
| H4 | El Gateway responde 403 a lecturas del rol USER (el PDF exige solo lectura) | 3 |
| H5 | `api-gateway/src/infrastructure/config/env.js` versionado con credenciales; contraseña del admin impresa en el log; secretos por defecto en el código; `auth-service` publica su puerto | 5 |
| — | Eventos inventados (`auth.password_*`), payloads distintos al catálogo, sin BearerAuth en Swagger, sin documentación del Reto 5 | 1, 4, 6 |

## Decisiones de diseño

| # | Decisión | Elección | Por qué |
|---|---|---|---|
| D1 | Token de activación / recuperación | **Opción B: UUID aleatorio** guardado como hash SHA-256 con `expira_en`; de un solo uso. Activación 24 h, recuperación 1 h (configurables) | El catálogo usa UUID (`tokenActivacion`, `tokenRecuperacion`) y `expiraEn`. Un solo uso y revocable al instante. Guardar el hash evita que una fuga de la BD entregue tokens válidos |
| D2 | Token de acceso | JWT HS256 con `sub` = `empleadoId` (o `admin` para la semilla), `role` = `ADMIN`\|`USER`, `email`, `iat`, `exp` | Estructura exigida por el PDF. La regla de propiedad compara `recurso.empleadoId == token.sub` |
| D3 | Estados de la cuenta | `PENDIENTE_ACTIVACION` · `ACTIVA` · `SUSPENDIDA_TEMPORAL` · `DESACTIVADA_PERMANENTE` | El PDF exige distinguir la suspensión temporal de la baja permanente (no un booleano). Restablecer la contraseña nunca saca a una cuenta de un estado suspendido o retirado |
| D4 | Consumo de eventos en `auth-service` | Cola `auth.eventos` ligada a `empleado.creado`, `empleado.retirado`, `vacaciones.iniciadas`, `vacaciones.finalizadas`; despacho por `type`; deduplicación por `id` del envelope en la misma transacción que el efecto; `ack` tras el commit; mensajes envenenados descartados | Mismo patrón que los consumidores del Reto 4 (D5 del Reto 4) |
| D5 | Caso borde | `vacaciones.finalizadas` solo reactiva si la cuenta está en `SUSPENDIDA_TEMPORAL`; sobre `DESACTIVADA_PERMANENTE` se ignora y se registra en el log | El catálogo y el PDF lo exigen y lo evalúan |
| D6 | Fechas de vacaciones | **Fecha y hora reales** (`LocalDateTime`) en la zona `ZONA_HORARIA` (America/Bogota). Se acepta `YYYY-MM-DDTHH:mm[:ss]`; por compatibilidad con el Reto 4 también `YYYY-MM-DD` (inicio = 00:00, fin = 23:59:59 de ese día). Reglas: `fechaFin > fechaInicio`; `fechaInicio` no puede estar en el pasado respecto de **ahora**; rangos semiabiertos `[inicio, fin)` para el solapamiento (un período puede empezar justo cuando termina otro). Columnas `TIMESTAMP`; `diasHabiles` = lunes a viernes entre la fecha de inicio y la de fin, inclusive. En los eventos las fechas viajan en ISO-8601 con desfase (`2026-10-20T08:00:00-05:00`) | Estrategia 2 del PDF (sección 3.3): permite demostrar el ciclo completo en minutos con fechas reales. Desviación documentada: el catálogo muestra las fechas sin hora; los nombres de los campos no cambian. El desfase evita que un consumidor interprete la hora en otra zona |
| D7 | Scheduler | `@Scheduled(cron = "${VACACIONES_CRON}")`, cada minuto por defecto. Cada pasada: `PROGRAMADA` con `fechaInicio <= ahora` → `EN_CURSO` + `vacaciones.iniciadas`; `EN_CURSO` con `fechaFin <= ahora` → `FINALIZADA` + `vacaciones.finalizadas`. Cada período en su propia transacción; el evento se publica después del commit. Un solo puerto de salida de eventos (`PublicadorEventos`): se elimina el puerto duplicado `EventoPublisherPort` | Con hora no hacen falta endpoints `forzar-*`: inicio = ahora + 2 min y fin = ahora + 5 min se demuestra solo. Publicar tras el commit es la regla ya aceptada en el Reto 4 (D6); si la publicación falla queda en el log (limitación conocida, Outbox en retos posteriores). Una sola instancia (limitación de escalado documentada: ShedLock) |
| D8 | Autorización en el Gateway | ADMIN: todo. USER: cualquier `GET`; `POST /auth/change-password`; `PUT /perfiles/{sub}`. Resto: 403. Sin token válido: 401 | Matriz del PDF. Se decide en un solo lugar (opción 1 recomendada por el PDF) |
| D9 | Rutas públicas | `/health`, `/auth/login`, `/auth/recover-password`, `/auth/reset-password` y la documentación (`/*/docs`, `/*/openapi.json`) | Sin token no se puede obtener un token; Swagger debe poder abrirse para probar con BearerAuth |
| D10 | Identidad hacia los servicios | El Gateway **elimina** las cabeceras `X-User-*` que envíe el cliente y agrega las suyas tras validar el token | Evita que un cliente se haga pasar por otro usuario enviando las cabeceras a mano |
| D11 | Secreto del JWT | Obligatorio por variable de entorno en `auth-service` y Gateway; sin valor por defecto en el código (el servicio no arranca sin él). Valor académico documentado en `.env.example` | Rúbrica, criterio 5: nada "quemado" en el código |
| D12 | Notificaciones | `usuario.creado` / `usuario.recuperacion` → `Tipo: SEGURIDAD`; `cuenta.desactivada` / `cuenta.activada` → `Tipo: CUENTA`, con los mensajes del PDF. La bienvenida pasa de `empleado.creado` a `usuario.creado` y la despedida de `empleado.retirado` a `cuenta.desactivada` (RETIRO) | Lo anticipó la D8 del Reto 4. `vacaciones.finalizadas` no se consume en notificaciones: la "bienvenida de regreso" sale de `cuenta.activada` (FIN_VACACIONES), así un retirado durante sus vacaciones no la recibe |
| D13 | Swagger | Esquema `bearerAuth` (HTTP bearer, JWT) y seguridad global en el OpenAPI de cada servicio; `auth-service` con su propio OpenAPI | Rúbrica, criterio 6 |

## Fases

| Fase | Contenido | Verificación | Estado |
|---|---|---|---|
| **F0** Preparación | Respaldo y limpieza de ramas; integrar `Vale292005`; este plan | Ramas: `lyc4nthrope`, `Vale292005`, `dev`, `main` | ✅ |
| **F1** Vacaciones | Fechas con hora (D6); scheduler y publicación de `vacaciones.iniciadas` / `vacaciones.finalizadas` con los campos del catálogo 3.9 y 3.10 (D7); `VACACIONES_CRON` en compose y `.env.example`; prueba de arranque del contexto de Spring | Pruebas; el jar arranca; en Docker, un período de minutos dispara ambos eventos con el formato correcto | ✅ |
| **F2** Auth: cuentas y tokens | D1, D2, D3; política de contraseñas; respuestas que no revelan si un email existe; sin contraseña del admin en el log (semilla por variables de entorno) | Pruebas unitarias del servicio y del token | ⬜ |
| **F3** Auth: ciclo de vida | D4, D5; publicar los 4 eventos con el payload exacto y `id` UUID; quitar `auth.password_*` de la topología | Pruebas; en Docker: alta → activación → suspensión → reactivación → retiro, y el caso borde | ⬜ |
| **F4** Notificaciones | D12 | Pruebas; logs con el formato del PDF | ⬜ |
| **F5** Gateway | D8, D9, D10; eliminar el árbol `src/` sin uso; Gateway importable para pruebas | Pruebas de la matriz completa (401 / 403 / 200 y propiedad) | ⬜ |
| **F6** Configuración | D11; borrar `env.js`; `auth-service` con `expose` y `/health`; compose y `.env.example` al día | `docker compose up` desde cero, todo sano | ⬜ |
| **F7** Swagger | D13 en los 7 servicios | Probar desde Swagger con el botón *Authorize* | ⬜ |
| **F8** Pruebas E2E | `docs/reto-05/demo.sh` con los 18 pasos del PDF; colección Bruno; evidencia del caso borde | Desde cero, todos los pasos | ⬜ |
| **F9** Documentación | README: cómo obtener un token, estrategia Gateway vs. interceptores, secreto, diagrama de secuencia, scheduler y su limitación (ShedLock); manual `docs/reto-05/` | Cada entregable del PDF presente | ⬜ |
| **F10** Cierre | Pruebas completas, revisión, `dev` a la par de `lyc4nthrope`, push | Ramas sincronizadas | ⬜ |

## Fuera de alcance (anotado para después)

Defectos de retos anteriores encontrados en la auditoría, sin relación con el Reto 5: `departamentoId` sin codificar en la URL, espera fija entre reintentos (el Reto 2 pide creciente), reconciliación solo al cerrar el circuito, `fechaIngreso` sin validar al crear, capturas o video de los Retos 3 y 4.

## Aprendizajes por fase

- **F0**: respaldo de todas las ramas en `~/Downloads/TalentFlow-respaldo-ramas-2026-10-15.bundle` antes de borrar `davidcr08`, `DillanSnayderBuitrago`, `mejoraDepartamento` y `mejoradepartamento`. La integración de `Vale292005` fue *fast-forward* (la rama no tenía nada que ella no tuviera), así que su autoría se conserva commit a commit. Las pruebas unitarias de vacaciones (29) pasan aunque el servicio no arranca: no levantan el contexto de Spring, por eso F1 incluye arrancar el jar.

- **F1**: se eliminó el puerto duplicado `EventoPublisherPort` (causa de que el servicio no arrancara) y el scheduler usa el `PublicadorEventos` existente. Una prueba nueva levanta el contexto de Spring: falla si vuelve a faltar un bean. Cada período cambia de estado con un `UPDATE ... WHERE estado = esperado`, en su propia transacción, y el evento se publica después del commit (probado con un doble de transacciones que verifica que el publicador nunca se invoca dentro de una). Compatibilidad con el Reto 4: una fecha sin hora sigue siendo válida si es hoy (empieza "ahora"); inicio y fin sin hora en el mismo día ahora es un período de un día (antes 400), cambio intencional que permite el paso 12 del PDF. Pruebas: 61/61, JaCoCo 71.0 % líneas / 69.2 % ramas (antes 43.7 / 55.9); 17 mutaciones detectadas de 17. Verificado en Docker con Postgres y RabbitMQ reales: columnas `timestamp`, `EXCLUDE ... tsrange '[)'`, las 4 validaciones, período contiguo aceptado, un período de 1 minuto pasó solo por `PROGRAMADA → EN_CURSO → FINALIZADA` y publicó `vacaciones.iniciadas` y `vacaciones.finalizadas` con los campos del catálogo y fechas `-05:00`. Un volumen de Postgres existente debe recrearse (`init.sql` solo corre con volumen nuevo). Revisión independiente: 0 bloqueantes, 0 graves; se corrigieron 4 menores (hoy sin hora, trazas en el log, prueba de publicación tras el commit, javadoc de la prueba de arranque) y se descartó uno (`email` nulo: no hay forma de programar sin réplica y la réplica nunca se borra). Pendiente para el Reto 6: pruebas automáticas del SQL contra Postgres real (Testcontainers).
