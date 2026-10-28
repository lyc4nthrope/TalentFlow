# Reto 7 — Manual paso a paso

Integración continua de TalentFlow: Jenkins en Docker con trabajos creados como código, un pipeline por servicio con pruebas, cobertura, SonarQube y Quality Gate, y un orquestador que publica las imágenes y corre la suite BDD del Reto 6 contra el sistema desplegado. Jenkins: `http://localhost:9090` (`admin` / `talentflow_ci_2026`) · SonarQube: `http://localhost:9000` (`admin` / `talentflow_sonar_2026`) · registro: `http://localhost:5000`.

La base del reto es el diseño de Valeria (`Vale292005`): Jenkins con sus plugins y herramientas, JCasC + Job DSL, SonarQube, el registro local, un pipeline raíz y uno por servicio (D1-D8 de [`PLAN.md`](PLAN.md)). Sobre ella se corrigió la portabilidad, se quitaron los `|| true`, se automatizó SonarQube y se aisló la E2E (D9-D20).

| Archivo | Para qué |
|---|---|
| **Este README** | Manual: criterios → dónde se demuestran, cómo ejecutar, cómo ver cada reporte, preguntas, problemas comunes |
| [Sección del README raíz](../../README.md#integración-continua-reto-7) | Lo que el PDF pide en el README: qué es CI, acceso, pasos a-d, etapas, tabla de reportes, interpretación, capturas, decisiones |
| [`PLAN.md`](PLAN.md) | Diagnóstico, decisiones D1-D20 y aprendizajes de cada fase |
| [`pruebas.md`](pruebas.md) | Evidencia: 3 ejecuciones seguidas de cada pipeline y los 4 fallos simulados |
| [`sustentacion.md`](sustentacion.md) | Paso a paso para la sustentación |
| [`capturas/`](capturas/) | Las 5 capturas del PDF y 3 más |
| [`../../jenkins/Dockerfile`](../../jenkins/Dockerfile) · [`../../jenkins/casc.yaml`](../../jenkins/casc.yaml) | Imagen de Jenkins (plugins y herramientas) · administrador, credencial de SonarQube y los 9 trabajos |
| [`../../sonar/init.sh`](../../sonar/init.sh) · [`../../sonar/sonarqube/`](../../sonar/sonarqube/) | Configuración de SonarQube como código (D15, D19) · imagen con los límites de disco (D20) |
| [`../../Jenkinsfile`](../../Jenkinsfile) · [`../../docker-compose.ci-e2e.yml`](../../docker-compose.ci-e2e.yml) | Orquestador · override de la E2E aislada |
| `microservicios/<servicio>/Jenkinsfile` | Un pipeline por servicio (7) |

## 0. Criterios de evaluación → dónde se demuestran

| # | Criterio (puntos) | Dónde | Comando o lugar |
|---|---|---|---|
| 1 | Configuración de Jenkins (1.0): servicio Docker, Dockerfile con plugins de reportes, acceso a Docker, pipeline de verificación, sin pasos manuales | [`jenkins/Dockerfile`](../../jenkins/Dockerfile), servicio `jenkins` de [`docker-compose.yml`](../../docker-compose.yml), [`jenkins/casc.yaml`](../../jenkins/casc.yaml) (trabajo `verificacion-docker`) | `docker compose --profile ci up --build -d` → `verificacion-docker` → Build Now (`whoami`=root, `docker ps` del host) |
| 2 | Jenkinsfile, Build, Test y reportes (1.0): Checkout/Build/Test, `junit` y `recordCoverage` en `post { always }`, trabajos como código, pruebas del Reto 6, tendencias | `microservicios/*/Jenkinsfile`, `casc.yaml` | Trabajo `gestion-empleados` → **Tests**, **Coverage** y las gráficas de tendencia de la página del trabajo |
| 3 | Calidad con SonarQube (1.0): dockerizado, cobertura del Reto 6 importada, gate ≥ 70 %, el pipeline falla si no pasa, dashboard | [`sonar/init.sh`](../../sonar/init.sh), etapas SonarQube y Quality Gate | `http://localhost:9000/dashboard?id=gestion-empleados`; fallo en [`pruebas.md` §2.2](pruebas.md#22-la-cobertura-baja-del-70---se-detiene-en-quality-gate) |
| 4 | Package, E2E y reporte BDD (1.0): imagen Docker, E2E automática, reporte navegable en español, logs archivados, ≥ 2 Jenkinsfiles en lenguajes distintos, `down -v` | [`Jenkinsfile`](../../Jenkinsfile), [`docker-compose.ci-e2e.yml`](../../docker-compose.ci-e2e.yml); 7 Jenkinsfiles en 5 lenguajes | `pipeline-orquestador-e2e` → **Cucumber reports**, **Reporte BDD**, **Build Artifacts** (`docker-logs.txt`) |
| 5 | Reproducibilidad y documentación (1.0): ejecuciones consistentes, 4 fallos en la etapa correcta identificados en el reporte, README con tabla de reportes, acceso y capturas | [`pruebas.md`](pruebas.md) §1 y §2, [sección del README raíz](../../README.md#integración-continua-reto-7) | 27 builds `SUCCESS` (9 trabajos × 3); fallos: `api-gateway` #14, `gestion-empleados` #15, `gestion-perfiles` #16, `pipeline-orquestador-e2e` #15 |

Desviaciones del PDF, con su razón:

| PDF | Aquí | Por qué |
|---|---|---|
| "El sistema completo debe levantarse con `docker compose up --build`" | La CI va en el perfil `ci`: `docker compose --profile ci up --build -d` (D11) | Los Retos 1 a 6 ya se calificaron con `docker compose up` levantando solo el sistema; no se cambia lo entregado |
| Se recomiendan imágenes Docker como agentes | `agent any` con la imagen de Jenkins (D13, la de Valeria) | Los agentes efímeros agregan otra capa de montajes (workspace y socket) sin mejorar el resultado |
| Por convención, pruebas fallidas = UNSTABLE | FAILURE y el pipeline se detiene (D12) | Código con pruebas rotas no debe empaquetarse |
| Pipeline con SonarQube en cada servicio | `gestion-departamentos` (PHP) sin Test, SonarQube ni gate | No tiene pruebas desde el Reto 6: un Test vacío sería un verde falso |

## 1. Prerrequisitos

| Herramienta | Versión probada | Nota |
|---|---|---|
| Docker Engine o Docker Desktop | 26.1.5 | Linux (amd64). Todas las imágenes de la CI tienen variante arm64 (Mac con Apple Silicon) |
| Docker Compose v2 | 5.6.1 | El override de la E2E usa `!reset` (Compose ≥ 2.24) |
| Disco libre | ≥ 10 GB recomendado | Imágenes de Jenkins y SonarQube, caché de builds. Ver [problemas comunes](#5-problemas-comunes) |
| Navegador | cualquiera | Jenkins y SonarQube |

No hace falta Node, Go, Java, Python ni PHP en la máquina: los trae la imagen de Jenkins.

## 2. Inicio rápido

```bash
# Desde la raíz del repo (la ruta de los workspaces sale de ${PWD}, D16)
docker compose --profile ci up --build -d       # sistema (16) + CI (5)
docker logs sonar-init                          # debe terminar en "listo" sin errores
docker compose ps jenkins                       # Up
```

1. Abrir `http://localhost:9090` e iniciar sesión con `admin` / `talentflow_ci_2026`. No hay asistente ni contraseña inicial: el administrador lo crea JCasC ([cómo](../../README.md#configuración)).
2. Comprobar que aparecen los 9 trabajos.
3. Ejecutar `verificacion-docker` → Build Now: verde en ~1 s.
4. Ejecutar un servicio (`gestion-empleados`, ~17 s) y el orquestador (`pipeline-orquestador-e2e`, ~2 min).

Los trabajos leen `main`. Para otra rama: `CI_BRANCH=lyc4nthrope docker compose --profile ci up -d` (se toma al arrancar; un `up` sin la variable vuelve a `main`).

Apagar la CI sin tocar el sistema ni borrar datos: `docker compose --profile ci stop registry sonarqube sonar-db jenkins`.

## 3. Cómo ver cada reporte

| Reporte | Dónde | Ejemplo verificado |
|---|---|---|
| Vista de etapas | Página del trabajo | `http://localhost:9090/job/pipeline-orquestador-e2e/` |
| Resultados de pruebas | Build → **Tests** | `http://localhost:9090/job/gestion-empleados/14/testReport/` |
| Cobertura | Build → **Coverage** | `http://localhost:9090/job/gestion-empleados/14/coverage/` |
| Tendencias (pruebas y cobertura) | Página del trabajo, después de varios builds | `http://localhost:9090/job/gestion-empleados/` |
| Reporte BDD navegable | Build → **Cucumber reports** | `http://localhost:9090/job/pipeline-orquestador-e2e/14/cucumber-html-reports/overview-features.html` |
| HTML de Cucumber | Build → **Reporte BDD** | `http://localhost:9090/job/pipeline-orquestador-e2e/14/Reporte_20BDD/` |
| Logs del sistema | Build → **Build Artifacts** → `docker-logs.txt` | `http://localhost:9090/job/pipeline-orquestador-e2e/14/artifact/docker-logs.txt` |
| Dashboard de calidad | SonarQube | `http://localhost:9000/dashboard?id=gestion-empleados` (claves: `api-gateway`, `auth-service`, `gestion-empleados`, `gestion-perfiles`, `gestion-vacaciones`, `notificaciones`) |
| Quality gate | SonarQube → Quality Gates → **TalentFlow** | `coverage` < 70 → ERROR |
| Imágenes publicadas | Registro | `curl -s localhost:5000/v2/_catalog` · `curl -s localhost:5000/v2/gestion-empleados/tags/list` |

Qué reporte produce cada etapa y con qué paso se publica: [tabla de reportes publicados](../../README.md#tabla-de-reportes-publicados).

## 4. Preguntas probables en la sustentación

- **¿Por qué JCasC + Job DSL y no `init.groovy.d` o un volumen?** Los trabajos y la configuración quedan versionados en `casc.yaml` y se aplican en cada arranque. `init.groovy.d` es código imperativo que se ejecuta una vez; un volumen local depende de la máquina donde se configuró a mano (D2, Valeria).
- **¿Socket o Docker-in-Docker?** Socket del host (*Docker-outside-of-Docker*): los pipelines usan el daemon de la máquina. Por eso el workspace se monta en la **misma ruta absoluta** en host y contenedor (D16): el daemon resuelve los montajes con rutas del host.
- **¿Por qué Jenkins corre como root?** Quien usa el socket ya controla el Docker del host: root dentro del contenedor no agrega privilegio. `chmod 666` en el host abriría Docker a todo proceso del equipo, y el GID de `docker` cambia entre máquinas (D17).
- **¿Cómo se obtiene la contraseña inicial?** No la hay: el asistente está desactivado y el administrador lo crea JCasC con `JENKINS_ADMIN_USER` / `JENKINS_ADMIN_PASSWORD` (D18). `docker exec jenkins printenv JENKINS_ADMIN_PASSWORD` muestra la que usa.
- **¿FAILURE o UNSTABLE?** FAILURE ante cualquier prueba fallida (D12): un servicio con pruebas rotas no debe llegar a Package. Los reportes se publican igual por el `post { always }`.
- **¿Dónde vive el token de SonarQube?** Lo genera `sonar-init` para un usuario `jenkins` con solo `scan` y `provisioning`, lo escribe en el volumen `sonar_token` y JCasC lo carga como la credencial `sonar-token`. No está en el repo; un token vigente se conserva entre arranques (D15, D19).
- **¿Cómo funciona `waitForQualityGate`?** El scanner sube el análisis; SonarQube avisa a Jenkins por el webhook `http://jenkins:8080/sonarqube-webhook/` (lo crea `sonar-init`); `waitForQualityGate abortPipeline: true` espera ese aviso con un tope de 5 minutos.
- **¿Por qué la simulación de cobertura se detuvo en Quality Gate con un módulo sin pruebas?** Hay dos capas. Test exige el 100 % de la lógica de negocio (Reto 6): quitar una prueba hace fallar **Test**, antes y más estricto que el gate. Pero el umbral de Node solo mide los archivos que alguna prueba importa, así que un módulo de negocio nuevo sin pruebas pasa Test. SonarQube mide todos los archivos de `sonar.sources`: con `liquidacion.js` (253 líneas sin cubrir) la cobertura bajó a 69.8 % y el gate detuvo el pipeline ([evidencia](pruebas.md#22-la-cobertura-baja-del-70---se-detiene-en-quality-gate)).
- **¿Cómo se aísla la E2E?** Proyecto de Compose propio por build (`talentflow-ci-<build>`, red y volúmenes propios), override sin `container_name` ni puertos y `down -v --remove-orphans --rmi local` en el `post`. Cada build parte de bases vacías y no toca el sistema de desarrollo de la misma máquina.
- **¿Cómo saca Jenkins los reportes del contenedor `bdd-tests`?** Volumen montado `./e2e-tests/reports`; con el workspace en la misma ruta, el archivo aparece en el workspace de Jenkins.
- **¿Y si el sistema nunca levanta?** No hay `allowEmptyResults`: `junit` no encuentra el archivo y el build queda en rojo. Un verde sin reporte escondería el fallo.
- **¿Por qué `BUILD_NUMBER` y no el hash del commit?** Liga cada imagen al build de Jenkins y a sus reportes; `:latest` queda para el siguiente despliegue y como caché.
- **¿Es reproducible?** Los 9 trabajos pasaron 3 veces seguidas (27 builds) sin cambios entre builds, con duraciones que varían pocos segundos ([evidencia](pruebas.md#1-cada-pipeline-pasa-tres-veces-seguidas-p51)).
- **¿Funciona en Mac?** Sin rutas propias, servicios por nombre en la red de Compose, el socket en la misma ruta en Docker Desktop y Linux, y las imágenes en amd64 y arm64 (D10). Lo probado de punta a punta es Linux; la prueba en el Mac de Valeria es parte de F8.

## 5. Problemas comunes

| Síntoma | Causa y solución |
|---|---|
| Jenkins no arranca; `docker logs sonar-init` termina en error | `sonar-init` debe terminar bien antes de Jenkins. Si dice `SonarQube no llegó a UP en 5 minutos`, ver la fila del disco. Si dice `admin no autentica con SONAR_ADMIN_PASSWORD`, la contraseña se cambió en `.env` después del primer arranque: volver a la anterior o cambiarla también en SonarQube (My Account → Security) |
| SonarQube se detiene o no indexa; su `logs/es.log` dice `flood stage disk watermark … exceeded` | Elasticsearch bloquea los índices cuando queda poco disco. D20 lo lleva a < 1 GB libre (por defecto sería > 95 % de uso). Liberar espacio (`docker builder prune`, `docker system df` para ver qué ocupa) y `docker compose --profile ci restart sonarqube`: el arranque quita el bloqueo |
| El nodo de Jenkins aparece *offline*; los builds quedan en cola | Quedó menos de 1 GiB libre. Liberar espacio y, en Manage Jenkins → Nodes, **Refresh status** (o esperar el siguiente chequeo) |
| Los trabajos fallan con `Jenkinsfile not found` o construyen otra rama | Leen `main` por defecto y `CI_BRANCH` se toma al arrancar. `CI_BRANCH=<rama> docker compose --profile ci up -d` (un `up` sin la variable vuelve a `main`) |
| Un trabajo nuevo de `casc.yaml` no aparece (404) | Carrera al arrancar: el trabajo quedó en disco pero no en memoria. `POST /reload` con *crumb* (ver el ejemplo REST de la [sección d](../../README.md#configuración)) o Manage Jenkins → Reload Configuration from Disk |
| `permission denied` al usar `/var/run/docker.sock` en `docker ps` | Jenkins debe correr como root (`user: root` en `docker-compose.yml`, D17). No usar `sudo chmod 666 /var/run/docker.sock`: deja que cualquier proceso controle Docker, que equivale a root en la máquina |
| Los bind mounts de la E2E salen vacíos (`init.sql` o `e2e-tests/reports` no encontrados) | Compose se ejecutó fuera de la raíz del repo: `${PWD}` no apunta al repo y la ruta del workspace no coincide (D16). Volver a levantar desde la raíz, o fijar `HOST_WORKSPACE` en `.env` |
| Mac: los montajes no funcionan | Docker Desktop comparte `/Users` por defecto: clonar el repo dentro de `/Users/<usuario>/…` (o agregar la carpeta en Settings → Resources → File sharing) y ejecutar desde la raíz |
| El enlace "SonarQube" de Jenkins no abre | Apunta a `http://sonarqube:9000` (nombre interno de la red de Compose). Abrir `http://localhost:9000` |
| `docker push` falla | El registro no está arriba: `docker compose ps registry` y `curl -s localhost:5000/v2/_catalog` |
| Un push dispara los nueve trabajos | El *polling* no filtra por carpeta. Para probar un solo trabajo, deshabilitar los demás (Disable Project); recrear Jenkins los vuelve a habilitar |
