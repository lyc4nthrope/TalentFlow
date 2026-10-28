# Reto 7 — Paso a paso para la sustentación

Orden: el de los criterios de evaluación del PDF (1 a 5). Cada paso dice **qué ejecutar**, **qué mostrar** y **qué se espera**. Todo desde la raíz del repo. Tiempo total: unos 20 minutos (la demostración en vivo del fallo suma unos 5).

Usuario y contraseña: Jenkins `admin` / `talentflow_ci_2026`, SonarQube `admin` / `talentflow_sonar_2026`.

## 0. Antes de entrar (preparación, no se muestra)

```bash
git switch lyc4nthrope && git pull
CI_BRANCH=lyc4nthrope docker compose --profile ci up --build -d   # o sin CI_BRANCH cuando el reto esté en main
df -h /                                                           # dejar más de 5 GB libres (D20)

# Ayudante para las llamadas POST a Jenkins (crumb + cookie); se usa en la demostración.
jpost() {
  JAR=$(mktemp)
  CRUMB=$(curl -s -u admin:talentflow_ci_2026 -c "$JAR" \
    'http://localhost:9090/crumbIssuer/api/xml?xpath=concat(//crumbRequestField,":",//crumb)')
  curl -s -o /dev/null -w "%{http_code}\n" -u admin:talentflow_ci_2026 -b "$JAR" -H "$CRUMB" -X POST "http://localhost:9090/$1"
}
```

Comprobar: `docker logs sonar-init` termina en `[sonar-init] listo`; en `http://localhost:9090` están los 9 trabajos en azul; abiertas en pestañas las páginas de la tabla de abajo.

## 1. Configuración de Jenkins (1.0 pt)

| # | Ejecutar / mostrar | Se espera |
|---|---|---|
| 1.1 | `docker compose --profile ci ps -a` | `jenkins`, `sonarqube`, `sonar-db`, `docker-registry` en `Up`; `sonar-init` terminó (`Exited (0)`) |
| 1.2 | [`jenkins/Dockerfile`](../../jenkins/Dockerfile) | `jenkins-plugin-cli` con `junit`, `coverage`, `cucumber-reports`, `htmlpublisher`, `sonar`, `configuration-as-code`, `job-dsl`; la imagen trae Node, Go, Python, Java/Maven, PHP y SonarScanner |
| 1.3 | Servicio `jenkins` de [`docker-compose.yml`](../../docker-compose.yml) | Socket `/var/run/docker.sock` montado (Docker-outside-of-Docker), `user: root` (D17), `runSetupWizard=false`, credenciales por variables |
| 1.4 | Abrir `http://localhost:9090` sin sesión | Pide iniciar sesión: los anónimos no ven nada (D18). No hay asistente ni contraseña inicial |
| 1.5 | `verificacion-docker` → **Build Now** → consola | Verde en ~1 s: `whoami` = `root`, `docker --version`, `docker compose version` y el `docker ps` **del host** |
| 1.6 | [`jenkins/casc.yaml`](../../jenkins/casc.yaml) | Administrador, credencial `sonar-token`, servidor SonarQube y los 9 trabajos: nada se configuró a mano |

## 2. Jenkinsfile, Build, Test y reportes (1.0 pt)

| # | Ejecutar / mostrar | Se espera |
|---|---|---|
| 2.1 | [`microservicios/gestion-empleados/Jenkinsfile`](../../microservicios/gestion-empleados/Jenkinsfile) | Declarativo: Checkout → Build → Test → SonarQube → Quality Gate → Package → Publish; `junit` y `recordCoverage` en `post { always }` de Test; `npm test` es el comando del Reto 6, sin `\|\| true` |
| 2.2 | Trabajo `gestion-empleados` → **Build Now** | Verde en ~17 s; la vista de etapas muestra las 7 |
| 2.3 | Build → **Tests** | 86 pruebas, 0 fallidas (captura `02-test-result.png`) |
| 2.4 | Build → **Coverage** | 100 % de líneas, 476/476 (captura `03-coverage.png`) |
| 2.5 | Página del trabajo | Gráficas de tendencia de pruebas y cobertura de los builds anteriores |
| 2.6 | `microservicios/auth-service/Jenkinsfile` (Go) y `gestion-vacaciones` (Java) | El mismo esqueleto con otro lenguaje y otro formato de cobertura (Cobertura XML, JaCoCo) |

## 3. Calidad con SonarQube (1.0 pt)

| # | Ejecutar / mostrar | Se espera |
|---|---|---|
| 3.1 | [`sonar/init.sh`](../../sonar/init.sh) y `docker logs sonar-init` | Contraseña de admin, usuario `jenkins` con token, gate `TalentFlow` (`coverage` < 70 → ERROR, por defecto) y el webhook a Jenkins, todo por script |
| 3.2 | `http://localhost:9000/dashboard?id=gestion-empleados` | Cobertura 100 %, duplicación, bugs, vulnerabilidades y *code smells* |
| 3.3 | SonarQube → Quality Gates → TalentFlow | Una condición: Coverage < 70 % |
| 3.4 | Consola del build de 2.2 | `Quality gate is 'OK'` en la etapa Quality Gate (el webhook respondió) |
| 3.5 | `gestion-empleados` #15 (fallo simulado ya ejecutado) y captura `06-quality-gate.png` | Test en verde (86 pruebas, 100 %), Quality Gate en rojo: `Pipeline aborted due to quality gate failure`; SonarQube 69.8 % por un módulo sin pruebas |

Si preguntan por qué la cobertura cayó con un archivo nuevo y no quitando pruebas: [las dos capas](README.md#4-preguntas-probables-en-la-sustentación).

## 4. Package, E2E y reporte BDD (1.0 pt)

| # | Ejecutar / mostrar | Se espera |
|---|---|---|
| 4.1 | `pipeline-orquestador-e2e` → **Build Now** (dejar corriendo) | ~2 min; las etapas Build & Package → Publish → Deploy → E2E Tests en verde |
| 4.2 | [`Jenkinsfile`](../../Jenkinsfile) mientras corre | 7 imágenes en paralelo con `:${BUILD_NUMBER}` y `:latest`; Deploy con proyecto `talentflow-ci-<build>` y [`docker-compose.ci-e2e.yml`](../../docker-compose.ci-e2e.yml); `post` con `junit`, `cucumber`, `publishHTML`, `archiveArtifacts` y `down -v` |
| 4.3 | `docker ps --format '{{.Names}}' \| grep talentflow-ci` durante el build | Los contenedores del build, con nombres propios (no chocan con el sistema de desarrollo) |
| 4.4 | Build → **Cucumber reports** | 8 características y 61 escenarios en español, todos en verde; entrar a una característica y ver los pasos (captura `04-reporte-bdd.png`) |
| 4.5 | Build → **Reporte BDD** y **Build Artifacts** → `docker-logs.txt` | El HTML de Cucumber y los logs de todos los contenedores |
| 4.6 | Al terminar: `docker ps -a --format '{{.Names}}' \| grep -c talentflow-ci` y `docker volume ls -q \| grep -c talentflow-ci` | `0` y `0`: la limpieza borró todo lo del build |
| 4.7 | `curl -s localhost:5000/v2/gestion-empleados/tags/list` | Las etiquetas por número de build y `latest` |

## 5. Reproducibilidad y documentación (1.0 pt)

| # | Ejecutar / mostrar | Se espera |
|---|---|---|
| 5.1 | [`pruebas.md` §1](pruebas.md#1-cada-pipeline-pasa-tres-veces-seguidas-p51) | 27 builds `SUCCESS` (9 trabajos × 3) con duraciones casi iguales |
| 5.2 | Los cuatro fallos ya ejecutados en Jenkins | `api-gateway` #14 rojo en **Test** → Tests muestra `aplana los componentes del /health del servicio` con el diff; `gestion-empleados` #15 rojo en **Quality Gate**; `gestion-perfiles` #16 rojo en **Package** (`"/aplicacion": not found`); `pipeline-orquestador-e2e` #15 rojo en **E2E Tests** → Cucumber reports → Failures con el escenario y el paso |
| 5.3 | [Sección del README raíz](../../README.md#integración-continua-reto-7) | Qué es CI, acceso, pasos a-d, etapas, tabla de reportes, verde/amarillo/rojo, capturas |
| 5.4 | **Romper una prueba en vivo** (abajo) | `api-gateway` se detiene en Test y Tests nombra la prueba |

Demostración en vivo y cómo restaurar. Jenkins solo ve lo que está en GitHub, así que se usa una rama temporal; los demás trabajos se deshabilitan para que el push no dispare los nueve (el *polling* no filtra por carpeta):

```bash
git switch -c reto7-demo && git push -u origin reto7-demo
CI_BRANCH=reto7-demo docker compose --profile ci up -d jenkins      # Jenkins vuelve a crear los trabajos con esa rama (~1 min)
for j in pipeline-orquestador-e2e gestion-empleados auth-service gestion-departamentos \
         gestion-vacaciones gestion-perfiles notificaciones; do jpost "job/$j/disable"; done   # 302 = hecho

sed -i "20s/status: 'UP'/status: 'DOWN'/" microservicios/api-gateway/tests/salud.test.js
git commit -am "test: prueba rota a propósito (demostración)" && git push
# En ~1 min el polling dispara api-gateway (o Build Now). Mostrar: Test en rojo, SonarQube,
# Quality Gate, Package y Publish saltados; Build → Tests → la prueba y su mensaje.

# Restaurar
git switch lyc4nthrope
git push origin --delete reto7-demo && git branch -D reto7-demo
CI_BRANCH=lyc4nthrope docker compose --profile ci up -d jenkins     # trabajos de nuevo en lyc4nthrope y habilitados
```

Para mostrar en vivo el escenario BDD en lugar de la prueba unitaria, el cambio es `sed -i "45s/código 403/código 200/" e2e-tests/features/seguridad/autorizacion.feature` y el trabajo habilitado es `pipeline-orquestador-e2e` (~2 min).

## Después

```bash
docker compose --profile ci stop registry sonarqube sonar-db jenkins   # apaga la CI; el sistema y los datos quedan
```
