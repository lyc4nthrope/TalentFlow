# Reto 7 — Evidencia

Cada sección dice qué se comprobó, cómo y qué salió. Las salidas son reales (recortadas), del 2026-10-28, con Jenkins 2.580.1 y SonarQube 9.9.8 levantados con `CI_BRANCH=lyc4nthrope docker compose --profile ci up -d`.

## 1. Cada pipeline pasa tres veces seguidas (P5.1)

Comando: un script que, por cada trabajo, lanza el build por REST (`POST /job/<trabajo>/build` con *crumb*), espera a que termine y anota resultado y duración; los tres builds de un trabajo van uno detrás del otro, sin cambios en el repositorio entre ellos (commit `b972355`), así que el *polling* SCM no metió builds intermedios.

Resultado (07:11 → 07:28, 27 builds):

| Trabajo | Builds | Resultado | Duración (s) |
|---|---|---|---|
| `verificacion-docker` | #6, #7, #8 | SUCCESS ×3 | 1 / 1 / 1 |
| `api-gateway` | #11, #12, #13 | SUCCESS ×3 | 14 / 14 / 13 |
| `auth-service` | #11, #12, #13 | SUCCESS ×3 | 18 / 17 / 17 |
| `gestion-departamentos` | #11, #12, #13 | SUCCESS ×3 | 3 / 2 / 2 |
| `gestion-empleados` | #12, #13, #14 | SUCCESS ×3 | 17 / 17 / 16 |
| `gestion-perfiles` | #13, #14, #15 | SUCCESS ×3 | 11 / 11 / 10 |
| `notificaciones` | #11, #12, #13 | SUCCESS ×3 | 19 / 15 / 16 |
| `gestion-vacaciones` | #11, #12, #13 | SUCCESS ×3 | 31 / 35 / 33 |
| `pipeline-orquestador-e2e` | #12, #13, #14 | SUCCESS ×3 | 113 / 115 / 119 |

Lo que demuestra: no hay builds intermitentes. Las duraciones varían pocos segundos y los resultados no dependen del orden: cada build parte de un workspace limpio (`npm ci`, entornos nuevos), Testcontainers crea un Postgres propio por ejecución y el orquestador despliega con un proyecto de Compose propio (`talentflow-ci-<build>`) que borra al terminar (después de cada build no queda ningún contenedor ni volumen `talentflow-ci-*`). No hubo nada que corregir.

## 2. El pipeline se detiene en la etapa correcta (P5.2)

Los cuatro fallos se hicieron en una rama temporal `reto7-fallos-simulados` (creada desde `lyc4nthrope` y borrada al final, local y en `origin`), con Jenkins recreado con `CI_BRANCH=reto7-fallos-simulados`. Para que el *polling* no disparara los ocho trabajos con cada push, se deshabilitaron y cada caso habilitó solo el suyo. Cada caso fue un commit seguido de su `git revert`; al terminar, la rama no tenía diferencias con `b972355`.

### 2.1 Una prueba unitaria falla → se detiene en Test

Cambio (`microservicios/api-gateway/tests/salud.test.js`):

```diff
-        assert.deepEqual(estado, { status: 'UP', app: 'UP', db: 'UP', broker: 'UP' });
+        assert.deepEqual(estado, { status: 'DOWN', app: 'UP', db: 'UP', broker: 'UP' });
```

Resultado: `api-gateway` #14 **FAILURE** en **Test**; la consola registra `Stage "SonarQube" / "Quality Gate" / "Package" / "Publish" skipped due to earlier failure(s)`, así que no se construyó imagen.

En Jenkins → #14 → **Tests** (`testReport`: `failCount` 1, `passCount` 141), la prueba fallida aparece con su nombre y mensaje (captura `05-build-fallido.png`):

```text
aplana los componentes del /health del servicio   (Regression)
Expected values to be strictly deep-equal:
+ actual - expected
  {  app: 'UP', broker: 'UP', db: 'UP',
+   status: 'UP'
-   status: 'DOWN'
  }
```

### 2.2 La cobertura baja del 70 % → se detiene en Quality Gate

Hay dos capas de cobertura y cumplen papeles distintos:

- **Test** exige el 100 % de la lógica de negocio (umbral del Reto 6). Una prueba que se quita o una rama nueva sin probar hacen fallar **Test**, antes que SonarQube: en esos casos el pipeline se detiene antes y de forma más estricta que el gate del 70 %.
- **Quality Gate** (SonarQube, `coverage` < 70 → ERROR) mide **todos** los archivos de `sonar.sources` que no están en `sonar.coverage.exclusions`, también los que ninguna prueba carga. El umbral de Node (`--test-coverage-include`) solo lista archivos que alguna prueba importa (hallazgo del Reto 6): un módulo de negocio nuevo sin pruebas pasa Test. SonarQube lo atrapa.

Para mostrar el gate se usó ese hueco: se agregó `microservicios/gestion-empleados/src/dominio/liquidacion.js` (475 líneas: liquidación de un retiro con cesantías, prima, vacaciones, indemnización, retención y pagos), que ninguna prueba importa y que no está en `sonar.coverage.exclusions`.

```diff
+ microservicios/gestion-empleados/src/dominio/liquidacion.js   (archivo nuevo, 475 líneas, sin pruebas)
```

Resultado: `gestion-empleados` #15 **FAILURE** en **Quality Gate**. Test pasó (`ℹ pass 86`, `ℹ fail 0`, `all files | 100.00 | 100.00 | 100.00`) y luego:

```text
SonarQube task 'AaGCfpcIQSbiFgczWY50' completed. Quality gate is 'ERROR'
ERROR: Pipeline aborted due to quality gate failure: ERROR
Stage "Package" skipped due to earlier failure(s)
Stage "Publish" skipped due to earlier failure(s)
```

SonarQube (API `measures` y `qualitygates/project_status`): `lines_to_cover` 729 (476 + 253 del archivo nuevo), `uncovered_lines` 253, `coverage` **69.8**, gate **ERROR** con la condición `coverage LT 70, actualValue 69.8` (captura `06-quality-gate.png`).

### 2.3 El Dockerfile tiene un error → se detiene en Package

Cambio (`microservicios/gestion-perfiles/Dockerfile`):

```diff
-COPY app ./app
+COPY aplicacion ./app
```

Resultado: `gestion-perfiles` #16 **FAILURE** en **Package**. Checkout, Build, Test, SonarQube y Quality Gate (`Quality gate is 'OK'`) pasaron; luego:

```text
ERROR: failed to build: failed to solve: failed to compute cache key: failed to calculate checksum of ref ...: "/aplicacion": not found
Stage "Publish" skipped due to earlier failure(s)
```

Aquí no hay prueba ni escenario que nombrar: el reporte es la etapa **Package** en rojo en la vista de etapas, y el log de esa etapa muestra la instrucción que falló.

### 2.4 Un escenario BDD falla → se detiene en E2E Tests

Cambio (`e2e-tests/features/seguridad/autorizacion.feature`, línea 45):

```diff
     Cuando intento actualizar la ciudad del perfil del otro empleado a "Cali"
-    Entonces la respuesta debe tener código 403
+    Entonces la respuesta debe tener código 200
```

Resultado: `pipeline-orquestador-e2e` #15 **FAILURE** en **E2E Tests** (Checkout, Build & Package, Publish y Deploy en verde; `61 scenarios (60 passed, 1 failed)`). Los reportes se publicaron igual y la limpieza del post dejó cero contenedores y volúmenes `talentflow-ci-*`.

- **Tests** (`testReport`: `failCount` 1, `passCount` 60): caso `Un empleado con rol USER solo puede editar su propio perfil`, clase `Permisos según el rol del usuario`, mensaje `Se esperaba código 200 y se obtuvo 403 en PUT /perfiles/E-mvs3jgva-48e01d. Cuerpo: {"error":"FORBIDDEN","message":"No tiene permisos para realizar esta acción.",...}`.
- **Cucumber reports → Failures**: la característica, el escenario y el paso `Entonces la respuesta debe tener código 200` en rojo con el mismo error (captura `07-escenario-bdd-fallido.png`); la vista de etapas marca **E2E Tests** como *failed* (captura `08-etapa-fallida-e2e.png`, tomada con el trabajo todavía deshabilitado para la prueba).

### 2.5 Cierre

Jenkins se recreó con `CI_BRANCH=lyc4nthrope` (JCasC vuelve a generar los trabajos habilitados y con la rama `*/lyc4nthrope`), se borró `reto7-fallos-simulados` en local y en `origin`, y `lyc4nthrope` siguió en `b972355` sin cambios.

## 3. Capturas (P5.3)

Tomadas con Brave sin interfaz (`puppeteer-core`, 1600×1000) después de iniciar sesión en Jenkins.

| Archivo | Qué muestra |
|---|---|
| [01-pipeline-exitoso-etapas.png](capturas/01-pipeline-exitoso-etapas.png) | Vista de etapas de `pipeline-orquestador-e2e`: #9–#14 en verde en todas las etapas |
| [02-test-result.png](capturas/02-test-result.png) | Test Result de `gestion-empleados` #14: 86 pruebas, 0 fallidas |
| [03-coverage.png](capturas/03-coverage.png) | Coverage de `gestion-empleados` #14: 100 % de líneas (476/476) y tendencia |
| [04-reporte-bdd.png](capturas/04-reporte-bdd.png) | Cucumber reports de `pipeline-orquestador-e2e` #14: 8 características y 61 escenarios en español, todos en verde |
| [05-build-fallido.png](capturas/05-build-fallido.png) | `api-gateway` #14: la prueba `aplana los componentes del /health del servicio` con su mensaje de error |
| [06-quality-gate.png](capturas/06-quality-gate.png) | SonarQube `gestion-empleados`: gate **Failed**, cobertura 69.8 % < 70 % |
| [07-escenario-bdd-fallido.png](capturas/07-escenario-bdd-fallido.png) | Cucumber reports #15 → Failures: escenario y paso fallidos con el mensaje |
| [08-etapa-fallida-e2e.png](capturas/08-etapa-fallida-e2e.png) | Vista de etapas del orquestador con #15 detenido en E2E Tests |
