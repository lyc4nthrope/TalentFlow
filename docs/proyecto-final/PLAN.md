# Proyecto final — Plan de cierre

Fuente de requisitos: `proyectoFinal.pdf` (requisitos, restricciones obligatorias y criterios de evaluación).
Punto de partida: rama `lyc4nthrope` en `71ce0e4` (igual a `main`, Retos 1 a 8 completados).
Cada fase termina **verificada** (pruebas + ejecución real cuando aplica), con su propio commit.

## Qué cubre cada reto del semestre (nota del PDF)

| Criterio | Peso | Retos | Estado |
|---|---|---|---|
| Funcionalidad | 35 % | 1–5 | ⚠️ casi completo; faltan detalles de CRUD y de perfil |
| Calidad técnica | 25 % | 2, 6, 7 | ⚠️ departamentos sin pruebas ni cobertura |
| Observabilidad | 15 % | 8 | ✅ |
| Seguridad y configuración | 10 % | 5, 9, 10 | ❌ secretos por defecto en el compose; sin Compose Secrets |
| Escalabilidad y resiliencia | 10 % | 3, 11 | ⚠️ hay Circuit Breaker; no hay réplicas ni balanceador |
| DevOps | 5 % | 7 | ✅ (departamentos con pipeline incompleto) |
| Documentación | 5 % | todos | ⚠️ faltan guías de herramientas y el entregable |

Los PDF de los Retos 9, 10 y 11 todavía no están en `context_microservicios/`. Este plan se basa en lo que el proyecto final exige de esos criterios; cuando lleguen los PDF se ajustan las fases F4–F8.

## Diagnóstico de partida (2026-11-09)

| Requisito del PDF | Estado actual (con evidencia) | Brecha |
|---|---|---|
| Departamentos: CRUD | ⚠️ solo `POST`, `GET` lista y `GET` uno (`gestion-departamentos/public/index.php:127,181,187`) | Falta `PUT` y `DELETE` |
| Pruebas y cobertura ≥ 70 % en cada servicio | ⚠️ 6 de 7 servicios con reporte; departamentos solo tiene un script HTTP (`test/test_departamentos.php`) | Pruebas unitarias e integración en PHP, cobertura y Sonar |
| Pipeline: build, test, calidad, package | ⚠️ departamentos solo hace `php -l` y package (`gestion-departamentos/Jenkinsfile:1-5`) | Etapas Test, SonarQube y Quality Gate |
| Perfil: foto, biografía, redes sociales, dirección completa | ⚠️ solo `telefono`, `direccion`, `ciudad`, `biografia` (`gestion-perfiles/app/dominio.py:16-27`) | `foto`, `redesSociales`, `codigoPostal` |
| Vacaciones: CRUD | ⚠️ `POST`, `GET`, `GET` uno y `DELETE`; sin `PUT` (`VacacionesController.java`) | `PUT` con las mismas validaciones |
| Notificaciones: plantillas configurables | ⚠️ textos fijos en Go (`notificaciones/internal/dominio/notificacion.go`) | Plantillas en archivos montados |
| Cero secretos en compose y repositorio | ❌ 15 valores por defecto, p. ej. `${JWT_SECRET:-talentflow_super_secret_2026}` (`docker-compose.yml:6`), `Admin123*` (`:189`), `RABBITMQ_PASS:-admin` | Quitar todos los valores por defecto |
| Script que **genere** los secretos | ❌ `scripts/setup-secrets.sh` solo copia `.env.example` | Generar valores aleatorios |
| Docker Compose Secrets (o Vault) | ❌ no hay ninguna sección `secrets:` | Secretos como archivos en `secrets/` |
| Escalado horizontal + balanceador | ❌ `container_name` en 26 servicios, sin réplicas, Gateway fijo en `8080` | Réplicas y balanceador con health checks |
| Seguir atendiendo si cae una instancia | ❌ hay una sola instancia de cada servicio | Demostración con réplicas |
| Timeouts y reintentos | ⚠️ solo empleados → departamentos (`departamentos.client.js:9-82`); el proxy del Gateway no tiene timeout (`api-gateway/app.js:174-180`) | Timeout del proxy |
| Redes segmentadas (diagrama del PDF) | ⚠️ una sola red, `microservices-network` | Redes por capa |
| `GET /empleados/{id}/completo` (puntaje extra) | ❌ no existe | Endpoint con manejo de fallo parcial |
| IdP con OAuth2/OIDC y JWKS (puntaje extra) | ❌ JWT HS256 con secreto compartido (`api-gateway/autenticacion.js:21`) | Keycloak (opcional) |
| Entregable comprimido | ❌ no existe | `.tar.gz` verificado desde cero |

## Decisiones de diseño

| # | Decisión | Elección | Por qué |
|---|---|---|---|
| D1 | Orden de los bloques | Calidad → Seguridad → Escalabilidad → Extras → Entrega | Primero lo que hoy hace fallar un criterio obligatorio (cobertura de departamentos, secretos). Los extras solo suman si lo obligatorio está completo |
| D2 | Rama de trabajo | Todo en `lyc4nthrope`; se sincroniza con `main` solo avanzando (`--ff-only`) | El usuario quiere hacerlo solo, en su rama |
| D3 | Variables obligatorias en el compose | `${VAR:?mensaje}` en vez de `${VAR:-valor}` | Si falta un secreto, el compose no arranca y dice cuál falta. Un valor por defecto es un secreto versionado |
| D4 | Gestión de secretos | Docker Compose Secrets: un archivo por secreto en `secrets/`, leído con la convención `*_FILE` | Lo pide el PDF; no aparece en `docker inspect` ni en las variables de entorno. Vault se descarta: otro servicio más para el mismo resultado en local |
| D5 | Valores viejos en el historial de Git | No reescribir el historial; los valores de desarrollo dejan de ser válidos porque el script genera otros | Reescribir el historial es destructivo y rompe las ramas de la compañera. Un valor que ya no abre nada no es un secreto |
| D6 | Balanceador | Nginx delante de las réplicas del Gateway | Configuración explícita y fácil de explicar. Traefik también sirve, pero agrega etiquetas en cada servicio |
| D7 | Scheduler de vacaciones con varias réplicas | Bloqueo en la base de datos (ShedLock) | Sin bloqueo, cada réplica publicaría el mismo evento. La deduplicación lo absorbería, pero la traza y los correos se duplican |
| D8 | Pruebas de departamentos | PHPUnit con cobertura (Xdebug o PCOV) en Clover para SonarQube | Es la herramienta estándar en PHP y genera el formato que ya lee Sonar |
| D9 | Plantillas de notificaciones | Archivos `text/template` montados como volumen | Se cambian sin recompilar; la librería ya viene con Go |
| D10 | Keycloak | Al final, solo si queda tiempo | Es puntaje extra, cambia el auth y el Gateway, y es la fase más larga |

## Fases

| Fase | Alcance | Verificación | Estado |
|---|---|---|---|
| **F0** Plan | Este plan; rama sincronizada con `main` | Plan revisado por el usuario | ✅ |
| **F1** Departamentos completo | `PUT` y `DELETE`; arquitectura por capas para poder probar; PHPUnit; Dockerfile multi-stage; Jenkinsfile con Test, Sonar y Quality Gate (D8) | Cobertura ≥ 70 % en Sonar; pipeline en verde; E2E en verde | ✅ |
| **F2** Perfiles y vacaciones | `foto`, `redesSociales`, `codigoPostal` en perfiles; `PUT /vacaciones/{id}` con las mismas validaciones | Pruebas en verde; cobertura ≥ 70 %; Swagger actualizado | ✅ |
| **F3** Plantillas configurables | Notificaciones lee las plantillas de archivos (D9) | Cambiar una plantilla sin recompilar y ver el correo en Mailhog | ⬜ |
| **F4** Cero secretos | Quitar los 15 valores por defecto y los `guest/guest` del código (D3); `setup-secrets.sh` genera valores aleatorios; `.env.example` con marcadores | `grep` sin valores reales; `docker compose up` sin `.env` falla y dice qué falta | ⬜ |
| **F5** Compose Secrets | JWT y contraseñas de base de datos y broker como archivos en `secrets/` (D4); cada servicio lee `*_FILE` | `docker inspect` no muestra contraseñas; pruebas y E2E en verde | ⬜ |
| **F6** Réplicas y balanceador | Quitar `container_name` de los servicios de aplicación; Nginx delante del Gateway (D6); 2 réplicas de los servicios sin estado; ShedLock en vacaciones (D7) | Las peticiones se reparten (logs por instancia); un solo evento por inicio de vacaciones | ⬜ |
| **F7** Resiliencia | Timeout y manejo de error en el proxy del Gateway; prueba de caída de una instancia | Con una réplica apagada el sistema sigue respondiendo; evidencia en `pruebas.md` | ⬜ |
| **F8** Redes por capa | Redes separadas por capa como en el diagrama del PDF (frontend, backend, mensajería, datos, observabilidad, CI/CD) | Una base de datos no es alcanzable desde la red del frontend; todo sigue funcionando | ⬜ |
| **F9** Composición (extra) | `GET /empleados/{id}/completo`, con respuesta parcial si perfiles no responde | Pruebas del Gateway; respuesta parcial con perfiles apagado | ⬜ |
| **F10** Keycloak (extra, D10) | IdP OIDC; el Gateway valida por JWKS; auth queda sincronizando cuentas | Login por Keycloak; E2E en verde | ⬜ |
| **F11** Documentación | Guías de Jenkins, SonarQube, Grafana, RabbitMQ y Keycloak; README; sustentación del proyecto | Checklist de la skill `documentacion-retos` | ⬜ |
| **F12** Entrega | Clonar en limpio → `setup-secrets.sh` → `docker compose up --build`; escaneo de secretos con gitleaks; `.tar.gz` | Todo arranca desde cero; gitleaks sin hallazgos válidos | ⬜ |

## Fuera de alcance (anotado para después)

- Patrón outbox para no perder eventos si el broker está caído al publicar (README, "Limitaciones conocidas").
- Duplicado de email o `numeroEmpleado` responde 400; 409 sería más preciso, pero el PDF no lo exige.
- Un `USER` puede leer el perfil de otro empleado (`api-gateway/autorizacion.js:88`); el PDF solo exige 403 al modificar.

- SonarQube marca 1 bug en vacaciones, anterior al proyecto final (2026-09-28): `SaludController.java:72` captura `InterruptedException` sin volver a interrumpir el hilo (regla `java:S2142`).

## Aprendizajes por fase

- **F0**: los PDF de los Retos 9–11 no están; el plan se ancla en los criterios del proyecto final. La rama `lyc4nthrope` estaba 3 commits detrás de `main` y se adelantó sin conflictos.
- **F1**: departamentos quedó en capas: `DepartamentoService` (reglas), `Aplicacion` (método + ruta + cuerpo → `Respuesta`, sin globales) y `DepartamentoRepository` detrás de la interfaz `RepositorioDepartamentos`; `public/index.php` solo hace E/S, métricas y trazas. Nuevos `PUT` (200/400/404) y `DELETE` (204/404). PHPUnit 11.5.57: **91 pruebas, 300 aserciones, 98,3 % de líneas de `src/`** (pcov). Las pruebas corren en la etapa `test` del Dockerfile con el mismo PHP 8.2 de producción; la imagen final no lleva `vendor/` ni pruebas. Mutaciones: 14 de 14 detectadas (una era equivalente: el chequeo de existencia antes del `UPDATE` sobraba y se quitó). La revisión independiente encontró que un `nombre` de más de 100 caracteres daba 500 en MySQL estricto (SQLite no limita `VARCHAR`): ahora se valida el largo de cada columna y el tipo (400); también se decodifica el id de la URL y la lista desempata por `id`. E2E: 65 de 65 escenarios (4 nuevos). Sorpresas: `rowCount()` de MySQL da 0 en un `UPDATE` que no cambia valores, por eso `actualizar` vuelve a leer la fila; con BuildKit, una capa de pruebas en caché no imprime la salida de PHPUnit (el resultado sigue en `reports/estado`). Jenkins: build #23 en verde (Test, SonarQube, Quality Gate, Package y Publish); SonarQube registra 91 pruebas, 98,3 % de cobertura, 0 bugs, 0 vulnerabilidades y 34 code smells. El primer intento (#22) falló porque el disco estaba al 100 %: con menos de 1 GB libre, Elasticsearch deja los índices de SonarQube en solo lectura; se liberaron 4,3 GB de caché de build.
- **F2**: perfiles acepta `foto` (URL `http`/`https`), `redesSociales` (`linkedin`, `github`, `twitter`; el `PUT` reemplaza el objeto completo y `""` queda `null`) y `codigoPostal` (máx. 10). Como `init.sql` solo corre con el volumen vacío, el servicio agrega las columnas al arrancar con `ADD COLUMN IF NOT EXISTS` y un candado de Postgres (`pg_advisory_xact_lock`), seguro con varias réplicas; se comprobó sobre el volumen existente (2338 perfiles conservados) y en un segundo arranque. Vacaciones: `PUT /vacaciones/{id}` solo para un período `PROGRAMADA` que no inició (si no, 409); mismas validaciones que al programar, con el solapamiento excluyendo al propio período; `UPDATE … WHERE estado = 'PROGRAMADA'` contra carreras con el scheduler; se publica de nuevo `vacaciones.programadas`, porque el catálogo no tiene un evento de actualización. Números: perfiles 82 pruebas, 100 % de la lógica de negocio; vacaciones 151 unitarias + 62 de integración, 100 % de líneas y ramas en las unitarias; E2E 71 de 71 (6 escenarios nuevos). Mutaciones: 16 de 16 detectadas. La revisión independiente encontró que la validación de URL dejaba pasar `https://linkedin.com@evil.com`, la barra invertida y los caracteres de control (`urlsplit` borra tab y saltos de línea sin avisar); se corrigió con 9 pruebas nuevas. Jenkins: vacaciones #30 en verde; perfiles #92 en verde con 100 % en SonarQube. Perfiles fallaba desde el build #22: el Reto 8 excluyó `metricas.py`, `trazas.py` y `logs.py` de la cobertura en `pyproject.toml` pero no en el Jenkinsfile, y SonarQube medía 63 % (< 70 %).
