# Reto 7 — Plan de diseño e integración continua

## 1. Diagnóstico de partida

Cuando se llega al reto 7, el proyecto ya tiene:

- arquitectura por microservicios,
- API Gateway y validación JWT,
- RabbitMQ para eventos,
- seguridad, cuentas y pruebas automáticas,
- flujo de pruebas E2E con reportes y cobertura.

Lo que faltaba era una capa de automatización real de integración continua: no solo ejecutar pruebas localmente, sino tener una herramienta que construya, valide y publique artefactos de cada servicio con trazabilidad.

## 2. Objetivo del reto

Diseñar una infraestructura de CI/CD centrada en Jenkins que permita:

- compilar cada microservicio,
- ejecutar pruebas y generar reportes,
- analizar calidad con SonarQube,
- publicar imágenes Docker en un registro local,
- orquestar la validación E2E del sistema completo.

## 3. Decisiones de diseño

### D1. Jenkins como orquestador principal

Se eligió Jenkins por su flexibilidad para ejecutar pipelines declarativos, integrar múltiples herramientas y incorporar reportes de pruebas, cobertura y calidad estática. También permite automatizar tareas de build, test y packaging del proyecto completo.

### D2. Job DSL para crear jobs automáticamente

La configuración `casc.yaml` crea los pipelines mediante Job DSL. Esto facilita:

- reproducibilidad,
- mantenimiento del control de versiones,
- registro de la estructura de CI en el repositorio.

### D3. Registro Docker local

Un registro local (`registry:2`) es suficiente para entorno académico y permite publicar las imágenes del proyecto sin depender de Docker Hub. Es una solución simple, funcional y totalmente integrada con Docker Compose.

### D4. SonarQube como capa de calidad

SonarQube se integra para detectar deuda técnica, vulnerabilidades y cobertura. El pipeline por microservicio usa `sonar-scanner` y un `Quality Gate` que aborta la ejecución si la calidad no cumple el criterio mínimo.

### D5. Pipeline raíz y pipelines por servicio

Se separan dos niveles:

- pipeline maestro: compila y publica todos los microservicios y ejecuta la suite E2E global,
- pipelines por servicio: repiten la lógica de build, test y análisis para cada proyecto técnico.

Esto da trazabilidad por componente y evita que un fallo en un servicio quede oculto dentro del orquestador general.

### D6. Infraestructura de CI/CD en Docker Compose

Se añade al `docker-compose.yml` la infraestructura de apoyo del reto 7, manteniendo el mismo paradigma de proyecto: todo se levanta con un solo comando, usando contenedores para aislar dependencias y facilitar la ejecución uniforme en equipos y CI.

### D7. Reportes estándar para Jenkins

Se generan salidas JUnit, Cobertura y HTML para que Jenkins pueda desplegarlos en la interfaz y permitir seguimiento visual del estado del pipeline.

### D8. E2E como validación final de integridad

La etapa de validación final del pipeline raíz ejecuta la suite BDD del sistema. En un proyecto de microservicios, el objetivo no es solo compilar; también es comprobar que el conjunto funciona como un sistema integrado.

## 4. Fases del reto

### Fase 1: preparar la base de CI/CD

- añadir `registry`
- añadir `sonarqube` y `sonar-db`
- crear imagen de Jenkins con plugins y herramientas necesarias
- montar la configuración de Job DSL

### Fase 2: crear pipelines por servicio

- representar build, test, cobertura y calidad por microservicio
- configurar `SonarQube` y `Quality Gate`
- preparar publicación de imágenes Docker en registry

### Fase 3: crear pipeline orquestador

- construir todos los microservicios en paralelo
- ejecutar la suite E2E del proyecto completo
- publicar reportes de test y HTML para revisión en Jenkins

### Fase 4: validación operativa

- verificar que el repositorio, Docker, Jenkins y SonarQube responden correctamente
- revisar los reportes generados
- corregir problemas de permisos en `docker.sock`, rutas y conexión al host

## 5. Aprendizajes clave

- La automatización no solo reduce esfuerzo humano, también crea trazabilidad y evidencia de calidad.
- Un sistema distribuido necesita validación por capas: build por servicio, análisis estático y validación del conjunto.
- En Jenkins, los problemas de permisos en Docker suelen aparecer al compilar imágenes desde el agente; la solución más directa en un entorno local es ajustar permisos del socket.
- Las herramientas de envío de reportes (`junit`, `cucumber-reports`, `htmlpublisher`) son esenciales para que el pipeline sea comprensible y ejecutable por un equipo real.

## 6. Resultado esperado

El reto queda resuelto cuando:

- `docker compose up --build -d` levanta Jenkins, SonarQube y registry,
- Jenkins crea los jobs del proyecto,
- los pipelines por servicio ejecutan build y pruebas,
- SonarQube analiza el código,
- el pipeline master publica imágenes y valida la suite E2E.

---

> Este archivo recoge el diagnóstico, decisiones y lecciones del reto; para la guía de uso práctica ver [`README.md`](README.md).
