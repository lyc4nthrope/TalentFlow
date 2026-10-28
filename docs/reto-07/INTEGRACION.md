# Reto 7 — Integración del trabajo del equipo

Cómo se juntó el avance de Valeria (`Vale292005`) con las correcciones hechas en `lyc4nthrope`, y cómo poner al día su rama. El detalle de cada decisión está en [`PLAN.md`](PLAN.md); la evidencia, en [`pruebas.md`](pruebas.md).

## 1. Línea de tiempo

| Fecha | Rama | Qué pasó |
|---|---|---|
| 2026-10-25 a 2026-10-27 | `Vale292005` (hasta `ea54ef4`) | Diseño y base del reto: Jenkins en Docker con plugins y herramientas, trabajos por JCasC + Job DSL, SonarQube, registro local, pipeline raíz y cinco por servicio |
| 2026-10-27 | `lyc4nthrope` | Su rama se integró con un merge que conserva sus 142 commits (`0b88e5e`) y se auditó contra el PDF |
| 2026-10-27 a 2026-10-28 | `lyc4nthrope` | Fases F1 a F7 del plan: correcciones, SonarQube como código, pipeline orquestador, evidencia y documentación |

## 2. Lo que aportó Valeria y se conservó

Las decisiones D1 a D8 del plan son suyas: Jenkins como orquestador, trabajos como código (JCasC + Job DSL), registro local, SonarQube con quality gate, un pipeline raíz más uno por servicio, la infraestructura en Docker Compose, la publicación de reportes en Jenkins y la suite BDD como validación final. También la imagen de Jenkins con las herramientas de todos los lenguajes (D13).

## 3. Lo que se cambió y por qué

| Problema encontrado | Por qué importaba | Cómo quedó |
|---|---|---|
| Ruta de una máquina (`/Users/valeria/…`) y `host.docker.internal` | No funcionaba en Linux ni en la máquina del profesor | Carpeta de trabajo común calculada en cada máquina (D16); servicios por su nombre (D10) |
| `buildx` descargado para un solo procesador | En Intel/AMD daba `exec format error` | Herramientas del repositorio de Docker, para cada arquitectura (F3b) |
| `\|\| true` en las pruebas y reportes con `allowEmptyResults` | Las fallas se escondían y se llegaba a empaquetar | Una prueba que falla detiene el pipeline (D12) |
| Reportes buscados en `target/…` y `tests/` | Jenkins publicaba resultados vacíos sin avisar | Mismos comandos y rutas del Reto 6 (D14) |
| SonarQube sin token, gate ni webhook | El Punto 3 no se cumplía | SonarQube como código: contenedor de arranque (D15, D19) |
| `docker-compose.e2e.yml` reescrito, `container_name` comentados, Dockerfile de empleados sin lockfile | Rompía el Reto 6 ya calificado | Restaurado; la CI va en el perfil `ci` (D11) |
| Jenkins sin usuario | Controla el Docker de la máquina | Administrador por JCasC (D18) |
| Limpieza con `docker image prune` / `volume prune` globales | Borraba recursos ajenos al pipeline | Se borra solo la copia aislada de cada build (F5) |
| Sugerencia de `chmod 666` sobre el socket | Daba control de Docker a cualquier proceso de la máquina | Explicación en D17 |

## 4. Cómo poner al día `Vale292005`

Su rama está contenida entera en la versión integrada (`ea54ef4` es antecesor de `dev`), así que **la actualización es un avance directo, sin conflictos y sin `--force`**:

```bash
git checkout Vale292005
git pull origin Vale292005
git fetch origin
git merge origin/dev          # fast-forward: no crea conflictos
git push origin Vale292005
```

Si en su rama hubiera commits nuevos posteriores a `ea54ef4`, el merge los combina con `dev`; en ese caso conviene avisar antes para revisarlos.

## 5. Checklist para probar en Mac (Docker Desktop)

| # | Paso | Se espera |
|---|---|---|
| 1 | Clonar o actualizar el repo dentro de `/Users/...` (Docker Desktop comparte esa carpeta por defecto) | — |
| 2 | Desde la raíz del repo: `docker compose --profile ci up --build -d` | Los 21 servicios arriba; `sonar-init` termina con código 0 |
| 3 | Abrir `http://localhost:9090` y entrar con `admin` / `talentflow_ci_2026` | Los 9 trabajos listados |
| 4 | Ejecutar `verificacion-docker` | SUCCESS, con `docker ps` en la consola |
| 5 | Ejecutar `gestion-empleados` y `gestion-vacaciones` | SUCCESS, con las pestañas Tests y Coverage llenas (vacaciones usa Testcontainers) |
| 6 | Ejecutar `pipeline-orquestador-e2e` | SUCCESS con 61 escenarios y el reporte de Cucumber |
| 7 | Abrir `http://localhost:9000` y entrar con `admin` / `talentflow_sonar_2026` | Los proyectos con el gate "TalentFlow" en OK |

Si algo falla en el Mac, se anota en "Fuera de alcance" del plan con el mensaje exacto.
