# Proyecto final — Plan del equipo (Cristhian y Valeria)

Plan para terminar el proyecto final en objetivos pequeños, repartidos en partes iguales.
Cada objetivo es **una rama, un PR a `dev` y una revisión de la otra persona**.

- Fuente de requisitos: `proyectoFinal.pdf` (carpeta `context_microservicios`).
- Diagnóstico y decisiones de diseño (D1–D10): [`PLAN.md`](PLAN.md). Este documento no las repite: las aplica.
- Punto de partida: `dev` con F0–F2 integradas (PR #17). Retos 1–8 completos.

> **Para un agente de IA:** lee primero las secciones 1 y 2 completas. Después, solo la ficha del objetivo que te asignaron. No hagas trabajo de otra ficha. Si algo de la ficha no coincide con el código, detente y pregunta: no improvises otra solución.

---

## 1. Contexto del sistema

### Servicios

| Servicio (Compose) | Lenguaje | Puerto interno | Base de datos | Pruebas (desde la raíz) |
|---|---|---|---|---|
| `api-gateway` | Node 22 (Express) | 8080 (publicado) | — | `make pruebas-gateway` |
| `empleados-service` | Node 22 (Express) | 8081 | PostgreSQL `database-empleados` | `make pruebas-empleados` |
| `departamentos-service` | PHP 8.2 | 8082 | MySQL `database-departamentos` | `make pruebas-departamentos` (Docker) |
| `perfiles-service` | Python 3.13 (FastAPI) | 8083 | PostgreSQL `database-perfiles` | `make pruebas-perfiles` |
| `notificaciones-service` | Go | 8084 | PostgreSQL `database-notificaciones` | `make pruebas-notificaciones` |
| `vacaciones-service` | Java (Spring Boot) | 8085 | PostgreSQL `database-vacaciones` | `make pruebas-vacaciones` (Testcontainers) |
| `auth-service` | Go | 8086 | PostgreSQL `database-auth` | `make pruebas-auth` |

- **Broker:** RabbitMQ (`message-broker`). **Correo de pruebas:** Mailhog (`localhost:8025`).
- **Observabilidad:** Prometheus `localhost:9091`, Grafana `localhost:3000`, Jaeger, Loki y Promtail.
- **CI** (perfil `ci`): Jenkins `localhost:9090`, SonarQube `localhost:9000` y registro local `localhost:5000`.

### Comandos

| Para… | Comando |
|---|---|
| Levantar el sistema | `docker compose up -d --build` |
| Levantar sin reconstruir imágenes | `docker compose up -d --no-build` |
| Todas las pruebas de los servicios | `make pruebas` |
| Suite E2E (BDD) completa | `make e2e` (reconstruye lo que cambió; no borra volúmenes) |
| Levantar la CI apuntando a una rama | `CI_BRANCH=<rama> docker compose --profile ci up -d` |
| Apagar todo sin borrar datos | `docker compose --profile ci stop` |

### Cosas que ya sabemos (evitan horas de depuración)

1. **No existe `.env`.** Hoy todo arranca con los valores por defecto `${VAR:-valor}` del compose. C2 los quita, así que desde C2 **hay que correr `bash scripts/setup-secrets.sh` antes de levantar**.
2. **`init.sql` solo corre con el volumen vacío.** Una columna nueva necesita una migración al arrancar (`ADD COLUMN IF NOT EXISTS`), como la de perfiles en F2. Nunca pedir que se borre un volumen.
3. **Las exclusiones de cobertura de SonarQube deben ser las mismas que las de las pruebas locales** (`pyproject.toml`, `package.json`, `phpunit.xml`…). Si no coinciden, el Quality Gate falla aunque las pruebas pasen (pasó en perfiles, build #22 a #86).
4. **Con menos de 1 GB libre en el disco, SonarQube deja sus índices en solo lectura** y el análisis falla. Revisar `df -h /` y, si hace falta, `docker builder prune -af`.
5. **Prometheus encuentra los servicios por nombre DNS** (`observability/prometheus/prometheus.yml`). Con varias réplicas, un nombre resuelve a varias IP: V6 debe cambiar a `dns_sd_configs`, o Prometheus verá una sola réplica.
6. **`container_name` impide escalar.** Los servicios de aplicación lo tienen (`ms-auth`, `api-gateway`, `ms-empleados`, `ms-departamentos`, `ms-notificaciones`, `ms-perfiles`, `ms-vacaciones`). Las guías usan `db-*` para las bases de datos: esas se quedan.
7. **El scheduler de vacaciones corre en cada instancia** (`ProcesadorCicloVacaciones.java`) y la reconciliación de empleados `PENDIENTE` también (`gestion-empleados/src/server.js`). Con réplicas hay que coordinarlos (V8).
8. **La regla de autorización vive en el Gateway** (`api-gateway/autorizacion.js`): ADMIN todo; USER solo lectura, `POST /auth/change-password` y `PUT /perfiles/{suId}`.

---

## 2. Reglas de trabajo (aplican a todos los objetivos)

### Ramas y PR

1. Cada objetivo nace de `dev` actualizado: `git switch dev && git pull && git switch -c pf/<ID>-<tema>` (ejemplo: `pf/C1-generar-secretos`).
2. Un objetivo = un PR a `dev`. El PR lo revisa **la otra persona**. Lo integra quien lo abrió, después de la aprobación.
3. El PR debe ser pequeño: menos de unas 400 líneas sin contar `composer.lock` o `package-lock.json`. Si crece, se parte en dos objetivos.
4. **`docker-compose.yml` es el archivo con más conflictos.** Solo puede haber **un PR abierto que lo toque** a la vez; se marca con la etiqueta `compose`. Antes de abrirlo, `git pull --rebase origin dev`.
5. `main` solo recibe un PR `dev → main` al cerrar cada ola (sección 3), como en los retos anteriores.

### Definición de terminado (todo PR debe cumplirla)

- [ ] Las pruebas del servicio tocado pasan (`make pruebas-<servicio>`) y la cobertura no baja del mínimo del servicio.
- [ ] `make e2e` pasa completo si el cambio toca comportamiento, el compose o el Gateway.
- [ ] El pipeline de Jenkins del servicio está en verde, Quality Gate incluido (`CI_BRANCH=<rama>`).
- [ ] Si cambia una API: OpenAPI y README del servicio actualizados.
- [ ] Ningún secreto, `.env`, `secrets/`, `vendor/`, `.venv/`, `target/` ni `reports/` en el commit.
- [ ] La ficha del objetivo marcada ✅ en este archivo, con una línea de "Resultado" con números reales.

### Convenciones

- **Idioma:** código, comentarios y documentación en español neutro, como el resto del repo. Respuestas cortas y tablas antes que prosa.
- **Commits:** conventional commits, `feat|fix|docs|test|refactor(proyecto-final): …`, **sin atribución a IA**.
- **Plantilla del PR:**

```markdown
<Una frase: qué resuelve y por qué.>

## Qué incluye
- …

## Verificación
- Pruebas: <servicio> N pruebas, X % · `make e2e`: N/N · Jenkins #N en verde

## Impacto para quien actualice
- <comandos nuevos, variables nuevas, contenedores que cambian> (o "Ninguno")
```

### Compatibilidad con macOS (Valeria trabaja en Mac)

Todo lo que se escriba debe funcionar en Linux **y** en macOS con Docker Desktop:

| Tema | En macOS | Regla |
|---|---|---|
| **Imágenes Docker** | Apple Silicon es `arm64` | Antes de fijar una imagen nueva, comprobar que tenga `linux/arm64`: `docker buildx imagetools inspect <imagen>:<versión>` |
| **bash** | Trae bash 3.2 | Sin arreglos asociativos, sin `${var,,}` ni `mapfile` |
| **sed** | `sed -i` es de BSD y pide un argumento extra (`sed -i ''`) | No usar `sed -i` en scripts: escribir a un temporal y moverlo con `mv` |
| **tr, stat, date, base64** | Son versiones BSD | `LC_ALL=C tr -dc …`; evitar `stat` y `date -d`; `openssl rand -base64 32` funciona igual |
| **Memoria** | Docker Desktop limita la RAM | Darle de 6 a 8 GB. Probar sin el perfil `ci` cuando no haga falta; con réplicas (V6 y V7), apagar la observabilidad si no se está midiendo |
| **Rutas y permisos** | El socket de Docker de Docker Desktop funciona como el de Linux (Jenkins y Promtail ya lo usan) | No usar rutas absolutas del host en scripts ni en el compose; usar `$PWD` o rutas relativas |
| **Puertos** | AirPlay usa el 5000 y el 7000 | El registro local usa `localhost:5000`: está fijo en `5000:5000` (`docker-compose.yml`). Si choca, desactivar *AirPlay Receiver* en Ajustes del Sistema → General → AirDrop y Handoff |

Para comprobar un script sin tener una Mac: `shellcheck` y `bash --posix`. En la revisión del PR, Valeria lo corre en su Mac.

---

## 3. Reparto y orden

| Persona | Bloque | Objetivos |
|---|---|---|
| **Cristhian** | Seguridad, secretos, redes y entrega | C1 a C8 |
| **Valeria** | Funcionalidad, resiliencia y escalado | V1 a V9 |

Tamaño: XS (< 1 h), S (1–2 h), M (media jornada), L (una jornada). Total de cada persona: **≈ 4 jornadas**.

### Olas (lo de una misma ola se hace en paralelo)

| Ola | Cristhian | Valeria | Por qué este orden |
|---|---|---|---|
| **1** | C1 (S) | V1 (M) · V2 (XS) · V3 (S) | Nada de esto toca el compose, salvo V1 (un volumen); sin conflictos |
| **2** | C2 (M) `compose` | V4 (M) · V5 (S) | C2 cambia todas las variables del compose; V4 y V5 son código del Gateway |
| **3** | C3 (S) → C4 (M) → C5 (M) `compose` | V8 (M) | Los secretos van servicio por servicio; V8 es Java sin compose |
| **4** | C8 (M) | V6 (M) `compose` → V7 (M) `compose` → V9 (S) | Las réplicas van después de los secretos para no chocar en el compose |
| **5** | C6 (M) `compose` → C7 (S) | Revisión de C6 y C7; ensayo de la sustentación | Las redes se tocan al final, con todo lo demás estable |

Al cerrar cada ola: PR `dev → main`.

**Opcional (puntaje extra):** Keycloak con JWKS (PLAN.md, F10). Solo si al final de la ola 4 sobra al menos una jornada; lo decide el equipo.

---

## 4. Fichas de Cristhian

### C1 · Generar los secretos con un script — S · Ola 1 · ⬜

- **Rama:** `pf/C1-generar-secretos` · **Depende de:** nada · **Revisa:** Valeria
- **Requisito del PDF:** "Debe incluirse un script de inicialización que genere los secretos localmente" y "`.env.example` con marcadores de posición".
- **Hoy:** `scripts/setup-secrets.sh` solo copia `.env.example` a `.env` y crea `secrets/`.
- **Archivos:** `scripts/setup-secrets.sh`, `.env.example`, README raíz (sección *Cómo levantar todo desde cero*).
- **Pasos:**
  1. Listar en `.env.example` **todas** las variables secretas del compose, con un marcador `CAMBIAR_ME`: `JWT_SECRET`, `ADMIN_PASSWORD`, `DB_AUTH_PASS`, `DB_EMPLEADOS_PASS`, `DB_DEPTO_PASS`, `DB_DEPTO_ROOT_PASS`, `DB_NOTIF_PASS`, `DB_PERFILES_PASS`, `DB_VACACIONES_PASS`, `RABBITMQ_PASS`, `GRAFANA_ADMIN_PASSWORD`, `JENKINS_ADMIN_PASSWORD`, `SONAR_ADMIN_PASSWORD` y la contraseña de `sonar-db`. Las variables que no son secretas (puertos, vigencias) conservan su valor.
  2. El script copia `.env.example` a `.env` **solo si `.env` no existe**, y cambia cada `CAMBIAR_ME` por un valor aleatorio (`openssl rand -base64 32`, sin `/`, `+` ni `=`, para que no rompa URLs ni `sed`).
  3. **`ADMIN_PASSWORD` debe cumplir la política de contraseñas de auth-service** (revisar sus reglas en `microservicios/auth-service`). Generarla con mayúscula, minúscula, número y símbolo.
  4. En `secrets/`, un archivo por secreto con el mismo valor (`secrets/jwt_secret`, `secrets/db_auth_pass`, …), con permisos `600`. Lo usarán C3 a C5.
  5. El script es idempotente: correrlo dos veces no cambia nada. Ofrece `--forzar` para regenerar todo.
- **Criterios de aceptación:**
  - `rm -f .env && rm -rf secrets && bash scripts/setup-secrets.sh` crea `.env` sin ningún `CAMBIAR_ME` (`grep -c CAMBIAR_ME .env` da `0`).
  - Correrlo una segunda vez no cambia `.env` (comparar `sha256sum`).
  - `git status` no muestra `.env` ni `secrets/`.
  - Con el `.env` generado y las bases **nuevas** (`-p tf-prueba`, sin tocar tus volúmenes), `docker compose -p tf-prueba up -d` arranca y `/health/servicios` da 200. Después: `docker compose -p tf-prueba down -v`.
- **Fuera de alcance:** quitar los valores por defecto del compose (C2).

### C2 · Cero secretos en el compose y en el código — M · Ola 2 · `compose` · ⬜

- **Rama:** `pf/C2-cero-secretos` · **Depende de:** C1 · **Revisa:** Valeria
- **Requisito del PDF:** "Ninguna contraseña, token, clave de firma o API key puede estar en el código fuente, en el `docker-compose.yml` ni en el historial de Git".
- **Archivos:**
  - **Compose:** `docker-compose.yml`, `docker-compose.e2e.yml` (`Admin123*`, línea ~42).
  - **CI y E2E:** `e2e-tests/support/config.js` (fallback `Admin123*`, línea ~49) y el `Jenkinsfile` raíz.
  - **Código:** `microservicios/notificaciones/cmd/notificaciones/main.go:127` y `microservicios/gestion-perfiles/app/config.py:25-26` (`guest/guest`).
  - **Documentación:** README.
- **Pasos:**
  1. Cambiar cada `${VAR:-valor_secreto}` por `${VAR:?Falta VAR: corre bash scripts/setup-secrets.sh}`. Hoy son 15 valores: buscarlos con `grep -nE '\$\{[A-Z_]*(PASS|SECRET|PASSWORD)[A-Z_]*:-' docker-compose.yml`. Las variables no secretas mantienen su valor por defecto.
  2. Quitar los fallbacks secretos del código. Si la variable falta, el servicio **falla al arrancar** con un mensaje claro, sin inventar un valor.
  3. **Jenkins (importante):** el `Jenkinsfile` raíz despliega una copia aislada (`talentflow-ci-<build>`). Sin `.env`, el compose fallará. Antes de *Deploy*, el pipeline corre `bash scripts/setup-secrets.sh`, que genera secretos nuevos para cada build.
  4. La suite E2E lee `ADMIN_PASSWORD` del `.env`. Revisar que `docker-compose.e2e.yml` se la pase al contenedor `bdd-tests`.
  5. README: una sección corta, *Secretos*, con los 2 comandos y el porqué.
- **Criterios de aceptación:**
  - `grep -nE ':-(talentflow|pass_|Admin123|admin\}|guest|root_pass|sonar)' docker-compose*.yml` no encuentra nada.
  - Sin `.env`, `docker compose config` falla y nombra la variable que falta.
  - Con `.env`: `make e2e` pasa completo, y el job `pipeline-orquestador-e2e` de Jenkins queda en verde.
- **Riesgo:** al cambiar las contraseñas, los volúmenes existentes conservan la contraseña vieja (Postgres solo la fija al crear el volumen). **En tu máquina:** pon en `.env` los valores de siempre (`pass_auth`, …) o recrea los volúmenes solo si no te importan los datos. Esto se documenta en el PR, en *Impacto*. Nunca borrar volúmenes sin preguntar.
- **Fuera de alcance:** pasar a Compose Secrets (C3–C5).

### C3 · Compose Secrets para la clave del JWT — S · Ola 3 · `compose` · ⬜

- **Rama:** `pf/C3-secreto-jwt` · **Depende de:** C2 · **Revisa:** Valeria
- **Requisito del PDF:** "Secretos gestionados con Docker Compose Secrets (o Vault)" (PLAN.md, D4).
- **Archivos:** `docker-compose.yml` (sección `secrets:` y servicios `auth-service` y `api-gateway`), el arranque de auth-service (Go), `microservicios/api-gateway/autenticacion.js` o su config, más sus pruebas.
- **Pasos:**
  1. En el nivel superior: `secrets: jwt_secret: file: ./secrets/jwt_secret`.
  2. `auth-service` y `api-gateway` montan el secreto y reciben `JWT_SECRET_FILE=/run/secrets/jwt_secret` en vez de `JWT_SECRET`.
  3. **Convención para todos los servicios (C3–C5):** si existe `X_FILE`, se lee el archivo (sin el salto de línea final); si no, `X`; si falta todo, falla al arrancar. Una función pequeña por lenguaje, con pruebas unitarias de los 3 casos.
- **Criterios de aceptación:**
  - `docker inspect ms-auth api-gateway | grep -i jwt_secret=` no muestra el valor.
  - Login y rutas protegidas funcionan: `make e2e` completo.
  - Las pruebas nuevas de la función de lectura pasan en Go y en Node.

### C4 · Compose Secrets para las contraseñas en Go y Node — M · Ola 3 · `compose` · ⬜

- **Rama:** `pf/C4-secretos-go-node` · **Depende de:** C3 · **Revisa:** Valeria
- **Alcance:**
  - **Servicios:** `auth-service`, `notificaciones-service` y `empleados-service`, con sus bases (`database-auth`, `database-notificaciones`, `database-empleados`) y `RABBITMQ_PASS`.
  - **Bases de datos:** la imagen de Postgres ya admite `POSTGRES_PASSWORD_FILE`. Usarla en `database-*`.
  - **RabbitMQ:** **verificar** si la versión de la imagen que usamos admite `RABBITMQ_DEFAULT_PASS_FILE`. Si no, `RABBITMQ_PASS` sigue por `.env` y se documenta en el README como desviación, con la razón.
- **Criterios de aceptación:**
  - `docker inspect` de esos contenedores no muestra contraseñas.
  - `make pruebas-auth pruebas-notificaciones pruebas-empleados` y `make e2e` pasan.

### C5 · Compose Secrets para las contraseñas en Python, Java y PHP — M · Ola 3 · `compose` · ⬜

- **Rama:** `pf/C5-secretos-py-java-php` · **Depende de:** C4 · **Revisa:** Valeria
- **Alcance:**
  - **Servicios:** `perfiles-service`, `vacaciones-service` y `departamentos-service`, con sus bases.
  - **MySQL:** la imagen admite `MYSQL_PASSWORD_FILE` y `MYSQL_ROOT_PASSWORD_FILE`.
  - **Spring Boot:** leer el archivo con la misma convención `X_FILE` en una clase de configuración con su prueba. Es más explícito que `configtree`.
  - **Grafana:** admite `GF_SECURITY_ADMIN_PASSWORD__FILE` (doble guion bajo); pasarla aquí también.
- **Criterios de aceptación:** los mismos de C4, en estos servicios y Grafana. Los pipelines de perfiles, vacaciones y departamentos en verde.

### C6 · Redes por capa — M · Ola 5 · `compose` · ⬜

- **Rama:** `pf/C6-redes-por-capa` · **Depende de:** V7 (para no chocar con Nginx) · **Revisa:** Valeria
- **Requisito del PDF:** el diagrama de infraestructura separa las redes `frontend`, `backend`, `messaging`, `data`, `observability` y `cicd`. Hoy hay una sola, `microservices-network`.
- **Reglas de conexión (escribirlas como tabla en el README):**
  - **frontend:** Nginx y Gateway.
  - **backend:** Gateway y los 6 servicios.
  - **messaging:** el broker y los servicios que publican o consumen.
  - **data:** cada base con su servicio. Opcional: una red por base, para que un servicio no vea la base de otro.
  - **observability:** Prometheus, Grafana, Loki, Promtail y Jaeger, más los servicios (métricas y trazas).
  - **cicd:** Jenkins, SonarQube, el registro y sonar-db.
  - **Mailhog:** comparte red con notificaciones y Grafana.
- **Criterios de aceptación:**
  - Desde el Gateway **no** se llega a una base de datos: `docker compose exec api-gateway wget -qO- --timeout=3 database-empleados:5432` falla.
  - `make e2e` pasa.
  - Los 7 targets de Prometheus siguen en UP.
  - La traza del alta de empleado sigue completa en Jaeger.
  - La CI (`pipeline-orquestador-e2e`) sigue en verde.

### C7 · Verificación desde cero, escaneo de secretos y entregable — S · Ola 5 · ⬜

- **Rama:** `pf/C7-entrega` · **Depende de:** todo lo anterior · **Revisa:** Valeria
- **Pasos:**
  1. Clonar el repo en una carpeta nueva, correr `bash scripts/setup-secrets.sh` y `docker compose up --build`: todo debe quedar `healthy` (el comando que pide el PDF).
  2. Escanear el historial con **gitleaks** en Docker (versión fija). Sobre los valores de desarrollo que ya están en el historial: no se reescribe el historial (PLAN.md, D5). Se documenta en `docs/proyecto-final/pruebas.md` que esos valores ya no son válidos.
  3. Generar el entregable `git archive --format=tar.gz --prefix=proyecto-final-microservicios/ -o proyecto-final-microservicios.tar.gz main` (desde `main`, después del último PR `dev → main`).
  4. Descomprimirlo en otra carpeta y repetir el paso 1 sobre esa copia.
- **Criterios de aceptación:** los pasos 1 y 4 con todo `healthy` y `make e2e` en verde; el reporte de gitleaks en `pruebas.md`; el `.tar.gz` sin `.env` ni `secrets/` (`tar tzf … | grep -E '\.env$|secrets/'` vacío).

### C8 · Documentación final — M · Ola 4 · ⬜

- **Rama:** `pf/C8-documentacion` · **Depende de:** C5 · **Revisa:** Valeria
- **Requisito del PDF:** "README con instrucciones claras", "Arquitectura documentada" y "Guías de configuración de herramientas de terceros".
- **Pasos:**
  1. Guías cortas en `docs/guias/`: `jenkins.md`, `sonarqube.md`, `rabbitmq.md`, `grafana.md` y `secretos.md` (y `keycloak.md` si se hace). Cada una cubre qué es, cómo se levanta, usuario y contraseña (de dónde salen, no el valor), cómo se usa en el proyecto y los problemas comunes.
  2. README raíz: estado ("Retos 1 a 8 y proyecto final completados"), una sección *Proyecto final* con lo agregado y el índice de guías.
  3. `docs/arquitectura.md` actualizado: Nginx, réplicas, redes y secretos. Las partes de V6 y V7 las completa Valeria en sus PR.
  4. Usar la skill `documentacion-retos` (`.claude/skills/documentacion-retos`) y su checklist.
- **Criterios de aceptación:** cada enlace del README abre un archivo que existe; cada comando de las guías se ejecutó y su resultado está escrito tal como salió.

---

## 5. Fichas de Valeria

### V1 · Plantillas de notificación en archivos — M · Ola 1 · ⬜

- **Rama:** `pf/V1-plantillas` · **Depende de:** nada · **Revisa:** Cristhian
- **Requisito del PDF:** "Plantillas de notificación configurables" (PLAN.md, F3 y D9).
- **Hoy:** los textos de los correos son strings fijos en Go, en `microservicios/notificaciones/internal/dominio/notificacion.go` (alrededor de las líneas 160–285; el asunto se arma en la línea ~284).
- **Pasos:**
  1. Una plantilla por tipo de evento en `microservicios/notificaciones/plantillas/<tipo>.tmpl`, con `text/template` de la librería estándar: `usuario.creado`, `usuario.recuperacion`, `vacaciones.programadas`, `cuenta.activada`, `cuenta.desactivada` (con sus variantes por motivo, como hoy).
  2. Cada plantilla define el asunto y el cuerpo, por ejemplo con `{{define "asunto"}}…{{end}}` y `{{define "cuerpo"}}…{{end}}`.
  3. El servicio carga las plantillas al arrancar desde `PLANTILLAS_DIR`, con `/app/plantillas` por defecto. **Si falta una o tiene un error de sintaxis, falla al arrancar con un mensaje claro**, no al enviar el primer correo.
  4. El Dockerfile copia `plantillas/` a la imagen. El compose la monta como volumen de solo lectura (`./microservicios/notificaciones/plantillas:/app/plantillas:ro`), así se cambia un texto sin reconstruir.
  5. **El texto de cada correo debe quedar idéntico al de hoy:** las pruebas E2E comparan frases exactas, por ejemplo "Su cuenta fue desactivada temporalmente por vacaciones.".
  6. **El correo de bienvenida lleva el token de activación y nunca una contraseña** (regla del PDF): agregar una prueba que lo verifique.
- **Criterios de aceptación:**
  - `make pruebas-notificaciones` en verde, con la cobertura que exige hoy.
  - `make e2e` completo.
  - **Prueba manual:** cambiar una frase del `.tmpl`, `docker compose restart notificaciones-service`, crear un empleado y ver la frase nueva en Mailhog. Captura en el PR.
- **Fuera de alcance:** plantillas HTML o en varios idiomas.

### V2 · Arreglar el bug de SonarQube en vacaciones — XS · Ola 1 · ⬜

- **Rama:** `pf/V2-interrupted-vacaciones` · **Depende de:** nada · **Revisa:** Cristhian
- **Hoy:** SonarQube marca la regla `java:S2142` en `microservicios/gestion-vacaciones/src/main/java/com/talentflow/vacaciones/infraestructura/web/SaludController.java:72`. Se captura un `InterruptedException` sin volver a interrumpir el hilo.
- **Pasos:** en ese `catch`, llamar `Thread.currentThread().interrupt()` y responder el check como `DOWN`. Agregar una prueba que interrumpa el hilo y compruebe las dos cosas.
- **Criterios de aceptación:** `make pruebas-vacaciones` en verde y SonarQube con **0 bugs** en gestion-vacaciones (pipeline en verde).

### V3 · Dockerfile multi-stage del Gateway — S · Ola 1 · ⬜

- **Rama:** `pf/V3-dockerfile-gateway` · **Depende de:** nada · **Revisa:** Cristhian
- **Requisito del PDF:** "Imágenes optimizadas (multi-stage builds recomendado)". El Gateway usa una sola etapa (`microservicios/api-gateway/Dockerfile`).
- **Pasos:** etapa `deps` con `npm ci --omit=dev`, luego una etapa final con solo el código y `node_modules` de producción, usuario no root (`USER node`). Seguir el patrón del Dockerfile de `gestion-empleados`, que ya es multi-stage.
- **Criterios de aceptación:**
  - La imagen nueva pesa menos que la anterior: anotar los dos tamaños de `docker images` en el PR.
  - `make e2e` en verde.
  - El pipeline `api-gateway` en verde.

### V4 · `GET /empleados/{id}/completo` en el Gateway — M · Ola 2 · ⬜

- **Rama:** `pf/V4-empleado-completo` · **Depende de:** nada · **Revisa:** Cristhian
- **Requisito del PDF (puntaje extra):** un endpoint del Gateway que combine en una sola respuesta los datos del empleado y los de su perfil, resolviendo el fallo parcial (PLAN.md, F9).
- **Contrato (escribirlo primero en el README del Gateway y en sus pruebas):**
  - Respuesta `200 { "empleado": {…}, "perfil": {…} | null, "parcial": false | true, "faltantes": ["perfil"] }`.
  - Si empleados responde 404, se devuelve **404**. Si empleados falla, **503**. Si **perfiles** falla o tarda más de 2 s, **200 con `perfil: null`, `parcial: true`**: el dato principal está.
  - **Autorización:** la misma que `GET /empleados/{id}` (ADMIN o USER, lectura). El token del cliente se reenvía a los dos servicios.
  - Las 2 llamadas van **en paralelo**: se ven como 2 spans hermanos en Jaeger.
- **Archivos:** `microservicios/api-gateway/app.js` (o un módulo nuevo `composicion.js` que `app.js` monta antes del proxy de `/empleados`), sus pruebas y su README.
- **Criterios de aceptación:**
  - Pruebas del Gateway de los 4 casos (completo, 404, empleados caído, perfiles caído o lento), con la cobertura del Gateway sin bajar.
  - **E2E nuevo:** un ADMIN pide el empleado completo y ve los dos bloques.
  - **Prueba manual:** con `docker compose stop perfiles-service`, la respuesta es 200 con `parcial: true`. Captura de Jaeger en el PR.

### V5 · Timeout y error claro en el proxy del Gateway — S · Ola 2 · ⬜

- **Rama:** `pf/V5-timeout-gateway` · **Depende de:** nada; si V4 ya está integrado, partir de ahí · **Revisa:** Cristhian
- **Requisito del PDF:** "Manejo correcto de timeouts y reintentos" (PLAN.md, F7).
- **Hoy:** `createProxyMiddleware` en `microservicios/api-gateway/app.js` (alrededor de las líneas 174–180) no tiene timeout.
- **Pasos:**
  1. Agregar `proxyTimeout` y `timeout`, configurables con `GATEWAY_PROXY_TIMEOUT_MS` (10000 por defecto).
  2. Si un servicio no responde a tiempo, devolver **504** con el formato de error del Gateway. El 503 queda para el servicio caído.
  3. **Sin reintentos en el proxy:** reintentar un `POST` podría duplicar un alta. Los reintentos viven donde la operación es idempotente (empleados → departamentos, Reto 3). Explicar esto en el README.
- **Criterios de aceptación:**
  - Una prueba con un servicio falso lento da 504 y otra con uno caído da 503.
  - **Prueba manual:** con `docker-compose.caos.yml` y `CAOS_LATENCIA_MS` mayor que el timeout, la respuesta es 504.
  - `make e2e` en verde.

### V6 · Réplicas de los servicios sin estado — M · Ola 4 · `compose` · ⬜

- **Rama:** `pf/V6-replicas` · **Depende de:** C5 (para no chocar en el compose) y V8 · **Revisa:** Cristhian
- **Requisito del PDF:** "Servicios escalables horizontalmente y stateless" (PLAN.md, F6).
- **Pasos:**
  1. Quitar `container_name` de los 7 servicios de aplicación (punto 6 de la sección 1). Buscar con `grep -rn "ms-\|api-gateway" docs Makefile scripts e2e-tests` los comandos que usan esos nombres y cambiarlos a `docker compose exec <servicio>`.
  2. `deploy.replicas: 2` en `empleados-service`, `departamentos-service`, `perfiles-service`, `notificaciones-service`, `auth-service` y `vacaciones-service`. El Gateway se escala en V7.
  3. **Prometheus:** cambiar los targets estáticos por `dns_sd_configs` (tipo `A`, por nombre de servicio), para ver cada réplica (punto 5 de la sección 1).
  4. **Revisar el estado en memoria:**
     - **Circuit breaker de empleados:** cada réplica tiene el suyo. Es aceptable, pero hay que documentarlo.
     - **Reconciliación de los `PENDIENTE`:** si corre en las dos réplicas, ¿duplica eventos? La deduplicación de los consumidores lo absorbe; probarlo y documentarlo.
     - **Correo duplicado:** que notificaciones no envíe dos veces el mismo correo con 2 réplicas, gracias a la deduplicación por la tabla `eventos_procesados`. Probarlo.
  5. Con 2 réplicas, RabbitMQ reparte los mensajes entre ellas (competing consumers). Verificar en la UI de RabbitMQ que cada cola tiene 2 consumidores.
  6. **Política de reinicio:** `auth-service`, `notificaciones-service` y `perfiles-service` usan `restart: on-failure`. Si Docker se reinicia, salen con código 0 y no vuelven solos (pasó el 2026-11-12, ver `docs/reto-08/pruebas.md`, sección 9). Pasar los servicios de aplicación a `restart: unless-stopped`, como el Gateway.
- **Criterios de aceptación:**
  - `docker compose ps` muestra 2 réplicas `healthy` de cada servicio.
  - Prometheus muestra 12 targets de esos servicios en UP.
  - `make e2e` en verde.
  - Crear un empleado manda **un** solo correo de bienvenida a Mailhog.
- **Nota de recursos:** con 2 réplicas, el sistema usa más memoria. En una Mac con poca RAM, probar sin el perfil `ci` y sin el caos.

### V7 · Nginx como balanceador del Gateway — M · Ola 4 · `compose` · ⬜

- **Rama:** `pf/V7-balanceador` · **Depende de:** V6 · **Revisa:** Cristhian
- **Requisito del PDF:** "Load balancer distribuyendo el tráfico entre instancias, con health checks" (PLAN.md, D6).
- **Pasos:**
  1. Un servicio `balanceador` (imagen `nginx` con versión fija) publica el `8080` del host. El Gateway deja de publicar su puerto y pasa a 2 réplicas.
  2. `infra/nginx/nginx.conf`: un `upstream` hacia `api-gateway:8080`, con el `resolver 127.0.0.11 valid=10s` de Docker para descubrir las réplicas, `proxy_next_upstream error timeout http_502 http_503`, `max_fails` y `fail_timeout` (health check pasivo).
  3. El healthcheck del contenedor `balanceador` llama al `/health` del Gateway a través de Nginx.
  4. **Reenviar las cabeceras** `Authorization`, `traceparent` y `X-Forwarded-For`, para que la traza siga completa en Jaeger.
- **Criterios de aceptación:**
  - Tras 20 peticiones a `localhost:8080/health`, los logs de las 2 réplicas del Gateway muestran tráfico en ambas: `docker compose logs api-gateway | grep -c "GET /health"` por réplica.
  - `make e2e` en verde, porque el puerto del host no cambia.
  - La traza del alta de empleado sigue completa.

### V8 · Un solo scheduler activo en vacaciones (ShedLock) — M · Ola 3 · ⬜

- **Rama:** `pf/V8-shedlock` · **Depende de:** nada · **Revisa:** Cristhian
- **Requisito:** con 2 réplicas, el scheduler (`VacacionesScheduler` y `ProcesadorCicloVacaciones`) correría dos veces cada pasada (PLAN.md, D7).
- **Pasos:**
  1. Agregar ShedLock (`shedlock-spring` y `shedlock-provider-jdbc-template`, versión fija en el `pom.xml`).
  2. Crear la tabla `shedlock` en `init.sql` **y** en una migración al arrancar, por el punto 2 de la sección 1.
  3. `@SchedulerLock(name = "ciclo-vacaciones", lockAtMostFor = …, lockAtLeastFor = …)`, con los valores justificados en un comentario según el cron (cada minuto; cada 5 s en E2E).
- **Criterios de aceptación:**
  - **Prueba de integración** (Testcontainers, como los `*IT.java` que ya existen): dos ejecuciones simultáneas del scheduler, y solo una procesa.
  - `make pruebas-vacaciones` en verde.
  - `make e2e` en verde, incluido el ciclo de suspensión por vacaciones.

### V9 · Evidencia: el sistema sigue respondiendo si cae una instancia — S · Ola 4 · ⬜

- **Rama:** `pf/V9-evidencia-instancia` · **Depende de:** V7 · **Revisa:** Cristhian
- **Requisito del PDF:** "El sistema sigue atendiendo peticiones cuando cae una instancia" y "Al menos un Circuit Breaker con fallback demostrable".
- **Pasos:** en `docs/proyecto-final/pruebas.md`, documentar con comandos, resultados reales y capturas:
  1. Un bucle de 30 peticiones a `localhost:8080/empleados` (con token) mientras se detiene **una** réplica del Gateway (`docker stop <id de una réplica>`). Contar las respuestas 200: deben ser todas o casi todas, y explicar cualquier fallo.
  2. Lo mismo con una réplica de empleados.
  3. El circuit breaker: con departamentos apagado, el empleado queda `PENDIENTE` (ya existe en el Reto 3; repetirlo con las réplicas).
  4. Prometheus y Grafana mientras tanto: la réplica caída en DOWN, la otra en UP.
- **Criterios de aceptación:** `pruebas.md` con las 4 pruebas, sus números y las capturas, y una sección en `docs/proyecto-final/sustentacion.md` con el guion de esta demostración.

---

## 6. Seguimiento

| ID | Objetivo | Dueño | Ola | PR | Estado |
|---|---|---|---|---|---|
| C1 | Generar los secretos | Cristhian | 1 | — | ⬜ |
| C2 | Cero secretos en compose y código | Cristhian | 2 | — | ⬜ |
| C3 | Compose Secrets: JWT | Cristhian | 3 | — | ⬜ |
| C4 | Compose Secrets: Go y Node | Cristhian | 3 | — | ⬜ |
| C5 | Compose Secrets: Python, Java y PHP | Cristhian | 3 | — | ⬜ |
| C6 | Redes por capa | Cristhian | 5 | — | ⬜ |
| C7 | Verificación desde cero y entregable | Cristhian | 5 | — | ⬜ |
| C8 | Documentación final | Cristhian | 4 | — | ⬜ |
| V1 | Plantillas de notificación | Valeria | 1 | — | ⬜ |
| V2 | Bug S2142 en vacaciones | Valeria | 1 | — | ⬜ |
| V3 | Dockerfile multi-stage del Gateway | Valeria | 1 | — | ⬜ |
| V4 | `GET /empleados/{id}/completo` | Valeria | 2 | — | ⬜ |
| V5 | Timeout del proxy | Valeria | 2 | — | ⬜ |
| V6 | Réplicas | Valeria | 4 | — | ⬜ |
| V7 | Nginx balanceador | Valeria | 4 | — | ⬜ |
| V8 | ShedLock en vacaciones | Valeria | 3 | — | ⬜ |
| V9 | Evidencia de caída de una instancia | Valeria | 4 | — | ⬜ |

Al terminar un objetivo: marcar ✅, poner el número del PR y una línea de resultado en su ficha.
