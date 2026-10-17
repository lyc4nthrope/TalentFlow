# Reto 6 — Paso a paso para la sustentación

Orden: el de los criterios de evaluación del PDF (1 a 6). Cada paso dice **qué ejecutar**, **qué mostrar** y **qué se espera**. Todo se ejecuta desde la raíz del repo. Tiempo total: unos 15 minutos.

## 0. Antes de entrar (preparación, no se muestra)

```bash
docker compose up --build -d                 # sistema completo
make sistema-e2e                             # aplica el override de pruebas (scheduler cada 5 s)
npm ci && (cd e2e-tests && npm ci)           # dependencias de Node, una sola vez
```

Comprobar: `curl -s localhost:8080/health` debe decir `"sistema":"OK"`. Dejar abiertas en el editor las rutas que se mencionan abajo.

## 1. Pruebas unitarias, integración y cobertura (1.5 pt)

| # | Ejecutar | Mostrar / se espera |
|---|---|---|
| 1.1 | `make pruebas` | Los 6 servicios en verde, cada uno con "100 %" de la lógica de negocio. Termina con "Todas las pruebas de los microservicios pasaron." (~30 s) |
| 1.2 | Abrir `microservicios/gestion-vacaciones/src/test/java/com/talentflow/vacaciones/dominio/ReglasVacacionesTest.java` | El solapamiento como **tabla de casos** (17 casos: idénticos, contenido, contiene, se tocan por un día…) |
| 1.3 | Abrir `microservicios/gestion-vacaciones/reports/coverage/index.html` en el navegador | JaCoCo: `dominio` y `aplicacion` al 100 % de líneas y ramas |
| 1.4 | Abrir `microservicios/gestion-vacaciones/src/test/java/com/talentflow/vacaciones/infraestructura/persistencia/` | Las 59 pruebas de integración contra PostgreSQL real (Testcontainers) |
| 1.5 | **Romper una regla en vivo** (ver abajo) | Fallan exactamente los casos de esa regla, con su nombre |

Paso 1.5, romper la regla de fechas y restaurarla:

```bash
sed -i '21s/!fechaFin.isAfter(fechaInicio)/!fechaFin.isBefore(fechaInicio)/' \
  microservicios/gestion-vacaciones/src/main/java/com/talentflow/vacaciones/dominio/ReglasVacaciones.java
(cd microservicios/gestion-vacaciones && mvn -q test)    # falla: "fin antes del inicio → 400 en fechaFin", etc.
git checkout -- microservicios/gestion-vacaciones/src/main   # restaurar
```

## 2. Proyecto BDD (0.5 pt)

| # | Mostrar | Qué señalar |
|---|---|---|
| 2.1 | Carpeta `e2e-tests/` | La estructura del PDF: `features/`, `step_definitions/`, `support/`, `package.json` |
| 2.2 | `e2e-tests/support/world.js` | Contexto nuevo por escenario: token actual, última respuesta, recursos creados |
| 2.3 | `e2e-tests/support/config.js` | Todo sale de variables de entorno (`BASE_URL`, credenciales, polling) |
| 2.4 | `cd e2e-tests && npx cucumber-js --name "El sistema responde correctamente"` | El escenario de humo del PDF, en verde |

## 3. Seguridad (0.5 pt)

| # | Ejecutar / mostrar | Se espera |
|---|---|---|
| 3.1 | `e2e-tests/features/seguridad/autenticacion.feature` | El escenario del PDF + 5 credenciales inválidas + rutas públicas |
| 3.2 | `e2e-tests/features/seguridad/autorizacion.feature` | `Antecedentes`; USER lee pero no modifica (403); ADMIN puede todo |
| 3.3 | `cd e2e-tests && npx cucumber-js --name "Acceso denegado"` | 6 escenarios en verde |

## 4. Onboarding (1.0 pt)

| # | Ejecutar / mostrar | Se espera |
|---|---|---|
| 4.1 | `e2e-tests/support/polling.js` | `eventualmente()`: 15 intentos de 1 s, sin `sleep` fijo |
| 4.2 | `e2e-tests/features/onboarding/registro.feature` y `activacion.feature` | Los pasos con "eventualmente": cuenta, bienvenida, perfil; activación y login; datos inválidos |
| 4.3 | `cd e2e-tests && npx cucumber-js --name "establece su contraseña con el token"` | El alta completa hasta el login, en verde |

## 5. Vacaciones y offboarding (1.0 pt)

| # | Ejecutar / mostrar | Se espera |
|---|---|---|
| 5.1 | `docker-compose.e2e.yml` | El único cambio para pruebas: el scheduler cada 5 s |
| 5.2 | `e2e-tests/features/vacaciones/suspension.feature` | El escenario del PDF y, en la descripción, cómo se controla el tiempo |
| 5.3 | `cd e2e-tests && npx cucumber-js --name "La cuenta se suspende al iniciar"` | En verde en unos 18 s: suspensión y reactivación con sus avisos |
| 5.4 | `e2e-tests/features/offboarding/retiro.feature` | Aviso, login bloqueado, recuperación que no sirve (prueba de ausencia), auditoría, retiro durante vacaciones |

## 6. Reportes y reproducibilidad (0.5 pt)

| # | Ejecutar / mostrar | Se espera |
|---|---|---|
| 6.1 | `make e2e` | Los 61 escenarios en verde, dentro de Docker (~1 min 25 s) |
| 6.2 | Abrir `e2e-tests/reports/cucumber.html` | Cada característica, escenario y paso, en lenguaje de negocio |
| 6.3 | Abrir `e2e-tests/reports/junit.xml` | `tests="61"` y `failures="0"` |
| 6.4 | **Fallo simulado** (ver abajo) | Mensaje claro, rojo en el HTML, registrado en el XML y código de salida distinto de cero |
| 6.5 | README, sección "Automatización de pruebas (Reto 6)" | La **tabla de reportes** (suite → comando → rutas) que leerá Jenkins |
| 6.6 | `docs/reto-06/pruebas.md` | Las 7 corridas seguidas en verde |

Paso 6.4, simular un fallo y restaurarlo:

```bash
sed -i '45s/código 403/código 200/' e2e-tests/features/seguridad/autorizacion.feature
(cd e2e-tests && npm test); echo "código de salida: $?"   # "Se esperaba código 200 y se obtuvo 403…", salida 1
grep -o 'failures="[0-9]*"' e2e-tests/reports/junit.xml | head -1   # failures="1"
git checkout -- e2e-tests/features                       # restaurar
```

## Después

`make e2e-restaurar` devuelve el scheduler a su ronda normal (cada minuto).
