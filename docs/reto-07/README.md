# Reto 7 — Integración continua con Jenkins, SonarQube y registro local

Este reto añade la capa de CI/CD al proyecto: un registro de imágenes Docker local, un servidor SonarQube para análisis de calidad, y un Jenkins automatizado con pipelines por servicio y un pipeline orquestador. La intención es dejar el repositorio preparado para compilar, probar, analizar y publicar cada microservicio desde un mismo flujo de integración continua.

| Archivo | Para qué |
|---|---|
| **Este README** | Manual de arranque, estructura, jobs, pipeline principal, reportes y problemas comunes |
| [`PLAN.md`](PLAN.md) | Diagnóstico de partida, decisiones de diseño y aprendizajes del reto |
| [`../../docker-compose.yml`](../../docker-compose.yml) | Orquestación de `registry`, `sonarqube`, `sonar-db` y `jenkins` |
| [`../../jenkins/`](../../jenkins/) | Configuración de Jenkins mediante Configuration as Code, plugins y Dockerfile base |
| [`../../Jenkinsfile`](../../Jenkinsfile) | Pipeline central que construye y publica todos los microservicios y lanza la suite BDD |
| [`../../microservicios/*/Jenkinsfile`](../../microservicios/) | Pipelines individuales por servicio, con build, test, cobertura, SonarQube y publicación |

## 0. Qué se incorpora en este reto

El sistema ya había superado los retos de arquitectura, seguridad y automatización de pruebas. En este reto se añade la capa operacional de ingeniería de calidad y entrega continua:

- `registry:2` para publicar imágenes Docker locales.
- `sonarqube` + `sonar-db` para medir calidad estática y cobertura.
- `jenkins` con Docker-in-Docker y herramientas del stack (Node, Maven, Go, PHP, Python, Java, Docker Compose, SonarScanner).
- `Jenkinsfile` raíz para construir y publicar todos los microservicios.
- `pipelineJob(...)` por servicio usando Job DSL, con triggers por polling SCM.
- Integración directa con la suite E2E y publicaciones de reportes HTML, JUnit y cobertura en Jenkins.

## 1. Estructura del reto

```text
TalentFlow/
├── Jenkinsfile                     # pipeline maestro del repositorio
├── docker-compose.yml              # incluye registry, sonar, jenkins y servicios
├── jenkins/
│   ├── Dockerfile                  # imagen base de Jenkins + plugins + toolchain
│   └── casc.yaml                  # Job DSL: jobs de Jenkins
├── microservicios/
│   ├── auth-service/Jenkinsfile
│   ├── gestion-empleados/Jenkinsfile
│   ├── gestion-departamentos/Jenkinsfile
│   ├── gestion-perfiles/Jenkinsfile
│   └── gestion-vacaciones/Jenkinsfile
└── e2e-tests/
```

## 2. Arranque rápido

Desde la raíz del proyecto:

```bash
docker compose up --build -d
```

Esto levanta la infraestructura de CI/CD junto al sistema principal. Las URLs relevantes son:

| Servicio | URL | Uso |
|---|---|---|
| Jenkins | `http://localhost:9090` | panel principal de CI/CD |
| SonarQube | `http://localhost:9000` | análisis de calidad y calidad de código |
| Registry local | `http://localhost:5000` | registro de imágenes Docker |
| API Gateway | `http://localhost:8080` | sistema de negocio |
| RabbitMQ UI | `http://localhost:15672` | monitorización del broker |
| Mailhog | `http://localhost:8025` | correos de pruebas |

## 3. Qué hace cada pieza

### 3.1 Registro local (`registry`)

Se añade un contenedor `registry:2` para guardar imágenes de cada servicio construidas por Jenkins.

```yaml
registry:
  image: registry:2
  ports:
    - "5000:5000"
```

Esto permite publicar artefactos del tipo:

```bash
localhost:5000/auth-service:latest
localhost:5000/gestion-empleados:latest
```

La intención es que el pipeline de entrega no dependa de Docker Hub ni de un registro externo; en un entorno académico o local, el registro local es suficiente para demostrar la publicación automática.

### 3.2 SonarQube (`sonarqube` + `sonar-db`)

Se añade una base PostgreSQL para SonarQube y un contenedor `sonarqube:lts-community` conectado a ella.

```yaml
sonarqube:
  image: sonarqube:lts-community
  ports:
    - "9000:9000"
  environment:
    - SONAR_JDBC_URL=jdbc:postgresql://sonar-db:5432/sonar
    - SONAR_JDBC_USERNAME=sonar
    - SONAR_JDBC_PASSWORD=sonar
```

Esto hace posible:

- analizar código fuente con calidad estática,
- revisar deuda técnica,
- definir calidad por proyecto,
- bloquear el pipeline con `Quality Gate` cuando la rama falla el criterio del proyecto.

### 3.3 Jenkins

El servicio `jenkins` se monta desde `./jenkins` y se configura con Configuration as Code (`casc.yaml`).

El `Dockerfile` instala:

- `jenkins/jenkins:lts-jdk17`
- plugins: `docker-workflow`, `pipeline-stage-view`, `workflow-aggregator`, `git`, `configuration-as-code`, `job-dsl`, `sonar`, `junit`, `coverage`, `cucumber-reports`, `htmlpublisher`
- Docker CLI + Docker Compose v2 + buildx
- Node.js, npm, Maven, Go, PHP, Python, Java JRE, unzip, curl, git
- SonarScanner CLI
- herramientas de conversión de cobertura (`go-junit-report`, `gocover-cobertura`)

Además se monta el socket Docker del host para que Jenkins pueda levantar contenedores y compilar imágenes dentro del mismo entorno local.

## 4. Job DSL y pipelines por servicio

La configuración en `jenkins/casc.yaml` crea pipelines por servicio, conectados con el repositorio GitHub y disparados por polling SCM cada minuto:

- `pipeline-orquestador-e2e`
- `gestion-empleados`
- `auth-service`
- `gestion-departamentos`
- `gestion-vacaciones`
- `gestion-perfiles`

Cada job apunta a su `Jenkinsfile` correspondiente y usa ramas del repositorio (por ejemplo `*/Vale292005`). La idea es que cada microservicio tenga su propia vida de CI, y que el pipeline raíz orqueste al final la validación global del sistema.

## 5. Pipeline maestro del repositorio

El `Jenkinsfile` raíz hace dos cosas importantes:

### 5.1 Construcción y publicación de imágenes

Se construyen en paralelo imágenes Docker para los microservicios con tags de build y `latest`:

```groovy
stage('Build & Publish Microservicios') {
  parallel {
    stage('auth-service') { ... }
    stage('gestion-departamentos') { ... }
    stage('gestion-empleados') { ... }
    stage('gestion-perfiles') { ... }
    stage('gestion-vacaciones') { ... }
  }
}
```

Cada paso ejecuta:

```bash
docker build -t localhost:5000/<servicio>:${BUILD_NUMBER} -t localhost:5000/<servicio>:latest .
docker push localhost:5000/<servicio>:${BUILD_NUMBER}
docker push localhost:5000/<servicio>:latest
```

### 5.2 Validación E2E

Después del build, se ejecuta la suite BDD del proyecto:

```bash
docker compose -f docker-compose.e2e.yml down -v --remove-orphans --rmi local
docker compose -f docker-compose.e2e.yml up -d --build --remove-orphans
docker compose -f docker-compose.e2e.yml run --rm bdd-tests
```

En el post-build se publican:

- `e2e-tests/reports/junit.xml`
- `e2e-tests/reports/cucumber.json`
- `e2e-tests/reports/cucumber.html`

con `junit` y `cucumber-reports` de Jenkins, y se archivan los logs del sistema para diagnóstico.

## 6. Pipelines por servicio

Cada microservicio tiene un `Jenkinsfile` muy similar, con la misma idea general:

1. `Checkout`
2. `Build`
3. `Test` con reportes de cobertura
4. `SonarQube`
5. `Quality Gate`
6. `Package`
7. `Publish`

### Ejemplo: `auth-service`

El pipeline de Go:

- compila con `go build ./...`
- ejecuta pruebas con `go test -v -coverprofile=reports/coverage.out ./...`
- convierte el log de Go a JUnit XML
- convierte la cobertura a Cobertura XML
- invoca `sonar-scanner`
- bloquea el pipeline si el `Quality Gate` falla
- crea la imagen y la publica en el registry local

La misma lógica se repite para los demás servicios con sus respectivos stacks y configuraciones.

## 7. Decisiones de diseño del reto

| Decisión | Justificación |
|---|---|
| Jenkins + Job DSL | Permite crear jobs de forma reproducible y declarativa |
| SonarQube en una imagen separada | Mantiene análisis de calidad y base de datos aislados del runtime del sistema |
| Docker-in-Docker en Jenkins | Permite construir, probar y publicar imágenes desde el propio agente |
| Registro local | Evita depender de Docker Hub para entregar artefactos en entorno local |
| Pipeline raíz + pipelines por servicio | Separa validación unitaria por microservicio y orquesta la validación del sistema completo |
| Publicación de reportes JUnit/HTML | Facilita trazabilidad y seguimiento visual del estado del pipeline |

## 8. Cómo probar que funciona

```bash
docker compose up --build -d
```

Luego:

1. Abre `http://localhost:9090`
2. Verifica que aparezcan los jobs creados por Job DSL
3. Ejecuta manualmente `pipeline-orquestador-e2e`
4. Revisa las etapas: `Checkout`, `Build & Publish Microservicios`, `E2E Tests`
5. Comprueba los reportes JUnit y Cucumber en Jenkins
6. Abre `http://localhost:9000` para revisar el análisis de SonarQube

## 9. Problemas comunes

| Síntoma | Causa y solución |
|---|---|
| Jenkins no puede ejecutar Docker | El socket del host no está accesible. Asegúrate de montar `/var/run/docker.sock` y, si hace falta, usa `chmod 666 /var/run/docker.sock` en un entorno local de pruebas |
| El pipeline falla al hacer `docker push` | El registry no está levantado o no responde en `localhost:5000`. Comprueba `docker compose ps` y `docker logs registry` |
| SonarQube no conecta con la base de datos | La BD `sonar-db` aún no está lista. Espera a que el contenedor esté `healthy` |
| El job no aparece en Jenkins | Revisa la configuración de Job DSL y si `casc.yaml` se está montando correctamente en `/var/jenkins_home/casc_configs` |
| El análisis de Sonar no se ejecuta | Faltan plugins o el contenedor no tiene `sonar-scanner` instalado correctamente; vuelve a reconstruir el `jenkins` image |
| Reportes de cobertura no aparecen | El servicio no generó `coverage.xml` o `junit.xml`; revisa el log del `Test` stage y la herramienta de conversión utilizada |
| La construcción tarda mucho | La imagen de Jenkins instala varias herramientas y compila varios servicios; la primera corrida es la más lenta |

## 10. Conclusión

El reto 7 cierra la capa de automatización de entrega con un flujo CI/CD realista para un sistema de microservicios: compilación, pruebas, análisis estático, publicación de artefactos y validación con la suite E2E. Es la base que convierte a TalentFlow de un conjunto de servicios funcionales en un proyecto listo para integración continua y despliegue automatizado.

---

> Este manual sigue el mismo estilo que los anteriores retos del proyecto: guía de uso, diagnóstico de piezas, decisiones de diseño y soluciones a errores frecuentes.
