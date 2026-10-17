# Reto 5 — Integración del trabajo del equipo

Este documento explica cómo se juntó el trabajo del Reto 5 que hicieron Valeria (`Vale292005`) y Cristhian (`lyc4nthrope`) en una sola versión: qué hizo cada uno, qué se conservó, qué se reemplazó y por qué. También explica cómo poner al día una rama personal con `dev`.

El detalle técnico completo está en [`PLAN.md`](PLAN.md) (decisiones D1 a D23 y aprendizajes por fase). La explicación funcional está en el [README raíz](../../README.md) y en el [manual del reto](README.md).

## 1. Línea de tiempo

| Fecha | Rama | Qué pasó |
|---|---|---|
| hasta 2026-10-15 | `Vale292005` (`cc4c2a3`) | Primer avance del Reto 5: `auth-service` en Go con arquitectura hexagonal, login con JWT, validación del token en el Gateway, estados de la cuenta y scheduler de vacaciones |
| 2026-10-15 | `lyc4nthrope` | Ese avance se integró **tal cual** (*fast-forward*: la autoría de Valeria se conserva en la historia) y se auditó contra `reto5.pdf`, el Catálogo de Eventos y la rúbrica |
| 2026-10-15 | `Vale292005` (`87e28f9`) | Valeria subió un segundo commit ("puliendo detalles del reto 05"), en paralelo a la auditoría |
| 2026-10-15 a 2026-10-17 | `lyc4nthrope` | Fases F0 a F10 del plan: se completó el reto y se auditó también el segundo commit de Valeria |

## 2. Lo que aportó Valeria y se conservó

- **La estructura de `auth-service`**: Go con arquitectura hexagonal (dominio, aplicación, infraestructura). Se mantuvo; sobre ella se reescribió la lógica.
- **La estrategia de seguridad**: validar el JWT **en el Gateway** y no con interceptores en cada servicio. Es la opción que recomienda el PDF y se mantuvo (D8).
- **Los estados de la cuenta** en vez de un booleano activo/inactivo: `ACTIVA`, `INACTIVA`, `SUSPENDIDA_TEMPORAL` y `DESACTIVADA_PERMANENTE`. Se mantuvieron; `INACTIVA` pasó a llamarse `PENDIENTE_ACTIVACION`, que dice por qué la cuenta no se puede usar todavía (D3).
- **La zona horaria del scheduler** de `vacaciones-service`, que agregó en su segundo commit al mismo tiempo que se hacía en `lyc4nthrope` (D6). De ese commit se trajo la prueba que la protege (ver la sección 4).
- **El script de pruebas `test_reto5.sh`**: cada escenario que verifica (401 sin token, activación, login, lectura con USER, 403 en escritura, propiedad del perfil) está incluido en [`demo.sh`](demo.sh), que además cubre los 18 pasos del PDF.

## 3. Lo que se cambió del primer avance (`cc4c2a3`) y por qué

La auditoría del 2026-10-15 encontró estos problemas. Cada uno está ligado a un criterio de la rúbrica, por eso no se podían dejar así.

| # | Problema encontrado | Por qué importaba | Cómo quedó |
|---|---|---|---|
| H1 | `vacaciones-service` no arrancaba: ninguna clase implementaba `EventoPublisherPort` | Rompía el Reto 5 y también el Reto 4 | F1: se usa el publicador que ya existía; una prueba levanta el contexto de Spring y falla si falta un bean |
| H2 | `auth-service` solo consumía `empleado.creado`; nunca publicaba `cuenta.desactivada`; el caso borde no existía | Es el núcleo del reto (ciclo de vida de la cuenta) | F3: consume los 4 eventos, publica `cuenta.activada` / `cuenta.desactivada` y resuelve el caso borde (D5, D18, D19) |
| H3 | Token de recuperación predecible (`reset_<nanosegundos>`), sin vencimiento; el reset reactivaba cuentas suspendidas o retiradas | Un token adivinable permite tomar cuentas ajenas | F2: UUID aleatorio guardado como hash, de un solo uso y con vencimiento (D1); el reset nunca saca a una cuenta de la suspensión |
| H4 | El Gateway respondía 403 a las lecturas del rol USER | El PDF exige que USER pueda leer | F5: matriz de autorización completa (D8) |
| H5 | `env.js` versionado con credenciales, contraseña del admin impresa en el log, secretos por defecto en el código, `auth-service` publicado en el host | Rúbrica, criterio 5 (configuración y secretos) | F5 y F6: sin secretos en el código, secreto inyectado por Compose, `auth-service` solo detrás del Gateway (D11, D21, D22, D23) |
| — | Eventos inventados (`auth.password_*`), payloads distintos al catálogo, Swagger sin BearerAuth, sin documentación | El catálogo es normativo y la rúbrica evalúa Swagger y la documentación | F3, F4, F7 y F9 |

## 4. El segundo commit de Valeria (`87e28f9`)

Se auditó solo ese commit, cambio por cambio, comparándolo con la versión de `lyc4nthrope`. Compila y sus pruebas pasan, y su intención coincide con el PDF. La conclusión es que **cada cambio ya estaba resuelto**, en la mayoría de los casos de forma más estricta:

| Cambio | Intención | Resultado |
|---|---|---|
| `expose: 8086` en `auth-service` | Sacarlo del acceso directo | Conservaba también `ports: 8086:8086`, así que seguía publicado. En la versión integrada solo tiene `expose` (D22) |
| Roles y propiedad del perfil en el Gateway | USER lee todo y edita solo su perfil | Correcto en lo esencial, pero `PUT /perfiles/E001/lo-que-sea` pasaba el chequeo y permitía PATCH. La versión integrada exige la ruta exacta (D8) |
| Métodos para suspender, reactivar y desactivar la cuenta | Ciclo de vida de la cuenta | Ningún consumidor de eventos los llamaba (solo las pruebas), así que las cuentas nunca cambiaban de estado. La versión integrada los conecta a los eventos, con deduplicación (F3) |
| Payloads de `cuenta.*` y `vacaciones.*` | Publicar los eventos del reto | Usaban `usuarioId` en vez de `empleadoId` y les faltaban `email`, `fechaInicio` y `fechaFin`, que el catálogo exige (secciones 3.6 a 3.10) |
| Decodificador de notificaciones | Un solo caso para los eventos de seguridad | Esperaba campos (`usuarioId`, `token`) que el catálogo no define, así que rechazaría los eventos correctos |
| Zona horaria del scheduler | Que el cron use la hora del negocio | Ya estaba igual en la versión integrada. **Se trajo su prueba** (commit `d13af5d`): protege el caso de un `VACACIONES_CRON` con hora fija, que sin zona se evaluaría en UTC |

## 5. Cómo poner al día una rama personal con `dev`

El flujo normal del equipo es `git merge dev` (ver el README raíz). En `Vale292005` eso produce **7 conflictos**, porque el commit `87e28f9` modifica archivos que en la versión integrada se reescribieron o se eliminaron. Como todo su contenido ya está cubierto, la forma limpia es hacer el merge y quedarse con el árbol de `dev`:

```bash
git checkout Vale292005
git pull origin Vale292005
git fetch origin
git branch respaldo-vale-reto5          # respaldo local, por si acaso
git merge --no-commit origin/dev        # muestra los conflictos; no se resuelven a mano
git read-tree -u --reset origin/dev     # el contenido queda exactamente igual a dev
git commit -m "merge: integrar el Reto 5 de dev en Vale292005"
git push origin Vale292005              # push normal, sin --force
```

Resultado (ensayado en un clon antes de escribir esto): la rama queda idéntica a `dev`, los commits `cc4c2a3` y `87e28f9` siguen en su historia y el push no reescribe nada. A partir de ahí, el flujo diario vuelve a ser el de siempre: `git merge dev` antes de empezar.

## 6. Estado final

- `lyc4nthrope` → `dev` → `main` con el Reto 5 completo (que también incluye el Reto 4, que no se había llevado a `dev`).
- Pruebas automatizadas: empleados 81, api-gateway 134, auth-service 116, notificaciones 33, vacaciones 63 y perfiles 29, todas pasando.
- `docker compose up --build` desde cero: todos los contenedores sanos; la demo de los 18 pasos del PDF pasa completa ([`pruebas.md`](pruebas.md)).
