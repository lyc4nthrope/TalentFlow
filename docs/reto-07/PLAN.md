# Reto 7 — Plan de implementación

Fuente de requisitos: `reto7.pdf` (Puntos 1 a 5, decisiones técnicas requeridas, entregables, consideraciones y criterios).
Punto de partida: el avance de Valeria (`Vale292005`, `ea54ef4`), integrado en `lyc4nthrope` con un merge que conserva sus commits (`0b88e5e`).
Cada fase termina **verificada** (pipelines ejecutados en Jenkins, no solo en consola), con su propio commit.

## Diagnóstico de partida (auditoría del 2026-10-27)

El avance de Valeria trae la base del reto: Jenkins en Docker con los plugins y las herramientas de todos los lenguajes (`jenkins/Dockerfile`), los trabajos creados como código (`jenkins/casc.yaml`), SonarQube, su base de datos y un registro local de imágenes (`docker-compose.yml`), un pipeline raíz y cinco por servicio. Lo que falta para cumplir el PDF:

| Requisito (punto) | Estado | Brecha |
|---|---|---|
| Jenkins en Compose, plugins, socket, `:9090` (P1) | ✅ `jenkins/Dockerfile:4-15`, `docker-compose.yml:51-76` | — |
| Pipeline de verificación con `docker ps` (P1) | ❌ | No existe |
| Publicar resultados y cobertura (P2) | ⚠️ empleados bien; vacaciones busca `target/surefire-reports` y `target/site/jacoco`, pero el Reto 6 deja `reports/junit` y `reports/coverage/jacoco.xml`; departamentos busca `tests/` y la carpeta es `test/` | Con `allowEmptyResults` los reportes salen vacíos sin aviso |
| Una prueba rota detiene el pipeline (P2, P5) | ❌ `npm test \|\| true`, `go test … \|\| true` | Las fallas se esconden y se llega a empaquetar |
| Trabajos como código (P2) | ⚠️ `casc.yaml` apunta a `*/Vale292005` | Debe apuntar a `main`, configurable |
| SonarQube: token, gate ≥ 70 %, webhook, `waitForQualityGate` (P3) | ❌ solo el servidor | Configuración manual y no documentada |
| Cobertura importada en SonarQube (P3) | ⚠️ rutas que no coinciden con las del Reto 6 | Corregir cada lenguaje |
| Empaquetar con `BUILD_NUMBER` y `latest`, E2E con Cucumber, logs y limpieza (P4) | ⚠️ `Jenkinsfile:95-186` existe, pero `run bdd-tests \|\| true` puede dar verde con cero pruebas | Que falle de verdad |
| Funciona en cualquier máquina | ❌ `Jenkinsfile:5` usa `/Users/valeria/…`; SonarQube en `host.docker.internal` (no existe en Linux) | Sin rutas propias; servicios por nombre |
| No romper el Reto 6 | ❌ `docker-compose.e2e.yml` reescrito, `container_name` comentados, Dockerfile de empleados sin lockfile, `docker compose up` levanta también la CI | Restaurar y separar |
| 3 ejecuciones, 4 fallos simulados, 5 capturas (P5) | ❌ | Todo |
| README: CI, acceso, pasos a–d, etapas, tabla de reportes, verde/amarillo/rojo (P5) | ⚠️ `docs/reto-07/README.md` cubre URLs y etapas; el README raíz enlaza una sección que no existe | Completar |

No hay contraseñas ni tokens en el repositorio.

## Decisiones de diseño

D1 a D8 son las decisiones de Valeria y se conservan. D9 en adelante completan lo que el PDF pide justificar.

| # | Decisión | Elección | Por qué |
|---|---|---|---|
| D1 | Orquestador | Jenkins (Valeria) | Pipelines declarativos y plugins para publicar pruebas, cobertura y Cucumber |
| D2 | Aprovisionamiento de trabajos | JCasC + Job DSL (Valeria) | Los trabajos quedan versionados y Jenkins arranca ya configurado, sin clics. Es lo que pide el PDF frente a `init.groovy.d` o volúmenes locales |
| D3 | Registro de imágenes | `registry:2` local (Valeria) | Opcional en el PDF; evita depender de Docker Hub |
| D4 | Calidad | SonarQube con quality gate (Valeria) | Frena la entrega si no se cumple el mínimo |
| D5 | Niveles de pipeline | Uno raíz (imágenes + E2E) y uno por servicio (Valeria) | Cada servicio muestra su propio estado; un fallo no queda escondido en el general |
| D6 | Infraestructura | En Docker Compose (Valeria) | Se levanta con un comando, igual en cualquier equipo |
| D7 | Reportes | JUnit, cobertura y HTML publicados en Jenkins (Valeria) | Se leen en la interfaz, no solo en la consola |
| D8 | E2E como validación final (Valeria) | La suite BDD del Reto 6 | Comprueba el sistema completo |
| D9 | Integración del equipo | Partir de la rama de Valeria (merge que conserva sus commits) y corregir encima | Cuando esto llegue a `dev`, su rama se actualiza con un `git merge origin/dev` sin conflictos, porque `dev` ya contiene sus commits |
| D10 | Portabilidad (Valeria usa Mac, Cristhian Linux) | Sin rutas propias; servicios por su nombre en la red de Compose (`sonarqube:9000`); socket en `/var/run/docker.sock`, que es la misma ruta en Docker Desktop y en Linux | Lo que funciona en una máquina tiene que funcionar en la del profesor |
| D11 | Separar la CI del sistema | La infraestructura de CI se levanta con su propio comando; `docker compose up` sigue levantando solo el sistema | Los Retos 1 a 6 ya están calificados con ese comportamiento |
| D12 | Fallido vs. inestable | Una prueba que falla es **FAILURE** y detiene el pipeline | Un servicio con pruebas rotas no debe llegar a empaquetarse; el PDF exige que se detenga en Test (P5) |
| D13 | Agente | La imagen de Jenkins con todas las herramientas (`agent any`), la de Valeria | Ya funciona en su Mac. Los contenedores efímeros son más limpios, pero agregan otra capa de montajes (workspace y socket dentro del agente) sin mejorar la nota |
| D14 | Reportes en el pipeline | Los mismos comandos y rutas del Reto 6 (tabla de reportes del README) | Una sola fuente de verdad: lo que se ejecuta en local es lo que ejecuta Jenkins |
| D15 | SonarQube como código | Un contenedor de arranque crea el token, el gate "TalentFlow" (cobertura ≥ 70 %) y el webhook hacia Jenkins; Jenkins lee el token de un volumen compartido | Sin credenciales escritas en el repo y sin pasos manuales |
| D16 | Ruta del workspace (Docker-outside-of-Docker) | Compose monta solo `.jenkins-workspace/` del repo en la **misma ruta absoluta** dentro de Jenkins (`${PWD}/.jenkins-workspace`, o `HOST_WORKSPACE` si se define) y fija la raíz de workspaces con `-Djenkins.model.Jenkins.workspacesDir=<esa ruta>/${ITEM_FULL_NAME}` | Los pipelines llaman al daemon del host, que resuelve los bind mounts con rutas del host: el workspace tiene que existir igual en los dos lados. Con la propiedad global ningún Jenkinsfile conoce rutas (todos quedan `agent any`, como el PDF) y no se expone el repo entero al contenedor. `docker compose --profile ci up` se ejecuta desde la raíz del repo |
| D17 | Permisos del socket | Jenkins corre como `root` dentro de su contenedor; sin `chmod 666` en el host | Quien usa el socket ya controla el Docker del host, así que root en el contenedor no agrega privilegio. `group_add` con el GID de `docker` del host no sirve en Docker Desktop (el socket es de root) y el GID cambia entre equipos Linux; `chmod 666` abre Docker a todo proceso del equipo |
| D18 | Acceso a Jenkins | Un administrador creado por JCasC (`securityRealm` local, sin registro de usuarios; los anónimos no ven nada). Usuario y contraseña los inyecta Compose: `admin` / `talentflow_ci_2026` por defecto, sobrescribibles en `.env` | Jenkins controla el Docker de la máquina a través del socket: abierto, cualquiera que llegue al puerto 9090 podría hacerlo. Mismo criterio que el secreto del JWT (Reto 5): valor académico para arrancar con un comando, nada escrito en el código |

## Fases

| Fase | Alcance | Verificación | Estado |
|---|---|---|---|
| **F0** Integración | D9: merge de `Vale292005`; este plan | Merge sin conflictos; commits de Valeria en la historia | ✅ |
| **F1** Recuperar el Reto 6 | D11: override de pruebas, `container_name`, Dockerfile de empleados con lockfile; CI con su propio comando | `make pruebas` y `make e2e` en verde, como antes | ✅ |
| **F2** Jenkins portable | D10: sin `/Users/…` ni `host.docker.internal`; rama configurable (por defecto `main`); trabajo de verificación con `docker ps` | Jenkins en `:9090` con sus trabajos; la verificación en verde | ✅ |
| **F3** Pipelines por servicio | D12, D14: comandos y rutas del Reto 6, sin `\|\| true`; se suman gateway y notificaciones | Cada trabajo en verde con Test Result y Coverage llenos | ⬜ |
| **F4** SonarQube | D15: token, gate ≥ 70 %, webhook, `waitForQualityGate`, cobertura de cada lenguaje | El gate pasa; con menos de 70 % se detiene en Quality Gate | ⬜ |
| **F5** Pipeline raíz | P4: imágenes `BUILD_NUMBER` y `latest`, E2E aislada, Cucumber, HTML, logs, limpieza sin borrar lo ajeno | En verde con el reporte BDD visible en Jenkins | ⬜ |
| **F6** Reproducibilidad | P5: 3 ejecuciones seguidas; 4 fallos simulados (unitaria, cobertura, Dockerfile, escenario BDD) | Cada fallo se detiene en su etapa y se identifica en el reporte de Jenkins; evidencia en `pruebas.md` y 5 capturas | ⬜ |
| **F7** Documentación | README raíz (sección del Reto 7, índice, mapa), manual, sustentación | Cada entregable del PDF presente | ⬜ |
| **F8** Cierre | PR `lyc4nthrope` → `dev` → `main`; Valeria actualiza su rama y prueba en su Mac | Ramas sincronizadas; checklist de Valeria en verde | ⬜ |

## Fuera de alcance (anotado para después)

- `docs/reto-07/guiaprueba.txt` sugiere `sudo chmod 666 /var/run/docker.sock`: da control de Docker (equivale a root) a cualquier proceso de la máquina. Se reemplaza en F2 por una solución dentro del contenedor y se documenta por qué.

## Aprendizajes por fase

- **F0**: el merge de `Vale292005` (142 commits, desde la base del Reto 6) entró sin conflictos. Su plan original tenía D1 a D8 y cuatro fases; se conservaron sus decisiones y se reorganizó en el formato del equipo, con el diagnóstico frente al PDF.
- **F1**: `docker-compose.yml`, `docker-compose.e2e.yml` y el Dockerfile de empleados vuelven a la versión del Reto 6. Los cuatro servicios de CI de Valeria (registry, sonarqube, sonar-db, jenkins) quedan en el mismo `docker-compose.yml` bajo el perfil `ci`, con sus `container_name`: `docker compose up` levanta solo los 16 servicios del sistema y `docker compose --profile ci up --build -d` suma la CI. Se quitaron los volúmenes de caché que ningún servicio usaba. Se conservan dos cambios de Valeria en la suite: el reintento acotado del login ante un 503 y el `try/catch` al vaciar `reports/`. Verificado: `make pruebas` en verde y `make e2e` 61/61, sin contenedores de CI. Queda para F3/F5: el pipeline raíz todavía apunta a su `docker-compose.e2e.yml` independiente y el de empleados construye la imagen con el contexto de la carpeta del servicio.
- **F2**: Jenkins ya no depende de la máquina. Workspaces en `<repo>/.jenkins-workspace/<trabajo>`, montados en la misma ruta en host y contenedor y fijados con `jenkins.model.Jenkins.workspacesDir` (D16); el pipeline raíz pasa a `agent any`. SonarQube por `${SONAR_HOST_URL}` (`http://sonarqube:9000`, red de Compose). El registro sigue en `localhost:5000` pero desde `REGISTRY_URL`: el push lo hace el daemon del host, que llega al puerto publicado y no exige TLS para `localhost`; el valor por defecto `registry:5000` de los Jenkinsfiles no lo resuelve el daemon y se cambió. `casc.yaml` lee `${CI_REPO_URL}` y `${CI_BRANCH:-main}`: JCasC los sustituye antes de que Job DSL lea el script (un `$` literal de Groovy iría como `^${…}`, por eso el script concatena). Nuevo trabajo `verificacion-docker` (P1). Socket: root en el contenedor (D17); `guiaprueba.txt` ya no recomienda `chmod 666`. Ojo: la ruta sale de `${PWD}`, así que Compose se ejecuta desde la raíz del repo. Verificado en Linux: los 7 trabajos aparecen, sin asistente ni login (Jenkins abierto: sin realm, como pide el entorno local; el REST exige crumb para POST) y `verificacion-docker` en verde con `whoami`=root y `docker ps` del host. Los seis de `main` fallan con "Jenkinsfile not found" hasta que el código llegue a `main` (F8); para probar antes, `CI_BRANCH=lyc4nthrope`. Si el disco baja de 1 GiB, Jenkins deja el nodo offline y no vuelve solo hasta el siguiente chequeo (`/computer/updateNow`).
- **F2 (cierre)**: Jenkins había quedado abierto, sin usuario. Se agregó D18; verificado: sin credenciales 403, con el admin 200, con otra contraseña 401, y `verificacion-docker` #2 en `SUCCESS` (root, Docker 26.1.5, Compose v2.29.1). Durante la fase el disco de la máquina llegó al 100 % y SonarQube se detuvo: Elasticsearch bloquea la escritura por encima del 95 % de uso (*flood-stage*). Hace falta el disco por debajo del ~90 %.
