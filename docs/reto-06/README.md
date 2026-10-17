# Reto 6 — Manual de las pruebas

Cómo ejecutar cada suite, dónde quedan los reportes y dónde se demuestra cada criterio de `reto6.pdf`. Todo se ejecuta desde la raíz del repositorio. URL base de la suite: **`http://localhost:8080`** · administrador semilla: **`admin@empresa.com` / `Admin123*`** · correos (tokens de activación y recuperación): **`http://localhost:8025`**.

| Archivo | Para qué |
|---|---|
| **Este README** | Manual: criterios y entregables → dónde se demuestran, inicio rápido, cómo abrir los reportes, preguntas para la sustentación, problemas comunes |
| [`pruebas.md`](pruebas.md) | Evidencia: reglas rotas a propósito y qué prueba lo detectó, tres corridas consecutivas con tiempos, fallo simulado (consola, XML, JSON, HTML y código de salida) |
| [`PLAN.md`](PLAN.md) | Diagnóstico de partida, decisiones de diseño (D1-D14), fases y aprendizajes de cada una |
| [README raíz](../../README.md#automatización-de-pruebas-reto-6) | Documento de entrega: pirámide, BDD, prerrequisitos, ejecución paso a paso, **tabla de reportes**, escenarios, herramientas y decisiones técnicas |
| [`../../Makefile`](../../Makefile) · [`../../docker-compose.e2e.yml`](../../docker-compose.e2e.yml) | Comandos únicos y override de la suite (scheduler cada 5 s + contenedor `bdd-tests`) |
| [`../../e2e-tests/`](../../e2e-tests/) | Proyecto BDD: `features/` (Gherkin), `step_definitions/`, `support/` (World, polling, configuración), `tests/` (pruebas del soporte) |

## 0. Criterios, entregables y consideraciones → dónde se demuestran

### Criterios de evaluación

| # | Criterio | Dónde |
|---|---|---|
| 1 | Unitarias de dominio en ≥ 2 servicios de lenguajes distintos (vacaciones con los bordes del solapamiento, empleados), dobles sin Docker, ≥ 1 integración contra BD real, reportes estándar, cobertura ≥ 70 %, casos de error | [Pirámide](../../README.md#pirámide-de-pruebas) (681 unitarias en Node, Go, Python y Java; 59 de integración) · `ReglasVacacionesTest` (17 casos de solapamiento en ambos órdenes) · `empleados.service.test.js` · [Testcontainers](../../README.md#decisiones-técnicas) · [tabla de reportes](../../README.md#tabla-de-reportes) · lógica de negocio al 100 % en los seis servicios · [reglas rotas a propósito](pruebas.md#1-las-pruebas-fallan-cuando-deben-punto-1-paso-6) · `make pruebas` |
| 2 | Proyecto BDD independiente, contexto compartido, variables de entorno, humo, justificación del framework | `e2e-tests/` (paquete propio) · `support/world.js` · [variables](../../README.md#ejecución-paso-a-paso) · `features/sistema.feature` · [por qué Cucumber.js](../../README.md#qué-es-bdd-y-por-qué-se-eligió) |
| 3 | ≥ 4 escenarios de seguridad: sin token, token inválido, USER vs ADMIN, `Antecedentes` | `features/seguridad/autenticacion.feature` (9) y `autorizacion.feature` (17, con `Antecedentes`) |
| 4 | Onboarding: registro, cuenta por evento, activación con token, login, datos inválidos; polling sin `sleep` y justificado; independencia | `features/onboarding/registro.feature` (15) y `activacion.feature` (8) · `support/polling.js` · [calibración](../../README.md#decisiones-técnicas) · cada escenario pasa solo con `--name` |
| 5 | Vacaciones y offboarding: programación, rechazos, ciclo de suspensión y reactivación, desvinculación, login bloqueado, recuperación fallida, auditoría; precondiciones, control del tiempo, reutilización | `features/vacaciones/` (6) y `features/offboarding/retiro.feature` (5) · [control del tiempo, polling negativo, ausencia y reutilización](../../README.md#decisiones-técnicas) |
| 6 | Un comando por suite con sus reportes, código ≠ 0 al fallar, 3 corridas consistentes, tabla de reportes | [`Makefile`](../../Makefile) · [tabla de reportes](../../README.md#tabla-de-reportes) · [tres corridas y fallo simulado](pruebas.md#2-reproducibilidad-de-la-suite-bdd-punto-6-pasos-1-4-y-5) |

### Entregables

| Entregable | Dónde |
|---|---|
| Unitarias e integración en ≥ 2 microservicios, junto al código de cada uno | `microservicios/*/tests/`, `*_test.go`, `src/test/java/` · [pirámide](../../README.md#pirámide-de-pruebas) |
| Reportes de resultados y de cobertura en cada microservicio probado | `microservicios/<servicio>/reports/` · [tabla de reportes](../../README.md#tabla-de-reportes) |
| Proyecto BDD con `.feature` y *step definitions* | `e2e-tests/features/` (8 archivos, 61 escenarios) y `e2e-tests/step_definitions/` |
| JUnit XML, Cucumber JSON y HTML de la suite BDD | `e2e-tests/cucumber.js` → `e2e-tests/reports/{junit.xml,cucumber.json,cucumber.html}` |
| README con lo del Punto 6 y la tabla de reportes | [README raíz: Automatización de pruebas](../../README.md#automatización-de-pruebas-reto-6) |
| (Opcional) Pruebas en un contenedor con los reportes montados | Servicio `bdd-tests` en [`docker-compose.e2e.yml`](../../docker-compose.e2e.yml) (perfil `e2e`, `./e2e-tests/reports:/app/reports`) |

### Consideraciones

| Consideración | Cómo se cumple |
|---|---|
| Las unitarias no requieren Docker ni el sistema | Repositorios, publicadores y relojes son dobles; las `*IT` de vacaciones están fuera de Surefire (`mvn test` pasa sin Docker) y las de Postgres en Go se omiten sin su variable |
| Las pruebas de cada servicio viven con su código | Ninguna prueba de servicio está en `e2e-tests/`; `e2e-tests/tests/` solo prueba el soporte de la suite |
| `.feature` en español con `# language: es` | Primera línea de los 8 archivos |
| URLs y credenciales por variables de entorno | [`e2e-tests/support/config.js`](../../e2e-tests/support/config.js); valores validados al cargar |
| Escenarios independientes | World nuevo por escenario, datos propios en `Antecedentes`, comprobado con `--name` (F7 a F10 en [`PLAN.md`](PLAN.md)) |
| Polling con reintentos, no `sleep` fijo | `eventualmente` y `nuncaDurante` en `e2e-tests/support/polling.js`; también la espera del sistema (`scripts/esperar-sistema.js`) |
| Ids únicos o limpieza | `idUnico`/`emailUnico` (`E-<ms base 36>-<6 hex>`); no se limpia porque la baja es lógica |
| Código de salida ≠ 0 al fallar | Cada herramienta y cada umbral de cobertura; `make e2e` con un escenario roto termina en 2 ([evidencia](pruebas.md#23-fallo-simulado)) |
| `reports/`, `target/` y `coverage/` en `.gitignore` | [`.gitignore`](../../.gitignore) |

## 1. Inicio rápido

Prerrequisitos y versiones: [README raíz](../../README.md#prerrequisitos). Una sola vez: `npm ci` en la raíz (workspaces de Node) y el `.venv` de perfiles.

```bash
make pruebas       # unitarias + integración de cada servicio, en orden; ~30 s; para en la primera falla
make e2e           # override + espera "sistema":"OK" + suite BDD en el contenedor bdd-tests; ~80 s
make e2e-local     # lo mismo, con el Node de la máquina
make e2e-restaurar # vacaciones-service vuelve al cron por defecto (cada minuto)
```

Resultado esperado de `make e2e`: `61 scenarios (61 passed)`, `313 steps (313 passed)` y código 0. Una sola característica o escenario: `cd e2e-tests && npx cucumber-js --name "<parte del nombre>"` (con `--name`; las rutas por línea de comandos no reemplazan las `paths` de `cucumber.js`).

## 2. Cómo abrir los reportes

Las rutas completas de cada suite están en la [tabla de reportes](../../README.md#tabla-de-reportes) (el contrato con el Reto 7).

| Reporte | Cómo abrirlo |
|---|---|
| Escenarios BDD (HTML) | `xdg-open e2e-tests/reports/cucumber.html` (o abrir el archivo en el navegador): características, escenarios y pasos con su duración; los fallidos en rojo con el mensaje |
| Conteo BDD (JUnit XML) | `grep -o '<testsuite [^>]*>' e2e-tests/reports/junit.xml` → `tests="61" failures="0"` |
| Cobertura de vacaciones (JaCoCo) | `xdg-open microservicios/gestion-vacaciones/reports/coverage/index.html`: por paquete, clase y línea; integración en `reports/coverage-integracion/index.html` |
| Cobertura de Node (LCOV) | El resumen por archivo (líneas, ramas y funciones) sale en la consola de `npm test`; `lcov.info` es para SonarQube |
| Cobertura de Go | `go tool cover -html=reports/coverage-negocio.out` en `microservicios/auth-service` o `microservicios/notificaciones` |
| Cobertura de Python | `make test` imprime las líneas faltantes (`term-missing`); el XML es para Jenkins y SonarQube |

## 3. Preguntas probables en la sustentación

| Pregunta | Respuesta corta | Dónde mostrarlo |
|---|---|---|
| ¿Cómo se reparten las pruebas en la pirámide? | 681 unitarias en seis servicios y el soporte de la suite, 59 de integración con PostgreSQL real (17 más a pedido en Go) y 61 escenarios BDD. Cada regla se prueba abajo; arriba solo los flujos entre servicios | [Pirámide](../../README.md#pirámide-de-pruebas) |
| ¿Por qué 100 % de la lógica de negocio y no 100 % del código? | Cubrir arranque, configuración y adaptadores sube el número sin agregar seguridad; el 100 % sobre dominio y casos de uso garantiza que cada regla tiene una prueba. Cada exclusión está configurada en la herramienta y listada | [Qué no se prueba](../../README.md#decisiones-técnicas) |
| ¿Cómo saben que las pruebas comprueban algo? | Se rompieron reglas a propósito. Invertir el vencimiento del token encontró una prueba que seguía pasando: miraba el instante exacto del vencimiento, donde la regla correcta y la invertida coinciden. Se agregó `TestTokenVencidoHaceRatoSeRechaza` | [`pruebas.md` §1.2](pruebas.md#12-auth-service-go-invertir-el-vencimiento-del-token) |
| Quitaron la condición de "retirado no se reactiva" y nada falló, ¿no es un hueco? | No: es defensa en profundidad. El repositorio solo reactiva una cuenta `SUSPENDIDA_TEMPORAL` por esas mismas vacaciones; sin una capa, el comportamiento sigue siendo el correcto, y las pruebas verifican el comportamiento, no la línea | [`pruebas.md` §1.3](pruebas.md#13-auth-service-go-una-regla-con-defensa-doble) |
| ¿Por qué Testcontainers y no H2? | Fidelidad: la consulta de solapamiento, el `EXCLUDE` y los bloqueos `FOR UPDATE` solo se comportan así en PostgreSQL. El costo (Docker, ~10 s) se paga en `verify`, no en `test` | `JdbcRepositorioVacacionesIT` |
| ¿Cómo prueban que la recuperación "falla" si responde 202? | Con una prueba de ausencia y su control positivo: antes del retiro el token sí llega; después, la respuesta es igual a la de un email inexistente, durante al menos 3 s no llega ningún token y el token anterior ya no sirve | `retiro.feature` · [README](../../README.md#decisiones-técnicas) |
| ¿Cómo hacen que unas vacaciones empiecen y terminen en la prueba? | Fechas con hora a pocos segundos y el scheduler cada 5 s solo en el override. No hay endpoint de disparo manual (sería una puerta nueva en producción). Los pasos esperan el estado, no un tiempo: con el cron de un minuto pasa igual con `E2E_SCHEDULER_SEGUNDOS=60` | `suspension.feature` · [`docker-compose.e2e.yml`](../../docker-compose.e2e.yml) |
| ¿Cómo distinguen "todavía no" de "nunca" en "eventualmente no puede iniciar sesión"? | Precondición confirmada (el login funciona) y solo cuenta un 401 con el período `EN_CURSO`; al agotarse, el mensaje dice si el scheduler no corrió o si auth no suspendió | `support/ciclo.js` |
| ¿Por qué el token sale de Mailhog y no de la API? | `GET /notificaciones` lo devuelve enmascarado (`****0130`) por seguridad (Reto 5): un ADMIN no debe ver tokens ajenos. El correo es el canal real del empleado; antes se confirma por el Gateway que la notificación existe y solo se acepta el correo con el mismo sufijo. Todo es HTTP, sin leer logs de Docker | `support/empleados.js` |
| ¿Por qué 15 intentos de 1 s? | Medido: perfil, notificación y correo aparecen en 5–26 ms. El tope de 14 s es unas 500 veces el peor caso; con 2 s casi toda espera pagaba un intervalo completo | [PLAN.md D5](PLAN.md) |
| ¿Qué pasa si un escenario deja datos? | Nada: ids y emails únicos por escenario, sin depender del orden ni de una base vacía. Siete corridas seguidas en verde sobre la misma base | [`pruebas.md` §2.2](pruebas.md#22-tres-ejecuciones-consecutivas) |

## 4. Problemas comunes

| Síntoma | Causa y solución |
|---|---|
| El ciclo de vacaciones falla con "el período nunca empezó: sigue PROGRAMADA … ¿corre el scheduler?" | vacaciones-service corre con el cron por defecto (cada minuto) y la suite espera 5 s: no se aplicó el override. Usar `make e2e` (lo aplica), o `E2E_SCHEDULER_SEGUNDOS=60` con el Compose normal |
| Después de la suite el scheduler sigue cada 5 s | Es el override: `make e2e-restaurar` lo devuelve al cron por defecto sin tocar datos |
| `make e2e` termina con "El sistema no quedó OK … Último estado: …" | Algún servicio no está sano; el mensaje dice cuál. `docker compose ps` y los logs de ese servicio |
| `make pruebas` falla en vacaciones con un error de Docker o de Testcontainers | `mvn verify` necesita Docker para las `*IT`. Solo unitarias, sin Docker: `cd microservicios/gestion-vacaciones && mvn test` (sin reporte de cobertura) |
| notificaciones: "no such tool covdata" | Se corrió `go test -coverprofile` a mano con go1.25.0. Usar `make test`, que fija `GOTOOLCHAIN ?= go1.25.14`; con `GOTOOLCHAIN=local` y un Go antiguo, instalar el toolchain que pide el `go.mod` |
| auth-service: "go.mod requires go >= 1.27.1" | `GOTOOLCHAIN=local` impide la descarga del toolchain; dejarlo en `auto` (el valor por defecto) |
| perfiles: `.venv/bin/python: No such file or directory` | Falta el entorno: `cd microservicios/gestion-perfiles && python3 -m venv .venv && .venv/bin/pip install -r requirements-dev.txt` (o `make pruebas PYTHON=python3` con las dependencias instaladas) |
| Node: `Cannot find module` en empleados o gateway | Falta `npm ci` en la raíz (las dependencias vienen de los workspaces) |
| La suite falla al cargar con "BASE_URL no es una URL válida" | Variable mal escrita o sin `http://`; lo mismo para `MAILHOG_URL`, los enteros y `E2E_ZONA_HORARIA` |
| Los escenarios con token fallan desde fuera de Docker | Mailhog no es alcanzable: la UI debe estar publicada en `8025` (o ajustar `MAILHOG_URL`). Puertos usados en el host: `8080` (Gateway), `8025` (Mailhog), `15672` (broker) |
| `reports/` de la suite quedó de root | Se ejecutó `docker compose run bdd-tests` sin `E2E_UID`/`E2E_GID` y sin crear la carpeta antes; `make e2e` hace las dos cosas |
| Swagger UI en blanco | No afecta las pruebas: carga `swagger-ui` desde un CDN. La suite solo pide `/empleados/openapi.json` y `/vacaciones/docs`, que responden 200 sin internet (el CDN lo pide el navegador, no la prueba) |
