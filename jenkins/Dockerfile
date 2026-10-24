FROM jenkins/jenkins:lts-jdk17

# Instalar plugins necesarios de forma automatizada (incluye reportes de pruebas)
RUN jenkins-plugin-cli --plugins \
    docker-workflow \
    pipeline-stage-view \
    workflow-aggregator \
    git \
    configuration-as-code \
    job-dsl \
    sonar \
    junit \
    coverage \
    cucumber-reports \
    htmlpublisher

USER root

# 1. Instalar dependencias base, Docker CLI, Node.js y npm
RUN apt-get update && \
    apt-get install -y \
        docker.io \
        nodejs \
        npm \
        curl \
        ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# 2. Instalar Docker Compose (CLI plugin v2) descargando el binario oficial
RUN mkdir -p /usr/libexec/docker/cli-plugins && \
    curl -SL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-$(uname -m)" \
    -o /usr/libexec/docker/cli-plugins/docker-compose && \
    chmod +x /usr/libexec/docker/cli-plugins/docker-compose

# Crear alias/enlace simbólico para compatibilidad con 'docker-compose'
RUN ln -s /usr/libexec/docker/cli-plugins/docker-compose /usr/local/bin/docker-compose || true

# 3. Agregar usuario jenkins al grupo docker para acceder al socket
RUN groupadd -f docker && usermod -aG docker jenkins

USER jenkins