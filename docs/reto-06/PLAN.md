# Reto 6 — Plan de implementación

Fuente de requisitos: `reto6.pdf` (Puntos 1 a 6, entregables, consideraciones y criterios de evaluación).
Punto de partida: `lyc4nthrope` con el Reto 5 completo (`378f3e7`, igual a `dev` y `main`).
Cada fase termina **verificada** (las suites pasan y generan sus reportes; contra el sistema desplegado cuando aplica), con su propio commit y un resumen en memoria, para poder retomar en cualquier punto.

## Diagnóstico de partida (2026-10-17)

El PDF parte de un ecosistema "sin una sola prueba automatizada". Este repo ya tiene unas 450 pruebas automatizadas (456 al cierre del Reto 5) con *test doubles*; lo que falta es sobre todo **reportes estándar, integración contra una BD real y toda la suite BDD**.

| Requisito | Estado actual | Brecha |
|---|---|---|
| Unitarias en ≥ 2 servicios de lenguajes distintos, con dobles y sin Docker (P1) | ✅ empleados (Node, 81), auth (Go, 116), notificaciones (Go, 33), vacaciones (Java, 63), perfiles (Python, 29), gateway (Node, 134) | — |
| Reglas mínimas: vacaciones (4 validaciones), empleados (unicidad y RETIRADO), un caso de error (P1) | ✅ cubiertas (`ReglasVacacionesTest`, `empleados.service.test.js`, `servicio_test.go`, `ciclo_test.go`) | El solapamiento **no está escrito como tabla de casos**; faltan los casos idéntico, el nuevo contiene al viejo y se tocan por un día con fecha sola |
| ≥ 1 prueba de integración contra la BD real (P1) | ⚠️ auth y notificaciones tienen pruebas de Postgres, pero se **omiten** si no hay una variable `*_TEST_DATABASE_URL` | Ninguna se ejecuta por defecto con un solo comando |
| Reportes de resultados y de cobertura en formato estándar y rutas documentadas (P1, P6) | ❌ solo los XML por defecto de Surefire en vacaciones; no hay cobertura en ningún servicio | Configurar los dos reportes en cada suite |
| Cobertura ≥ 70 % en un servicio (P1); **meta del equipo: 100 % de la lógica de negocio** (D14) | ⚠️ medida a mano: auth `dominio` 92.9 % y `aplicacion` 90.2 %; notificaciones `dominio` 93.2 % y `aplicacion` 96.4 %; empleados 95 %; vacaciones sin medir | No queda en un archivo de reporte; faltan las líneas de negocio que hoy no cubre ninguna prueba |
| Romper una regla a propósito y que una prueba lo señale (P1) | ❌ sin evidencia | Hacerlo y documentarlo |
| Proyecto BDD `e2e-tests/` con Gherkin en español, contexto compartido, variables de entorno y escenario de humo (P2) | ❌ no existe | Todo |
| Escenarios de seguridad, onboarding, vacaciones y offboarding (P3 a P5) | ❌ no hay `.feature`; `docs/reto-05/demo.sh` ya resuelve los mismos flujos en bash (polling, tokens, scheduler) | Llevar esa lógica a Gherkin y *step definitions* |
| BDD con JUnit XML + Cucumber JSON + HTML; 3 ejecuciones seguidas; fallo simulado; código de salida ≠ 0 (P6) | ❌ | Todo |
| `reports/` en `.gitignore` | ⚠️ ya están `coverage/` y `target/`, falta `reports/` | Agregarlo |
| README: pirámide, BDD, prerrequisitos, ejecución, escenarios, herramientas y tabla de reportes (P6) | ❌ | Todo |

## Decisiones de diseño

| # | Decisión | Elección | Por qué |
|---|---|---|---|
| D1 | Framework BDD | **Cucumber.js** (Node 24) con `fetch` nativo, sin librería HTTP extra | El repo ya usa Node (workspaces de npm) y no hace falta instalar otra herramienta. Cucumber.js trae los tres formatos de reporte. Los pasos de `demo.sh` se traducen casi uno a uno |
| D2 | Contexto compartido ("World") | Una instancia nueva por escenario (lo normal en Cucumber.js) con el token actual, la última respuesta y los ids creados; `BASE_URL` y las credenciales salen de variables de entorno | Un escenario nunca hereda el estado de otro, que es lo que exige el PDF |
| D3 | Aislamiento de datos | Cada escenario crea sus propios datos en `Antecedentes`, con ids y emails únicos (`E-<marca de tiempo>-<aleatorio>`); no se limpia nada después | La baja es lógica, así que no hay nada que borrar. Con ids únicos, la suite se puede ejecutar sobre una BD con datos y en cualquier orden |
| D4 | Token de activación y recuperación | Se obtiene de `GET /notificaciones` **a través del Gateway** (con el token del ADMIN), no de los logs de Docker ni de MailHog | Así la suite solo necesita `BASE_URL`, lo que permite ejecutarla en un contenedor o en Jenkins (Reto 7) sin acceso a Docker |
| D5 | Polling | Una función `eventualmente(condicion, { intentos, intervaloMs })`: 15 intentos cada 2 s (unos 30 s) por defecto, configurable por variable de entorno; si se agota el tiempo, falla con el último valor observado | Son los valores que sugiere el PDF. El mensaje dice qué se esperaba y qué se vio |
| D6 | Control del tiempo en vacaciones | Fechas **con hora** (inicio a pocos segundos, fin unos segundos después) y, solo para la suite, `VACACIONES_CRON` cada 5 s mediante un `docker-compose.e2e.yml`. El paso espera la **condición observable** (el login falla, el período queda `EN_CURSO`), nunca un tiempo fijo | El PDF acepta bajar el intervalo por variable de entorno. No se agrega un endpoint de disparo manual, porque sería una puerta nueva en producción. Esperar la condición y no "N segundos" evita acoplar la prueba a la frecuencia del scheduler |
| D7 | Polling sobre una condición negativa | Primero se confirma la precondición (el login funciona y el período está `EN_CURSO`) y después se espera el 401. Si se agota el tiempo, el mensaje distingue "el período nunca empezó" (el scheduler no corrió) de "el período empezó pero la cuenta sigue activa" (falla de auth) | Separa "todavía no se procesó" de "nunca se va a procesar", que es la pregunta del PDF |
| D8 | Prueba de integración | **Testcontainers** con PostgreSQL 16 en `vacaciones-service` (repositorio JPA y restricciones del schema), dentro de `mvn verify`. Las de Go siguen disponibles con su variable de entorno | Es la estrategia más fiel: el mismo motor que en producción. Vacaciones tiene la consulta de solapamiento, que es justo lo que una BD embebida podría aceptar y PostgreSQL rechazar. El costo es que necesita Docker, y por eso queda en `verify` y no en `test` |
| D9 | Convención de reportes | `<servicio>/reports/junit.xml` (resultados) y `<servicio>/reports/coverage.xml` (cobertura) en todos los servicios que se puedan configurar así; `e2e-tests/reports/{junit.xml,cucumber.json,cucumber.html}` | El PDF pide una sola convención para que el pipeline del Reto 7 apunte siempre al mismo lugar |
| D10 | Herramientas de reporte por lenguaje | Java: Surefire + JaCoCo. Node: los *reporters* nativos de `node:test` (`junit` y `lcov`, sin instalar nada). Go: `gotestsum` con la versión fijada, vía `go run`, y `-coverprofile`. Python: `pytest --junitxml` + `pytest-cov` | Se usan las herramientas estándar de cada lenguaje, que Jenkins y SonarQube ya saben leer |
| D11 | Qué no se prueba unitariamente | Se excluyen de la cobertura `main`/`cmd`, el arranque y el cableado de dependencias, la configuración y las clases sin lógica (DTO, records) | Inflan el porcentaje sin dar seguridad. Se justifica en el README (decisión pedida en el P1) |
| D12 | Que la cobertura no engañe | Cada prueba verifica un resultado observable (valor devuelto, evento publicado, estado); en el P1 se rompe una regla a propósito y se deja la evidencia | Responde a "la trampa del 70 %" del PDF |
| D13 | Suite BDD en Docker (opcional del P6) | Un servicio `bdd-tests` en un perfil `e2e` del Compose, con `e2e-tests/reports` montado como volumen | Prepara el Reto 7, porque Jenkins podrá ejecutarla sin instalar Node. Al estar en un perfil, `docker compose up` no la levanta |
| D14 | Meta de cobertura | **100 % de líneas y ramas en la lógica de negocio** (dominio y casos de uso) de los servicios con reportes; el resto queda fuera de la medición según D11, con cada exclusión listada en el README. La herramienta hace fallar el build si la lógica de negocio baja del 100 % (JaCoCo `check`; umbrales equivalentes en las demás) | El PDF exige 70 % en un servicio. Llevar la lógica de negocio al 100 % garantiza que **cada regla** tiene una prueba, sin inflar el número con código sin lógica. Si una línea solo se puede cubrir con una prueba artificial, se documenta como exclusión en lugar de forzarla |

## Fases

| Fase | Alcance | Verificación | Estado |
|---|---|---|---|
| **F0** Convenciones | D9; `reports/` en `.gitignore`; este plan | `git status` limpio después de generar un reporte | ✅ |
| **F1** Vacaciones: tabla de solapamiento y cobertura | Solapamiento como `@ParameterizedTest` con todos los casos borde; JaCoCo con D11; reportes en `reports/` (D9) | `mvn verify` genera `reports/junit.xml` y `reports/coverage.xml`; 100 % de la lógica de negocio (D14) y el build falla si baja | ⬜ |
| **F2** Integración contra PostgreSQL | D8: Testcontainers en vacaciones (persistencia, consulta de solapamiento y restricciones del schema) | `mvn verify` levanta y destruye el contenedor; las pruebas pasan | ⬜ |
| **F3** Reportes en Node | empleados y api-gateway con los *reporters* `junit` + `lcov` (D10) en `reports/`; pruebas que falten hasta el 100 % de la lógica de negocio (D14) | Un comando por suite; los archivos existen, el conteo coincide y la lógica de negocio está al 100 % | ⬜ |
| **F4** Reportes en Go y Python | auth, notificaciones (`gotestsum` + `coverage.out` → `coverage.xml`) y perfiles (`pytest-cov`); pruebas que falten hasta el 100 % de la lógica de negocio (D14) | Igual que F3 | ⬜ |
| **F5** Prueba de que las pruebas fallan | P1.6: invertir una comparación de fechas, ver qué prueba falla y con qué mensaje, y revertir | Evidencia en `docs/reto-06/pruebas.md` (el cambio no se versiona) | ⬜ |
| **F6** Proyecto BDD y escenario de humo | P2: `e2e-tests/` con D1, D2, D5 y variables de entorno; los tres reportes | `npm test` en `e2e-tests` pasa el humo y genera los tres archivos | ⬜ |
| **F7** Escenarios de seguridad | P3: sin token, token inválido o malformado, USER lee pero no escribe, ADMIN puede todo; `Antecedentes` | Al menos 4 escenarios en verde | ⬜ |
| **F8** Escenarios de onboarding | P4: credenciales y notificación por evento (polling), activación y login, datos inválidos; D3 y D4 | Cada escenario pasa también solo (`--name`) | ⬜ |
| **F9** Escenarios de vacaciones | P5: programación con notificación, fechas incoherentes, solapamiento, ciclo completo de suspensión y reactivación (D6, D7) | En verde, con la duración medida | ⬜ |
| **F10** Escenarios de offboarding | P5: notificación de desvinculación, login bloqueado, recuperación que falla, auditoría `?estado=RETIRADO`; reutilizar pasos | En verde | ⬜ |
| **F11** Reproducibilidad | P6: 3 ejecuciones seguidas sin fallos; fallo simulado (mensaje claro, rojo en el HTML, registrado en el XML, código de salida ≠ 0); D13 opcional | Evidencia en `pruebas.md` | ⬜ |
| **F12** Documentación | README: pirámide y reparto, BDD y por qué, prerrequisitos, ejecución paso a paso, escenarios, herramientas y **tabla de reportes**; manual `docs/reto-06/` | Cada entregable del PDF presente | ⬜ |
| **F13** Cierre | Pruebas completas, revisión, PR `lyc4nthrope` → `dev` → `main` | Ramas sincronizadas | ⬜ |

## Fuera de alcance (anotado para después)

- `departamentos-service` (PHP) no tiene pruebas unitarias. El PDF pide al menos dos servicios y ya hay seis cubiertos; su prueba actual es de caja negra contra el sistema desplegado.
- La revocación de un JWT antes de que venza (documentada como limitación en el Reto 5).

## Aprendizajes por fase

_(se completa al cerrar cada fase)_
