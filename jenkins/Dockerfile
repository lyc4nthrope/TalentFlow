FROM jenkins/jenkins:lts-jdk17

# Instalar plugins necesarios de forma automatizada (incluye reportes de pruebas)
# Ref: https://github.com/jenkinsci/docker#preinstalling-plugins
RUN jenkins-plugin-cli --plugins \
    docker-workflow \
    pipeline-stage-view \
    workflow-aggregator \
    git \
    configuration-as-code \
    job-dsl \
    sonar \
    # --- Publicación de reportes de pruebas (Reto 6) ---
    junit \
    coverage \
    cucumber-reports \
    htmlpublisher

# Cambiar a usuario root para instalar herramientas del sistema
USER root

# Instalar Docker CLI y utilidades necesarias
RUN apt-get update && \
    apt-get install -y docker.io curl ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# Descargar e instalar la versión V2 de Docker Compose para arquitectura ARM64 (Apple Silicon M4)
RUN mkdir -p /usr/libexec/docker/cli-plugins/ && \
    curl -SL "https://github.com/docker/compose/releases/download/v2.29.1/docker-compose-linux-aarch64" \
    -o /usr/libexec/docker/cli-plugins/docker-compose && \
    chmod +x /usr/libexec/docker/cli-plugins/docker-compose

# Crear enlace simbólico para que funcione tanto 'docker compose' como 'docker-compose'
RUN ln -s /usr/libexec/docker/cli-plugins/docker-compose /usr/local/bin/docker-compose

# Agregar usuario jenkins al grupo docker para acceder al socket
RUN groupadd -f docker && usermod -aG docker jenkins

# Garantizar permisos de acceso al socket
RUN chmod 666 /var/run/docker.sock || true

USER jenkins